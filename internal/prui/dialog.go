package prui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo/ui"
)

const (
	dialogWidth      = 460
	dialogTextHeight = 120
	pickerListHeight = 240
	pickerRowHeight  = 28
)

var errCancelled = errors.New("prui: cancelled")

var reviewChoices = []string{"Comment", "Approve", "Request Changes"}

var reviewEvents = []string{github.ReviewComment, github.ReviewApprove, github.ReviewRequestChanges}

type dialogInput struct {
	body    string
	subject string
	choice  int
	checks  []bool
}

type dialogCheck struct {
	label string
	on    bool
}

type dialog struct {
	open         bool
	title        string
	hint         string
	placeholder  string
	action       string
	subjectLabel string
	subject      string
	body         string
	noBody       bool
	choices      []string
	choice       int
	checks       []dialogCheck
	suggest      string
	busy         bool
	err          string
	gen          int
	validate     func(in dialogInput) string
	submit       func(ctx context.Context, in dialogInput) (func(), error)
}

func trimmed(s string) string {
	return strings.TrimSpace(s)
}

func requireBody(message string) func(in dialogInput) string {
	return func(in dialogInput) string {
		if in.body == "" {
			return message
		}
		return ""
	}
}

func (p *PRs) openDialog(dl dialog) {
	dl.open = true
	dl.gen = p.dialog.gen + 1
	p.dialog = dl
}

func (p *PRs) openReviewDialog(d *prData, choice int) {
	be, repo, number, pending := p.be, p.repo, d.number, d.pendingReview
	hint := "Your review is sent with the comments you added in the diffs."
	if pending == "" {
		hint = "Review the changes and send your verdict."
	}
	p.openDialog(dialog{
		title:       fmt.Sprintf("Submit Review for #%d", number),
		hint:        hint,
		placeholder: "Review summary",
		action:      "Submit Review",
		choices:     reviewChoices,
		choice:      choice,
		validate: func(in dialogInput) string {
			event := reviewEvents[in.choice]
			if in.body == "" && (event == github.ReviewRequestChanges || event == github.ReviewComment && pending == "") {
				return "Write a summary for this review."
			}
			return ""
		},
		submit: func(ctx context.Context, in dialogInput) (func(), error) {
			id := pending
			if id == "" {
				var err error
				if id, err = be.StartReview(ctx, repo, number); err != nil {
					return nil, err
				}
			}
			if err := be.SubmitReview(ctx, id, reviewEvents[in.choice], in.body); err != nil {
				return nil, err
			}
			return func() {
				d.pendingReview = ""
				p.h.Notify(fmt.Sprintf("Submitted review for #%d", number))
				p.loadDetail(d)
			}, nil
		},
	})
}

func (p *PRs) submitDialog() {
	dl := &p.dialog
	if dl.busy {
		return
	}
	in := dialogInput{body: trimmed(dl.body), subject: trimmed(dl.subject), choice: dl.choice}
	for _, c := range dl.checks {
		in.checks = append(in.checks, c.on)
	}
	if dl.validate != nil {
		if msg := dl.validate(in); msg != "" {
			dl.err = msg
			return
		}
	}
	dl.busy, dl.err = true, ""
	gen, submit, title := dl.gen, dl.submit, dl.title
	go func() {
		apply, err := submit(p.ctx, in)
		p.post(func() {
			current := gen == p.dialog.gen
			if current {
				p.dialog.busy = false
			}
			switch {
			case errors.Is(err, errCancelled):
			case err != nil && current:
				p.authFailure(err)
				p.dialog.err = errorText(err)
			case err != nil:
				p.h.ShowError(title, err)
			default:
				if current {
					p.dialog.open = false
				}
				if apply != nil {
					apply()
				}
			}
		})
	}()
}

type picker struct {
	open     bool
	title    string
	action   string
	single   bool
	items    []string
	selected map[string]bool
	filter   string
	loading  bool
	busy     bool
	err      string
	gen      int
	apply    func(ctx context.Context, selected []string) (func(), error)
}

func (p *PRs) openPicker(title, action string, single bool, selected []string, load func(ctx context.Context) ([]string, error), apply func(ctx context.Context, selected []string) (func(), error)) {
	gen := p.picker.gen + 1
	pk := picker{open: true, title: title, action: action, single: single, selected: map[string]bool{}, loading: true, gen: gen, apply: apply}
	for _, s := range selected {
		pk.selected[s] = true
	}
	p.picker = pk
	go func() {
		items, err := load(p.ctx)
		p.post(func() {
			if gen != p.picker.gen {
				return
			}
			p.picker.loading = false
			if err != nil {
				p.authFailure(err)
				p.picker.err = errorText(err)
				return
			}
			for s := range p.picker.selected {
				if !slices.Contains(items, s) {
					items = append(items, s)
				}
			}
			slices.Sort(items)
			p.picker.items = items
		})
	}()
}

func (pk *picker) chosen() []string {
	var out []string
	for s, on := range pk.selected {
		if on {
			out = append(out, s)
		}
	}
	slices.Sort(out)
	return out
}

func (pk *picker) visible() []string {
	needle := strings.ToLower(trimmed(pk.filter))
	var out []string
	for _, s := range pk.items {
		if needle == "" || strings.Contains(strings.ToLower(s), needle) {
			out = append(out, s)
		}
	}
	return out
}

func (p *PRs) submitPicker() {
	pk := &p.picker
	if pk.busy || pk.loading {
		return
	}
	pk.busy, pk.err = true, ""
	gen, selected, apply := pk.gen, pk.chosen(), pk.apply
	go func() {
		done, err := apply(p.ctx, selected)
		p.post(func() {
			current := gen == p.picker.gen
			if current {
				p.picker.busy = false
			}
			switch {
			case err != nil && current:
				p.authFailure(err)
				p.picker.err = errorText(err)
			case err != nil:
				p.h.ShowError("Cannot update pull request", err)
			default:
				if current {
					p.picker.open = false
				}
				if done != nil {
					done()
				}
			}
		})
	}()
}

func (p *PRs) Overlay(c *ui.Context) {
	if p.closed {
		return
	}
	if p.dialog.open {
		p.dialogView(c)
	}
	if p.picker.open {
		p.pickerView(c)
	}
}

func (p *PRs) dialogView(c *ui.Context) {
	dl := &p.dialog
	t := c.Theme()
	ui.Modal(c, &dl.open, func() {
		ui.Column(c).Gap(spacingMedium).Width(dialogWidth).Children(func() {
			ui.Text(c, dl.title).Bold()
			if dl.hint != "" {
				mutedText(c, dl.hint)
			}
			if len(dl.choices) > 0 {
				ui.Segmented(c, &dl.choice, dl.choices...).Label("Choice")
			}
			if dl.subjectLabel != "" {
				ui.TextInput(c, &dl.subject).Placeholder(dl.subjectLabel).FillWidth().Label(dl.subjectLabel)
			}
			if !dl.noBody {
				ui.TextArea(c, &dl.body).AutoFocus().Placeholder(dl.placeholder).MinHeight(dialogTextHeight).FillWidth().Label(dl.title)
			}
			if dl.suggest != "" {
				ui.Button(c, "Insert Suggestion").OnClick(func() { dl.body = appendSuggestion(dl.body, dl.suggest) })
			}
			for i := range dl.checks {
				ui.Checkbox(c.Key(i), &dl.checks[i].on, dl.checks[i].label)
			}
			if dl.err != "" {
				ui.Text(c, dl.err).FontSize(fontSmall).TextColor(t.Danger)
			}
			var cancel, ok ui.Element
			ui.Row(c).Gap(spacingMedium).AlignItems(ui.Center).Justify(ui.End).FillWidth().Children(func() {
				if dl.busy {
					ui.Spinner(c).Label("Working")
				}
				cancel = ui.Button(c, "Cancel")
				ok = ui.PrimaryButton(c, dl.action).Disabled(dl.busy)
			})
			if cancel.Clicked() {
				dl.open = false
			}
			if ok.Clicked() {
				p.submitDialog()
			}
		})
	})
	if !dl.open && dl.busy {
		dl.open = true
	}
}

func (p *PRs) pickerView(c *ui.Context) {
	pk := &p.picker
	t := c.Theme()
	ui.Modal(c, &pk.open, func() {
		ui.Column(c).Gap(spacingMedium).Width(dialogWidth).Children(func() {
			ui.Text(c, pk.title).Bold()
			ui.SearchField(c.Key("filter"), &pk.filter).Placeholder("Filter").Label("Filter").FillWidth()
			switch {
			case pk.loading:
				loadingRow(c, "Loading...")
			case pk.err != "" && len(pk.items) == 0:
				ui.Text(c, pk.err).FontSize(fontSmall).TextColor(t.Danger)
			default:
				items := pk.visible()
				if len(items) == 0 {
					mutedText(c, "Nothing matches.")
				}
				ui.Scroll(c).Height(pickerListHeight).FillWidth().Children(func() {
					for _, item := range items {
						on := pk.selected[item]
						if ui.Checkbox(c.Key(item), &on, item).Height(pickerRowHeight).Changed() {
							if pk.single {
								clear(pk.selected)
							}
							pk.selected[item] = on
						}
					}
				})
			}
			if pk.err != "" && len(pk.items) > 0 {
				ui.Text(c, pk.err).FontSize(fontSmall).TextColor(t.Danger)
			}
			var cancel, ok ui.Element
			ui.Row(c).Gap(spacingMedium).AlignItems(ui.Center).Justify(ui.End).FillWidth().Children(func() {
				if pk.busy {
					ui.Spinner(c).Label("Working")
				}
				cancel = ui.Button(c, "Cancel")
				ok = ui.PrimaryButton(c, pk.action).Disabled(pk.busy || pk.loading)
			})
			if cancel.Clicked() {
				pk.open = false
			}
			if ok.Clicked() {
				p.submitPicker()
			}
		})
	})
	if !pk.open && pk.busy {
		pk.open = true
	}
}

func appendSuggestion(body, code string) string {
	block := "```suggestion\n" + code + "\n```"
	if strings.TrimSpace(body) == "" {
		return block
	}
	return strings.TrimRight(body, "\n") + "\n\n" + block
}
