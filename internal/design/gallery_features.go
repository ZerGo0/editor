package design

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/detail"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/filediff"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/picker"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const galleryPickerRowCount = 5

var galleryScopes = []string{"All", "Files", "Symbols", "Text", "Actions"}

var galleryPickerTabs = []picker.Tab{{Title: "All"}, {Title: "Actions"}, {Title: "Files"}, {Title: "Text"}}

type featureState struct {
	scope, layout int
	rows          ui.ListState
	row           int
	pickerTab     int
	pickerRows    picker.ResultsState
	fileOpen      bool
	fileViewed    bool
}

func galleryFeatures(c *ui.Context, g *gallery) {
	f := &g.features
	f.rows.Selected = &f.row
	gallerySection(c, "Scope chips and flat trees", func() {
		ui.Row(c).FillWidth().Gap(token.GapInline.Px()).Wrap().Role(ui.RoleTabList).Label("Search scope").Children(func() {
			for i, name := range galleryScopes {
				count := ""
				if name == "Text" {
					count = "12"
				}
				if action.Chip(c, name, action.ChipOptions{Active: i == f.scope, Count: count, ID: "gallery-scope-" + name}).Clicked() {
					f.scope = i
				}
			}
		})
		rows := []list.Item{
			{Label: "Waiting for My Review", Strong: true, Branch: true, Open: true, Count: 2},
			{Label: "Fix the layout", Depth: 1, Description: "#12 · 3h ago", Leading: galleryPRIcon, Trailing: galleryPRTrailing},
			{Label: "Add caching", Depth: 1, Description: "#31 · 1d ago", Leading: galleryPRIcon, Trailing: galleryPRTrailing},
			{Label: "internal/design", Strong: true, Branch: true, Open: true, Count: 3},
			{Depth: 1, Rich: galleryMatch, Description: "gallery.go:42"},
		}
		ui.Column(c).FillWidth().Height(token.RowHeight.Px() * float32(len(rows))).Children(func() {
			list.View(c, &f.rows, len(rows), list.ViewOptions{Label: "Results", Tree: true}, func(i int) list.Item { return rows[i] })
		})
	})
	gallerySection(c, "Detail page parts", func() {
		Inline(c, token.GapControl, func() {
			action.Segmented(c, "Diff layout", &f.layout, []string{"Split", "Unified"}, action.SegmentedOptions{})
			badge.Tag(c, "bug", badge.TagOptions{Hex: "d73a4a"})
			badge.Tag(c, "docs", badge.TagOptions{Hex: "0075ca"})
			badge.Tag(c, "Merged", badge.TagOptions{Tone: token.ToneInfo})
		})
		detail.Card(c, "Checks", detail.CardOptions{Count: 2}, func() {
			detail.Property(c, "Labels", func(c *ui.Context) { badge.Tag(c, "bug", badge.TagOptions{Hex: "d73a4a"}) }, func(c *ui.Context) {
				action.Button(c, "Edit Labels", action.ButtonOptions{Small: true})
			})
			detail.Comment(c, detail.CommentOptions{Author: "Ada Lovelace", Verb: "commented", Time: "1h ago"}, func() { Text(c, "Looks good to me.", token.TextBody) })
			detail.Activity(c, "Grace Hopper", "closed this pull request", "2h ago")
		})
		feedback.Empty(c, feedback.EmptyOptions{Icon: icons.Github, Title: "Sign in to GitHub", Message: "Run this command in a terminal, then check again:",
			Code: "gh auth login", Action: "Check Again", OnAction: func() {}, Link: "Copy Command", OnLink: func() {}})
	})
	gallerySection(c, "File diff header", func() {
		for _, h := range galleryFileHeaders {
			o := h
			o.Open = f.fileOpen
			o.OnToggle = func() { f.fileOpen = !f.fileOpen }
			o.Viewed, o.OnViewed = f.fileViewed, func(on bool) { f.fileViewed = on }
			o.Actions = func(c *ui.Context) {
				action.IconButton(c, icons.Plus, "Stage Changes", action.IconOptions{})
				action.IconButton(c, icons.File, "Open File", action.IconOptions{})
			}
			filediff.Header(c, o)
		}
	})
	gallerySection(c, "Search popup parts", func() {
		picker.Tabs(c, "Search scope", &f.pickerTab, galleryPickerTabs)
		picker.Results(c, &f.pickerRows, galleryPickerRows, picker.ResultsOptions{Label: "Gallery results", Rows: galleryPickerRowCount})
	})
}

var galleryFileHeaders = []filediff.HeaderOptions{
	{Path: "internal/design/filediff/header.go", Status: token.GitModified, Adds: 24, Dels: 6},
	{Path: "internal/design/new.go", OldPath: "internal/design/old.go", Status: token.GitRenamed, Adds: filediff.CountHidden, Dels: filediff.CountHidden},
}

var galleryPickerRows = []picker.Row{
	{Kind: picker.RowHeader, Title: "Files"},
	{Item: list.Item{Label: "gallery.go", Description: "internal/design", Icon: icons.File, Matches: []int{0, 1}}},
	{Kind: picker.RowMore, Title: "More files (4)"},
	{Kind: picker.RowHeader, Title: "Actions"},
	{Item: list.Item{Label: "Go Back", Icon: icons.ArrowLeft}, Keys: "⌃-"},
}

func galleryPRIcon(c *ui.Context) {
	ui.Icon(c, icons.PullRequest).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(token.Of(c).Tone(token.ToneSuccess)).Shrink(0)
}

func galleryPRTrailing(c *ui.Context) {
	badge.Tag(c, "draft", badge.TagOptions{})
	ui.Icon(c, icons.Check).Size(token.IconSizeSmall.Px(), token.IconSizeSmall.Px()).TextColor(token.Of(c).Tone(token.ToneSuccess)).Shrink(0)
	badge.Avatar(c, "Ada Lovelace", token.AvatarSize)
}

func galleryMatch(s list.Style) []ui.Span {
	return []ui.Span{{Text: "func ", Color: s.Text}, {Text: "Gallery", Color: s.Text, Weight: token.WeightBold}, {Text: "(c *ui.Context)", Color: s.Text}}
}
