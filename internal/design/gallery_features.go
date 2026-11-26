package design

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/detail"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

var galleryScopes = []string{"All", "Files", "Symbols", "Text", "Actions"}

type featureState struct {
	scope, layout int
	rows          ui.ListState
	row           int
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
