//go:build e2e

package e2e

import (
	"fmt"
	"image"
	"time"

	"github.com/ZerGo0/editor/e2e/qa"
)

const (
	wordWrapKey      = "Alt+Z"
	wrapPixelStride  = 5
	wrapShownTimeout = 2 * time.Second
	wrapShownPoll    = 20 * time.Millisecond
)

func samePixels(a, b image.Image) bool {
	r := a.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y += wrapPixelStride {
		for x := r.Min.X; x < r.Max.X; x += wrapPixelStride {
			if a.At(x, y) != b.At(x, y) {
				return false
			}
		}
	}
	return true
}

func toggleWrap(q *QAApp) {
	q.Send(KeyEvent(wordWrapKey))
	q.Settle()
}

type wrapKit struct {
	base image.Image
}

func (k *wrapKit) prepare(q *QAApp, d DemoProject) (err error) {
	if _, err = focusCodeEditor(q); err != nil {
		return err
	}
	k.base, err = q.Capture()
	return err
}

func (k *wrapKit) expectWrapped(q *QAApp, d DemoProject) error {
	for deadline := time.Now().Add(wrapShownTimeout); time.Now().Before(deadline); time.Sleep(wrapShownPoll) {
		img, err := q.Capture()
		if err != nil {
			return err
		}
		if !samePixels(k.base, img) {
			return nil
		}
	}
	return fmt.Errorf("the editor does not show wrapped lines %s after the key", wrapShownTimeout)
}

func wrapAction(options func(d DemoProject) QAOptions, shown bool) latencyAction {
	k := &wrapKit{}
	a := latencyAction{
		Owner: qa.OwnerWorkspace, IdleFrames: blinkFramesPerSec, Options: options,
		Prepare: k.prepare,
		Reset: func(q *QAApp, d DemoProject, i int) {
			if i > 0 {
				toggleWrap(q)
			}
		},
		Do: func(q *QAApp, d DemoProject, i int) { q.Send(KeyEvent(wordWrapKey)) },
	}
	if shown {
		a.Verify = k.expectWrapped
	}
	return a
}

func scrollWrappedAction(options func(d DemoProject) QAOptions) latencyAction {
	a := scrollAction(options, cmdTop, cmdBottom)
	prepare := a.Prepare
	a.Prepare = func(q *QAApp, d DemoProject) error {
		if err := prepare(q, d); err != nil {
			return err
		}
		toggleWrap(q)
		q.WaitCPUQuiet()
		return nil
	}
	return a
}

func stressWrapActions() []latencyAction {
	long := stressBigFiles[2]
	longOptions := stressOptions(stressOnFile(long.name))
	wideOptions := stressOptions(stressOnFile(stressWideName))
	wide := fmt.Sprintf("%dk-line file with %d characters per line", stressWideLines/1000, stressWideLineChars)
	return []latencyAction{
		wrapAction(longOptions, false).onFixture(stressFixtureFiles, "turn on word wrap in the "+long.label, stressQuietLong, stressBigSamples),
		scrollWrappedAction(longOptions).onFixture(stressFixtureFiles, "scroll to the end of the "+long.label+" with word wrap", stressQuietLong, stressBigSamples),
		wrapAction(wideOptions, true).onFixture(stressFixtureWrap, "turn on word wrap in the "+wide, stressQuietLong, stressBigSamples),
		scrollWrappedAction(wideOptions).onFixture(stressFixtureWrap, "scroll to the end of the "+wide+" with word wrap", stressQuietLong, stressBigSamples),
	}
}
