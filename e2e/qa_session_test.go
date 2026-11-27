//go:build e2e

package e2e

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ZerGo0/editor/e2e/qa"
	"github.com/ZerGo0/editor/internal/session"
)

const (
	windowKindContent  = "content"
	frameStatsEnv      = "MYGO_FRAME_STATS"
	frameStatsAll      = "all"
	settleTimeoutMs    = 300
	windowStateFile    = "window-state.json"
	sessionFileName    = "session.json"
	settingsFileName   = "settings.json"
	windowStateKeyBase = "project:"
	smallestWindowW    = 640
	smallestWindowH    = 400
	normalWindowW      = 1280
	normalWindowH      = 820
	narrowestSidebar   = 170
	defaultSidebar     = 260
	themeDark          = "Dark"
	themeLight         = "Light"
	cropMargin         = 8.0
	maxCropsPerView    = 12
	shotFilePerm       = 0o644
	artifactDirPerm    = 0o755
	cropDirName        = "crops"
	idleWatchWindow    = 600 * time.Millisecond
)

type QAOptions struct {
	Width        int
	Height       int
	SidebarWidth float32
	Theme        string
	SideView     string
	PanelOpen    bool
	Panel        string
	Tabs         []string
	ActiveTab    int
	Settings     map[string]any
	FrameStats   bool
	QuietTimeout time.Duration
	WaitTimeout  time.Duration
}

type QAApp struct {
	*App
	Win  int
	Root string
	Opts QAOptions
	Temp string
	t    *testing.T
}

type Sample struct {
	Elapsed  time.Duration
	Settled  bool
	Frames   []qa.Frame
	FrameSum time.Duration
}

func seedUserData(t *testing.T, root string, o QAOptions) string {
	t.Helper()
	dir := t.TempDir()
	settings := map[string]any{}
	if o.Theme != "" {
		settings["workbench.colorTheme"] = o.Theme
	}
	for k, v := range o.Settings {
		settings[k] = v
	}
	writeJSON(t, filepath.Join(dir, settingsFileName), settings)
	store, err := session.Load(filepath.Join(dir, sessionFileName))
	if err != nil {
		t.Fatal(err)
	}
	project := session.Project{SidebarWidth: o.SidebarWidth, SideView: o.SideView, PanelOpen: o.PanelOpen, Panel: o.Panel}
	if len(o.Tabs) > 0 {
		tabs := make([]session.Tab, len(o.Tabs))
		for i, p := range o.Tabs {
			tabs[i] = session.Tab{Path: p}
		}
		project.Layout = &session.Node{Group: &session.Group{Tabs: tabs, Active: o.ActiveTab}}
	}
	store.State.Projects[root] = project
	store.State.Recent = []string{root}
	store.State.Open = []string{root}
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	if o.Width > 0 && o.Height > 0 {
		writeJSON(t, filepath.Join(dir, windowStateFile), map[string]any{windowStateKeyBase + root: map[string]any{"x": 0, "y": 0, "width": o.Width, "height": o.Height}})
	}
	return dir
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, shotFilePerm); err != nil {
		t.Fatal(err)
	}
}

func LaunchQA(t *testing.T, root string, o QAOptions) *QAApp {
	t.Helper()
	env := []string{frameStatsEnv + "="}
	var temp string
	if o.FrameStats {
		temp = t.TempDir()
		env = []string{frameStatsEnv + "=" + frameStatsAll, showUnfocusedEnv + "=1", tempDirEnv + "=" + temp}
	}
	app := LaunchEnv(t, seedUserData(t, root, o), env, root)
	var win int
	app.Eventually("project window", func() bool {
		for _, w := range app.Windows() {
			if w.Kind == windowKindContent {
				win = w.ID
				return true
			}
		}
		return false
	})
	q := &QAApp{App: app, Win: win, Root: root, Opts: o, Temp: temp, t: t}
	q.Settle()
	if o.FrameStats {
		q.WaitCPUQuiet()
	}
	return q
}

func (q *QAApp) Settle() bool {
	q.t.Helper()
	_, err := q.Call("idle", map[string]any{"window": q.Win, "timeout": settleTimeoutMs})
	if err == nil {
		return true
	}
	if rpc, ok := err.(*rpcError); ok && rpc.Code == codeTimeout {
		return false
	}
	q.t.Fatalf("%v\napp output:\n%s", err, q.Output())
	return false
}

func (q *QAApp) Send(events ...map[string]any) {
	q.t.Helper()
	q.must("input", map[string]any{"window": q.Win, "events": events})
}

func KeyEvent(key string) map[string]any   { return map[string]any{"type": "key", "key": key} }
func TextEvent(text string) map[string]any { return map[string]any{"type": "text", "text": text} }
func ClickEvent(x, y float64) map[string]any {
	return map[string]any{"type": "click", "x": x, "y": y, "clicks": 1}
}

func (q *QAApp) Press(keys ...string) {
	q.t.Helper()
	for _, k := range keys {
		q.Send(KeyEvent(k))
		q.Settle()
	}
}

func (q *QAApp) Measure(do func()) Sample {
	q.t.Helper()
	offset := len(q.Output())
	start := time.Now()
	do()
	settled := q.Settle()
	elapsed := time.Since(start)
	frames := qa.ParseFrames(q.Output()[offset:])
	var sum time.Duration
	for _, f := range frames {
		sum += f.Total
	}
	return Sample{Elapsed: elapsed, Settled: settled, Frames: frames, FrameSum: sum}
}

func (q *QAApp) outputFrom(offset int) string {
	q.output.mu.Lock()
	defer q.output.mu.Unlock()
	data := q.output.buf.Bytes()
	if offset >= len(data) {
		return ""
	}
	return string(data[offset:])
}

func (q *QAApp) outputLen() int {
	q.output.mu.Lock()
	defer q.output.mu.Unlock()
	return q.output.buf.Len()
}

func (q *QAApp) Snapshot() *qa.Tree {
	q.t.Helper()
	var tree qa.Tree
	if err := json.Unmarshal(q.must("tree", map[string]any{"window": q.Win}), &tree); err != nil {
		q.t.Fatal(err)
	}
	var res struct {
		Windows []struct {
			ID            int     `json:"id"`
			ContentBounds qa.Rect `json:"contentBounds"`
		} `json:"windows"`
	}
	if err := json.Unmarshal(q.must("windows", nil), &res); err != nil {
		q.t.Fatal(err)
	}
	for _, w := range res.Windows {
		if w.ID == q.Win {
			tree.Window = qa.Rect{Width: w.ContentBounds.Width, Height: w.ContentBounds.Height}
		}
	}
	tree.Index()
	return &tree
}

func (q *QAApp) WaitFor(what string, match func(qa.Node) bool) qa.Node {
	q.t.Helper()
	var found qa.Node
	q.Eventually(what, func() bool {
		n, ok := q.Snapshot().Find(match)
		found = n
		return ok
	})
	return found
}

func (q *QAApp) Has(match func(qa.Node) bool) bool {
	_, ok := q.Snapshot().Find(match)
	return ok
}

func (q *QAApp) ClickNode(n qa.Node) {
	q.t.Helper()
	q.Send(ClickEvent(n.Bounds.CenterX(), n.Bounds.CenterY()))
	q.Settle()
}

func (q *QAApp) Capture() (image.Image, error) {
	res, err := q.Call("capture", map[string]any{"window": q.Win})
	if err != nil {
		return nil, err
	}
	var shot struct {
		PNG string `json:"png"`
	}
	if err := json.Unmarshal(res, &shot); err != nil {
		return nil, err
	}
	data, err := base64.StdEncoding.DecodeString(shot.PNG)
	if err != nil {
		return nil, err
	}
	return png.Decode(bytes.NewReader(data))
}

func writePNG(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), artifactDirPerm); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), shotFilePerm)
}

func (q *QAApp) SaveShot(name string) {
	q.t.Helper()
	dir := os.Getenv(artifactsEnv)
	if dir == "" {
		return
	}
	img, err := q.Capture()
	if err != nil {
		q.t.Logf("capture %s: %v", name, err)
		return
	}
	if err := writePNG(filepath.Join(dir, name+".png"), img); err != nil {
		q.t.Logf("write %s: %v", name, err)
	}
}

func (q *QAApp) SaveCrops(name string, flags []qa.Flag, window qa.Rect) {
	q.t.Helper()
	dir := os.Getenv(artifactsEnv)
	if dir == "" || len(flags) == 0 || window.Width == 0 {
		return
	}
	img, err := q.Capture()
	if err != nil {
		q.t.Logf("capture crops %s: %v", name, err)
		return
	}
	scale := float64(img.Bounds().Dx()) / window.Width
	type subImager interface {
		SubImage(image.Rectangle) image.Image
	}
	sub, ok := img.(subImager)
	if !ok {
		return
	}
	for i, f := range flags[:min(len(flags), maxCropsPerView)] {
		r := image.Rect(
			int((f.Bounds.X-cropMargin)*scale), int((f.Bounds.Y-cropMargin)*scale),
			int((f.Bounds.Right()+cropMargin)*scale), int((f.Bounds.Bottom()+cropMargin)*scale),
		).Intersect(img.Bounds())
		if r.Empty() {
			continue
		}
		path := filepath.Join(dir, cropDirName, fmt.Sprintf("%s-%02d-%s.png", name, i, f.Rule))
		if err := writePNG(path, sub.SubImage(r)); err != nil {
			q.t.Logf("write crop: %v", err)
		}
	}
}

func (q *QAApp) Report(name, text string) { writeReport(q.t, name, text) }

func artifactDir() string { return os.Getenv(artifactsEnv) }

func writeReport(t *testing.T, name, text string) {
	t.Helper()
	dir := artifactDir()
	if dir == "" || text == "" {
		return
	}
	if err := os.MkdirAll(dir, artifactDirPerm); err != nil {
		t.Logf("report dir: %v", err)
		return
	}
	if err := os.WriteFile(filepath.Join(dir, name+".txt"), []byte(text), shotFilePerm); err != nil {
		t.Logf("write report: %v", err)
	}
}
