package viewer

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/detail"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	backdropChecker = iota
	backdropPlain
	backdropInverse
)

const (
	checkerSize     = 10
	panStep         = 48
	imageViewLabel  = "Image viewer"
	svgPreviewLabel = "SVG preview"
	pointerPrimary  = 0
)

var (
	backdropLabels = []string{"Checker", "Plain", "Invert"}
	errEmptySource = errors.New("empty source")
)

type dragState struct {
	active bool
	x, y   float32
}

type imagePane struct {
	s        *State
	kind     Kind
	limit    int64
	convert  converter
	source   func() ([]byte, error)
	label    string
	gen      uint64
	loading  bool
	err      error
	big      *tooLargeError
	empty    bool
	pic      *decoded
	player   *ui.VideoPlayer
	size     int64
	zoom     zoomState
	backdrop int
	viewW    float32
	viewH    float32
	drag     dragState
	painted  bool
	released bool
}

func newImagePane(s *State, kind Kind) *imagePane {
	p := newImageSurface(s, kind, imageViewLabel)
	p.source = func() ([]byte, error) { return readLimited(s.path, imageFileLimit) }
	p.load()
	return p
}

func newImageSurface(s *State, kind Kind, label string) *imagePane {
	return &imagePane{
		s: s, kind: kind, limit: imagePixelLimit, convert: sipsConvert, label: label,
		zoom: zoomState{fit: true, enlargeToFit: kind == SVG},
	}
}

func (p *imagePane) setText(text string) {
	p.source = func() ([]byte, error) { return []byte(text), nil }
	p.load()
}

func (p *imagePane) load() {
	p.gen++
	gen := p.gen
	source, kind, path, limit, convert := p.source, p.kind, p.s.path, p.limit, p.convert
	p.loading, p.err, p.big, p.empty = true, nil, nil, false
	applog.Go(func() {
		res, size, err := decodeSource(source, kind, path, limit, convert)
		p.s.publish(func() { p.finish(gen, res, size, err) })
	})
	p.s.redraw()
}

func decodeSource(source func() ([]byte, error), kind Kind, path string, limit int64, convert converter) (*decoded, int64, error) {
	data, err := source()
	if err != nil {
		return nil, 0, err
	}
	if len(data) == 0 {
		return nil, 0, errEmptySource
	}
	var res *decoded
	if kind == SVG {
		res, err = decodeSVG(data)
	} else {
		res, err = decodeRaster(data, path, limit, convert)
	}
	return res, int64(len(data)), err
}

func (p *imagePane) finish(gen uint64, res *decoded, size int64, err error) {
	if gen != p.gen {
		return
	}
	p.loading = false
	var big *tooLargeError
	switch {
	case errors.Is(err, errEmptySource):
		p.empty = true
		p.dropPicture()
	case errors.As(err, &big):
		p.big = big
		p.dropPicture()
	case err != nil:
		p.err = err
		p.dropPicture()
	default:
		p.install(res, size)
	}
}

func (p *imagePane) install(res *decoded, size int64) {
	var player *ui.VideoPlayer
	if res.frames != nil {
		var err error
		if player, err = ui.NewVideoPlayer(res.frames); err != nil {
			p.err = fmt.Errorf("play animation: %w", err)
			p.dropPicture()
			return
		}
		player.SetLoop(true)
	}
	if p.pic == nil || p.pic.width != res.width || p.pic.height != res.height {
		p.zoom = zoomState{fit: true, enlargeToFit: p.zoom.enlargeToFit}
	}
	p.dropPlayer()
	p.pic, p.player, p.size = res, player, size
	p.painted, p.released = false, false
}

func (p *imagePane) dropPlayer() {
	if p.player != nil {
		p.player.Close()
		p.player = nil
	}
}

func (p *imagePane) dropPicture() {
	p.dropPlayer()
	p.pic = nil
}

func (p *imagePane) reload() { p.load() }

func (p *imagePane) pause() {
	if p.player != nil {
		p.player.Pause()
	}
}

func (p *imagePane) close() {
	p.gen++
	p.dropPicture()
}

func (p *imagePane) info() string {
	if p.pic == nil {
		return ""
	}
	parts := []string{fmt.Sprintf("%d x %d px", p.pic.width, p.pic.height), formatSize(p.size), p.pic.format}
	if len(p.pic.unsupported) > 0 {
		parts = append(parts, "Not shown: "+strings.Join(p.pic.unsupported, ", "))
	}
	return joinInfo(parts...)
}

func (p *imagePane) body(c *ui.Context) {
	switch {
	case p.big != nil:
		message(c, "This image is too large to show", p.big.Error(), "Open in Preview", p.s.openInPreview)
	case p.err != nil:
		detail.Failed(c, "Cannot show this image", p.err, p.load)
	case p.empty:
		message(c, "Nothing to preview", "Add SVG markup to the file to see it here.", "", nil)
	case p.pic == nil:
		detail.Loading(c, "Loading image")
	default:
		p.canvas(c)
	}
}

func (p *imagePane) controls(c *ui.Context) {
	if p.pic == nil {
		return
	}
	if iconButton(c, "zoom-out", icons.ZoomOut, "Zoom Out", false) {
		p.zoomStep(-1)
	}
	design.Text(c.Key("zoom-level"), fmt.Sprintf("%d%%", percentLabel(p.effective())), token.TextLabel).Width(token.ValueLabelWidth.Px()).TextAlign(ui.Center).SingleLine()
	if iconButton(c, "zoom-in", icons.ZoomIn, "Zoom In", false) {
		p.zoomStep(1)
	}
	if action.Button(c, "Fit", action.ButtonOptions{Variant: action.Ghost, Small: true, Tooltip: "Fit to Window", ID: "fit"}).Clicked() {
		p.zoom = zoomState{fit: true, enlargeToFit: p.zoom.enlargeToFit}
	}
	if action.Button(c, "1:1", action.ButtonOptions{Variant: action.Ghost, Small: true, Tooltip: "Actual Size", Keys: "⌘0", ID: "actual"}).Label("Actual Size").Clicked() {
		p.zoomTo(1)
	}
	action.Segmented(c, "Image background", &p.backdrop, backdropLabels, action.SegmentedOptions{})
	if p.player != nil {
		label, icon := "Pause", icons.Pause
		if !p.player.Playing() {
			label, icon = "Play", icons.Play
		}
		if iconButton(c, "animation", icon, label, false) {
			if p.player.Playing() {
				p.player.Pause()
			} else {
				p.player.Play()
			}
		}
	}
}

func (p *imagePane) canvas(c *ui.Context) {
	pal := token.Of(c)
	if p.painted && !p.released && p.pic.bitmap != nil {
		p.pic.bitmap.ReleasePixels()
		p.released = true
	}
	box := ui.Column(c.Key("canvas")).Grow(1).FillWidth().Center().Clip().Focusable().FocusRing(false).
		Role(ui.RoleImage).Label(p.label).Bind(&p.s.focus)
	if b := box.Bounds(); b.W > 0 && b.H > 0 {
		p.viewW, p.viewH = b.W, b.H
	}
	if p.pannable() {
		box.Cursor(ui.CursorGrab)
	}
	box.HandleInput(p.input).Draw(func(pnt *ui.Painter, r ui.Rect) { p.paintBackdrop(pnt, r, pal) })
	box.Children(func() { p.picture(c) })
}

func (p *imagePane) picture(c *ui.Context) {
	var el ui.Element
	if p.player != nil {
		el = ui.Video(c.Key("picture"), p.player)
	} else if p.pic.svg != nil {
		el = ui.Image(c.Key("picture"), p.pic.svg)
	} else {
		el = ui.Image(c.Key("picture"), p.pic.bitmap)
	}
	el.Label(p.s.name())
	if p.zoom.fit {
		fit := ui.ScaleDown
		if p.zoom.enlargeToFit {
			fit = ui.Contain
		}
		el.Fill().Fit(fit)
		return
	}
	w, h := p.scaledSize()
	el.Size(w, h).Fit(ui.FillBox).Shrink(0).Translate(p.zoom.panX, p.zoom.panY)
}

func (p *imagePane) paintBackdrop(pnt *ui.Painter, r ui.Rect, t *token.Palette) {
	p.painted = true
	switch p.backdrop {
	case backdropPlain:
		pnt.Fill(r, t.Layer.Editor, 0)
	case backdropInverse:
		pnt.Fill(r, t.Text.Primary, 0)
	default:
		pnt.Fill(r, t.Layer.Editor, 0)
		for row, y := 0, float32(0); y < r.H; row, y = row+1, y+checkerSize {
			for col, x := 0, float32(0); x < r.W; col, x = col+1, x+checkerSize {
				if (row+col)%2 == 1 {
					pnt.Fill(ui.Rect{X: r.X + x, Y: r.Y + y, W: min(checkerSize, r.W-x), H: min(checkerSize, r.H-y)}, t.List.Hover, 0)
				}
			}
		}
	}
}

func (p *imagePane) effective() float32 {
	if !p.zoom.fit {
		return p.zoom.scale
	}
	return fitScale(p.viewW, p.viewH, float32(p.pic.width), float32(p.pic.height), p.zoom.enlargeToFit)
}

func (p *imagePane) scaledSize() (float32, float32) {
	s := p.effective()
	return float32(p.pic.width) * s, float32(p.pic.height) * s
}

func (p *imagePane) pannable() bool {
	if p.zoom.fit {
		return false
	}
	w, h := p.scaledSize()
	return w > p.viewW || h > p.viewH
}

func (p *imagePane) panBy(dx, dy float32) bool {
	if !p.pannable() {
		return false
	}
	w, h := p.scaledSize()
	p.zoom.panX = clampPan(p.zoom.panX+dx, w, p.viewW)
	p.zoom.panY = clampPan(p.zoom.panY+dy, h, p.viewH)
	return true
}

func (p *imagePane) zoomAt(factor, px, py float32) {
	cur := p.effective()
	next := clampScale(cur * factor)
	panX, panY := p.zoom.panX, p.zoom.panY
	if p.zoom.fit {
		panX, panY = 0, 0
	}
	p.zoom.fit = false
	p.zoom.scale = next
	p.zoom.panX = zoomAround(panX, cur, next, px, p.viewW/2)
	p.zoom.panY = zoomAround(panY, cur, next, py, p.viewH/2)
	w, h := p.scaledSize()
	p.zoom.panX = clampPan(p.zoom.panX, w, p.viewW)
	p.zoom.panY = clampPan(p.zoom.panY, h, p.viewH)
}

func (p *imagePane) zoomTo(scale float32) {
	p.zoomAt(scale/p.effective(), p.viewW/2, p.viewH/2)
}

func (p *imagePane) zoomStep(dir int) {
	p.zoomTo(stepStop(p.effective(), dir))
}

func (p *imagePane) input(ev ui.InputEvent) bool {
	switch ev.Kind {
	case ui.InputScroll:
		if ev.Mods&ui.Cmd != 0 {
			p.zoomAt(wheelFactor(ev.DY), ev.X, ev.Y)
			return true
		}
		return p.panBy(-ev.DX, -ev.DY)
	case ui.InputZoom:
		p.zoomAt(ev.Scale, ev.X, ev.Y)
		return true
	case ui.InputPointerDown:
		p.s.focus.Focus()
		p.drag = dragState{active: p.pannable(), x: ev.X, y: ev.Y}
		return p.drag.active
	case ui.InputPointerMove:
		if !p.drag.active {
			return false
		}
		p.panBy(ev.X-p.drag.x, ev.Y-p.drag.y)
		p.drag.x, p.drag.y = ev.X, ev.Y
		return true
	case ui.InputPointerUp:
		held := p.drag.active
		p.drag.active = false
		return held
	case ui.InputKeyDown:
		return p.key(ev)
	}
	return false
}

func (p *imagePane) key(ev ui.InputEvent) bool {
	if ev.Mods&^ui.Shift == ui.Cmd {
		switch ev.Key {
		case ui.KeyEqual:
			p.zoomStep(1)
		case ui.KeyMinus:
			p.zoomStep(-1)
		case ui.Key0:
			p.zoomTo(1)
		default:
			return false
		}
		return true
	}
	if ev.Mods != 0 {
		return false
	}
	switch ev.Key {
	case ui.KeyLeft:
		return p.panBy(panStep, 0)
	case ui.KeyRight:
		return p.panBy(-panStep, 0)
	case ui.KeyUp:
		return p.panBy(0, panStep)
	case ui.KeyDown:
		return p.panBy(0, -panStep)
	}
	return false
}
