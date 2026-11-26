package design

import (
	"fmt"
	"image"
	"math"
	"os"
	"strings"
	"testing"
	"unicode"

	"github.com/ZerGo0/editor/internal/design/redline"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const (
	redlineScale     = 2
	redlineWidth     = 640
	redlineHeight    = 1500
	redlineAllowPath = "testdata/redline_allowlist.txt"
	redlineUpdateEnv = "DESIGN_REDLINE_UPDATE"
	redlineDetailEnv = "DESIGN_REDLINE_DETAIL"
	stateMargin      = 4
)

var stateRoles = map[string]bool{
	"button": true, "toggleButton": true, "menuButton": true, "tab": true, "checkBox": true, "switch": true,
	"popUpButton": true, "listItem": true, "treeItem": true, "link": true,
}

var redlineThemes = []struct {
	name    string
	palette func() *token.Palette
}{
	{"dark", token.Dark},
	{"light", token.Light},
	{"dark-hc", func() *token.Palette { return token.For(true, ui.Transparent, true) }},
	{"light-hc", func() *token.Palette { return token.For(false, ui.Transparent, true) }},
}

var redlineRoles = map[ui.Role]string{
	ui.RoleGroup: "group", ui.RoleText: "text", ui.RoleButton: "button", ui.RoleLink: "link",
	ui.RoleCheckBox: "checkBox", ui.RoleRadio: "radio", ui.RoleSwitch: "switch", ui.RoleSlider: "slider",
	ui.RoleProgress: "progress", ui.RoleTextField: "textField", ui.RoleImage: "image", ui.RoleList: "list",
	ui.RoleScroll: "scroll", ui.RoleDialog: "dialog", ui.RolePopup: "popup", ui.RoleTooltip: "tooltip",
	ui.RolePopUpButton: "popUpButton", ui.RoleTabList: "tabList", ui.RoleTab: "tab", ui.RoleSplitter: "splitter",
	ui.RoleStatus: "status", ui.RoleTable: "table", ui.RoleRow: "row", ui.RoleCell: "cell",
	ui.RoleTree: "tree", ui.RoleTreeItem: "treeItem", ui.RoleListItem: "listItem", ui.RoleMenuButton: "menuButton",
	ui.RoleToolbar: "toolbar", ui.RoleToggleButton: "toggleButton", ui.RoleComboBox: "comboBox",
	ui.RoleDisclosure: "disclosure", ui.RoleMenu: "menu", ui.RoleMenuItem: "menuItem", ui.RoleHeading: "heading",
	ui.RoleRegion: "region", ui.RoleAlert: "alert", ui.RoleSeparator: "separator", ui.RoleSearch: "search",
	ui.RoleListBox: "listBox", ui.RoleOption: "option", ui.RoleTabPanel: "tabPanel",
}

func redlineRole(r ui.Role) string {
	if name, ok := redlineRoles[r]; ok {
		return name
	}
	return fmt.Sprintf("role%d", r)
}

func testerBoxes(tt *ui.Tester) []redline.Box {
	var boxes []redline.Box
	for _, a := range tt.AccessAll() {
		if a.Offscreen {
			continue
		}
		boxes = append(boxes, redline.Box{Role: redlineRole(a.Role), Label: a.Name, Disabled: a.Disabled,
			Rect: redline.Rect{X: float64(a.Bounds.X), Y: float64(a.Bounds.Y), W: float64(a.Bounds.W), H: float64(a.Bounds.H)}})
	}
	return boxes
}

func loadRedlineAllow(t *testing.T) *redline.Allowlist {
	t.Helper()
	allow, err := redline.LoadAllowlist(redlineAllowPath)
	if err != nil {
		t.Fatal(err)
	}
	return allow
}

func checkRedlines(t *testing.T, res redline.Result, allow *redline.Allowlist) {
	t.Helper()
	if bad := allow.Check(res); len(bad) > 0 {
		t.Errorf("%d redline findings in %s (fix them in internal/design, or add a reviewed entry to %s):\n%s",
			len(bad), res.Name, redlineAllowPath, res.Describe(bad))
	}
}

func finishRedlineAllow(t *testing.T, allow *redline.Allowlist) {
	t.Helper()
	stale := allow.Stale()
	if len(stale) == 0 {
		return
	}
	if os.Getenv(redlineUpdateEnv) == "" {
		t.Errorf("%s has entries that match no finding; remove them with %s=1:\n%s", redlineAllowPath, redlineUpdateEnv, strings.Join(stale, "\n"))
		return
	}
	if err := allow.Prune(); err != nil {
		t.Fatal(err)
	}
}

func analyzeTester(t *testing.T, tt *ui.Tester, name string) redline.Result {
	t.Helper()
	return analyze(t, name, tt.Image(), testerBoxes(tt), true)
}

func analyze(t *testing.T, name string, img image.Image, boxes []redline.Box, write bool) redline.Result {
	t.Helper()
	res, err := redline.Analyze(redline.Input{Name: name, Image: img, Boxes: boxes, Scale: redlineScale,
		Detail: os.Getenv(redlineDetailEnv) != ""})
	if err != nil {
		t.Fatal(err)
	}
	if dir := os.Getenv(shotsEnv); dir != "" && (write || len(res.Findings) > 0) {
		if _, err := res.Write(dir); err != nil {
			t.Fatal(err)
		}
	}
	return res
}

func analyzeState(t *testing.T, tt *ui.Tester, target redline.Box, name string) redline.Result {
	t.Helper()
	area := target.Rect
	area = redline.Rect{X: area.X - stateMargin, Y: area.Y - stateMargin, W: area.W + 2*stateMargin, H: area.H + 2*stateMargin}
	full := tt.Image()
	crop := image.Rect(int(math.Floor(area.X*redlineScale)), int(math.Floor(area.Y*redlineScale)),
		int(math.Ceil(area.Right()*redlineScale)), int(math.Ceil(area.Bottom()*redlineScale))).Intersect(full.Bounds())
	ox, oy := float64(crop.Min.X)/redlineScale, float64(crop.Min.Y)/redlineScale
	var boxes []redline.Box
	for _, b := range testerBoxes(tt) {
		r := b.Rect
		if r.X >= target.Rect.X && r.Y >= target.Rect.Y && r.Right() <= target.Rect.Right() && r.Bottom() <= target.Rect.Bottom() {
			b.Rect = redline.Rect{X: r.X - ox, Y: r.Y - oy, W: r.W, H: r.H}
			boxes = append(boxes, b)
		}
	}
	return analyze(t, name, full.SubImage(crop), boxes, false)
}

func slug(label string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return '-'
	}, label)
}

func sweepStates(t *testing.T, tt *ui.Tester, view string, allow *redline.Allowlist) {
	t.Helper()
	for _, b := range testerBoxes(tt) {
		if !stateRoles[b.Role] || b.Disabled {
			continue
		}
		x, y := float32(b.Rect.CenterX()), float32(b.Rect.CenterY())
		tt.Move(x, y)
		checkRedlines(t, analyzeState(t, tt, b, view+"-hover-"+slug(b.Label)), allow)
		tt.Press(x, y)
		checkRedlines(t, analyzeState(t, tt, b, view+"-pressed-"+slug(b.Label)), allow)
		tt.CancelPress()
	}
	tt.Move(0, 0)
	seen := map[redline.Rect]bool{}
	for range testerBoxes(tt) {
		tt.Key(0, ui.KeyTab)
		var focused *redline.Box
		for _, a := range tt.AccessAll() {
			if a.Focused && !a.Offscreen {
				focused = &redline.Box{Role: redlineRole(a.Role), Label: a.Name,
					Rect: redline.Rect{X: float64(a.Bounds.X), Y: float64(a.Bounds.Y), W: float64(a.Bounds.W), H: float64(a.Bounds.H)}}
				break
			}
		}
		if focused == nil || seen[focused.Rect] {
			break
		}
		seen[focused.Rect] = true
		checkRedlines(t, analyzeState(t, tt, *focused, view+"-focus-"+slug(focused.Label)), allow)
	}
}

func TestGalleryRedlines(t *testing.T) {
	useFakeCommands(t)
	allow := loadRedlineAllow(t)
	for _, th := range redlineThemes {
		p, g := th.palette(), newGallery()
		tt := ui.NewTester(func(c *ui.Context) {
			ui.Themed(c.Key(th.name), p.Theme()).Fill().Background(p.Layer.SideBar).Children(func() { galleryColumn(c, g) })
		}, redlineWidth, redlineHeight)
		tt.SetScale(redlineScale)
		tt.Frame()
		checkRedlines(t, analyzeTester(t, tt, "gallery-"+th.name), allow)
		sweepStates(t, tt, "gallery-"+th.name, allow)
	}
	finishRedlineAllow(t, allow)
}
