package redline

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

const (
	contextDepth = 2
	cropMargin   = 16
	dirMode      = 0o755
	fileMode     = 0o644
)

var roleComponents = map[string]string{
	"button":       "action.IconButton or action.Button",
	"toggleButton": "action.ToggleIconButton or field.Toggle",
	"checkBox":     "action.Checkbox",
	"switch":       "action.Switch",
	"popUpButton":  "action.Select",
	"textField":    "field.TextInput or field.SearchInput",
	"tab":          "chrome.TabStrip, chrome.ActivityBar or chrome.ViewTabs",
	"tabList":      "chrome.TabStrip or chrome.ActivityBar",
	"row":          "list.View row (list/row.go)",
	"listItem":     "list.View row (list/row.go)",
	"treeItem":     "list.Tree row (list/row.go)",
	"heading":      "pane.ViewTitle or pane.Header",
	"toolbar":      "pane.Toolbar",
	"status":       "chrome.StatusItem",
	"text":         "design.Text, badge.Count or badge.Tag",
	"menuButton":   "action.MenuIconButton",
	"link":         "action.Button (Ghost) or feedback link",
}

func (r Result) context(i int) string {
	var labels []string
	for p, steps := r.Parents[i], 0; p != noBox && steps < len(r.Boxes); p, steps = r.Parents[p], steps+1 {
		if l := r.Boxes[p].Label; l != "" {
			labels = append([]string{fmt.Sprintf("%s %q", r.Boxes[p].Role, l)}, labels...)
		}
	}
	if len(labels) > contextDepth {
		labels = labels[len(labels)-contextDepth:]
	}
	if len(labels) == 0 {
		return "window"
	}
	return strings.Join(labels, " > ")
}

func (r Result) element(i int) string {
	b := r.Boxes[i]
	return fmt.Sprintf("%s %q (%s)", b.Role, b.Label, b.Rect)
}

func (r Result) cropName(n int, f Finding) string {
	return fmt.Sprintf("%s.redline/%02d-%s.png", r.Name, n+1, f.Rule)
}

func value(v float64, unit string) string {
	if unit == UnitRatio {
		return num(v) + unit
	}
	if unit == UnitPx {
		return signed(v) + " " + unit
	}
	return num(v) + " " + unit
}

func (r Result) Report() string {
	var b strings.Builder
	img := r.Image.Bounds()
	fmt.Fprintf(&b, "redline %s\n", r.Name)
	fmt.Fprintf(&b, "image %d x %d device px at scale %s: px values are layout px (1 px = %s device px)\n", img.Dx(), img.Dy(), num(r.Scale), num(r.Scale))
	counts := map[MeasureKind]int{}
	for _, m := range r.Measures {
		counts[m.Kind]++
	}
	fmt.Fprintf(&b, "checked %d boxes, %d icon boxes, %d row icons, %d text rows, drew %d guides\n",
		len(r.Boxes), counts[MeasureIcon], counts[MeasureRowIcon], counts[MeasureText], r.Guides)
	byRule := map[Rule]int{}
	for _, f := range r.Findings {
		byRule[f.Rule]++
	}
	var parts []string
	for _, rule := range ruleOrder {
		if byRule[rule] > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", rule, byRule[rule]))
		}
	}
	fmt.Fprintf(&b, "findings %d", len(r.Findings))
	if len(parts) > 0 {
		fmt.Fprintf(&b, ": %s", strings.Join(parts, ", "))
	}
	b.WriteString("\n")
	groups := map[string][]int{}
	var order []string
	for n, f := range r.Findings {
		ctx := r.context(f.Box)
		if _, ok := groups[ctx]; !ok {
			order = append(order, ctx)
		}
		groups[ctx] = append(groups[ctx], n)
	}
	for _, ctx := range order {
		fmt.Fprintf(&b, "\n== in %s ==\n", ctx)
		for _, n := range groups[ctx] {
			r.writeFinding(&b, n)
		}
	}
	if r.detail {
		r.writeMeasures(&b)
	}
	return b.String()
}

func (r Result) Describe(findings []int) string {
	var b strings.Builder
	for _, n := range findings {
		if n >= 0 && n < len(r.Findings) {
			fmt.Fprintf(&b, "[%s in %s] ", r.Name, r.context(r.Findings[n].Box))
			r.writeFinding(&b, n)
		}
	}
	return b.String()
}

func (r Result) writeFinding(b *strings.Builder, n int) {
	f := r.Findings[n]
	fmt.Fprintf(b, "#%d %s: %s\n", n+1, f.Rule, f.Check)
	fmt.Fprintf(b, "  element: %s\n", r.element(f.Box))
	if f.Other != noBox {
		fmt.Fprintf(b, "  against: %s\n", r.element(f.Other))
	}
	expected := value(f.Expected, f.Unit)
	if f.Rule == RuleContrast {
		expected = "at least " + expected
	}
	if f.Tolerance > 0 {
		expected += " (tolerance " + num(f.Tolerance) + ")"
	}
	fmt.Fprintf(b, "  measured: %s, expected: %s\n", value(f.Measured, f.Unit), expected)
	fmt.Fprintf(b, "  fix: %s\n", f.Fix)
	if c, ok := roleComponents[r.Boxes[f.Box].Role]; ok {
		fmt.Fprintf(b, "  likely component: %s\n", c)
	}
	fmt.Fprintf(b, "  allow key: %s\n", r.Key(f))
	fmt.Fprintf(b, "  crop: %s\n", r.cropName(n, f))
}

func (r Result) writeMeasures(b *strings.Builder) {
	b.WriteString("\n== measurements ==\n")
	for _, m := range r.Measures {
		bx := r.Boxes[m.Box]
		fmt.Fprintf(b, "%s %s %q ink w %s h %s", m.Kind, bx.Role, bx.Label, num(m.Ink.W), num(m.Ink.H))
		if m.HasX {
			fmt.Fprintf(b, " dx %s", signed(m.DX))
		}
		fmt.Fprintf(b, " dy %s", signed(m.DY))
		if m.Kind != MeasureText {
			fmt.Fprintf(b, " stroke %s soft %s", num(m.Stroke), num(m.Soft))
		}
		fmt.Fprintf(b, " contrast %s%s\n", num(m.Contrast), UnitRatio)
	}
}

type Files struct {
	Image  string
	Report string
	Crops  []string
}

func (r Result) Write(dir string) (Files, error) {
	if r.Image == nil {
		return Files{}, errNoImage
	}
	out := Files{Image: filepath.Join(dir, r.Name+".redline.png"), Report: filepath.Join(dir, r.Name+".redline.txt")}
	if err := os.MkdirAll(filepath.Join(dir, r.Name+".redline"), dirMode); err != nil {
		return Files{}, fmt.Errorf("redline: create crop folder: %w", err)
	}
	if err := writePNG(out.Image, r.Image); err != nil {
		return Files{}, err
	}
	if err := os.WriteFile(out.Report, []byte(r.Report()), fileMode); err != nil {
		return Files{}, fmt.Errorf("redline: write report: %w", err)
	}
	for n, f := range r.Findings {
		crop := pxRect(f.Area.grow(cropMargin), r.Scale).Intersect(r.Image.Bounds())
		if crop.Empty() {
			continue
		}
		path := filepath.Join(dir, r.cropName(n, f))
		if err := writePNG(path, r.Image.SubImage(crop)); err != nil {
			return Files{}, err
		}
		out.Crops = append(out.Crops, path)
	}
	return out, nil
}

func writePNG(path string, img image.Image) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return fmt.Errorf("redline: encode %s: %w", path, err)
	}
	if err := os.WriteFile(path, buf.Bytes(), fileMode); err != nil {
		return fmt.Errorf("redline: write %s: %w", path, err)
	}
	return nil
}
