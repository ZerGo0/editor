package design

import (
	"errors"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

func galleryButtons(c *ui.Context, g *gallery) {
	gallerySection(c, "Buttons", func() {
		Inline(c, token.GapControl, func() {
			action.Button(c, "Commit", action.ButtonOptions{Variant: action.Primary, Icon: icons.Check, Keys: "⌘↩"})
			action.Button(c, "Discard", action.ButtonOptions{})
			action.Button(c, "Cancel", action.ButtonOptions{Variant: action.Ghost})
			action.Button(c, "Disabled", action.ButtonOptions{Variant: action.Primary, Disabled: true})
			action.Button(c, "Small", action.ButtonOptions{Small: true})
		})
		Inline(c, token.GapControl, func() {
			action.SplitButton(c, "Commit", action.SplitOptions{Variant: action.Primary, Menu: func(m *ui.Menu) {
				action.Item(m, "Commit & Push", action.ItemOptions{})
				action.Item(m, "Commit & Sync", action.ItemOptions{})
			}})
			action.IconButton(c, icons.Refresh, "Refresh", action.IconOptions{Keys: "⌘R"})
			action.IconButton(c, icons.ListView, "View as List", action.IconOptions{Active: true})
			action.IconButton(c, icons.Push, "Push", action.IconOptions{Disabled: true})
			action.MenuIconButton(c, icons.More, "More Actions...", action.IconOptions{}, func(m *ui.Menu) {
				action.Item(m, "Pull", action.ItemOptions{})
			})
			action.ToggleIconButton(c, icons.CaseSensitive, "Match Case", &g.matchCase, action.IconOptions{Keys: "⌥⌘C"})
			action.ToggleIconButton(c, icons.WholeWord, "Match Whole Word", &g.wholeWord, action.IconOptions{Keys: "⌥⌘W"})
			if action.ExpandButton(c, "Toggle Replace", g.expanded, action.IconOptions{}).Clicked() {
				g.expanded = !g.expanded
			}
		})
	})
}

func galleryInputs(c *ui.Context, g *gallery) {
	gallerySection(c, "Inputs", func() {
		field.TextInput(c, "Message", &g.textValue, field.InputOptions{Placeholder: "Message (⌘↩ to commit)"})
		field.TextInput(c, "File name", &g.invalid, field.InputOptions{Error: "A file or folder name cannot end with a slash."})
		field.SearchInput(c, "Search", &g.query, field.SearchOptions{Placeholder: "Search", Toggles: []field.Toggle{
			{Icon: icons.CaseSensitive, Label: "Match Case", Keys: "⌥⌘C", On: &g.matchCase},
			{Icon: icons.Regex, Label: "Use Regular Expression", Keys: "⌥⌘R", On: &g.wholeWord},
		}})
		field.TextArea(c, "Description", &g.notes, field.AreaOptions{MinLines: 2})
		Inline(c, token.GapSection, func() {
			action.Select(c, "Color Theme", &g.choice, []action.Option[string]{{Value: "dark", Label: "Dark Modern"}, {Value: "light", Label: "Light Modern"}}, action.SelectOptions{})
			action.Checkbox(c, "Auto Save", &g.checked, action.ChoiceOptions{})
			action.Checkbox(c, "Word Wrap", &g.unchecked, action.ChoiceOptions{})
			action.Checkbox(c, "Locked", &g.checked, action.ChoiceOptions{Disabled: true, ID: "locked"})
		})
		Inline(c, token.GapSection, func() {
			action.Switch(c, "Minimap", &g.on, action.ChoiceOptions{})
			action.Switch(c, "Sticky Scroll", &g.off, action.ChoiceOptions{})
		})
	})
}

var galleryFiles = []list.Item{
	{Label: "gallery.go", Icon: icons.File, Git: token.GitModified, Description: "internal/design"},
	{Label: "palette.go", Icon: icons.File, Matches: []int{0, 1, 2}, Git: token.GitAdded, Count: 2},
	{Label: "tokens.md", Icon: icons.File, Git: token.GitUntracked},
	{Label: "old.go", Icon: icons.File, Git: token.GitDeleted, Strike: true},
	{Label: "vendor", Icon: icons.Folder, Git: token.GitIgnored, Muted: true},
}

var galleryTree = map[string][]string{
	"":                {"internal", "go.mod"},
	"internal":        {"internal/design", "internal/icons"},
	"internal/design": {"internal/design/token", "internal/design/list"},
}

func galleryLists(c *ui.Context, g *gallery) {
	gallerySection(c, "Lists and trees", func() {
		ui.Row(c).FillWidth().Gap(token.GapGroup.Px()).Children(func() {
			ui.Column(c).Grow(1).Basis(0).Height(token.RowHeight.Px() * float32(len(galleryFiles))).Children(func() {
				list.View(c, &g.list, len(galleryFiles), list.ViewOptions{Label: "Changes"}, func(i int) list.Item {
					it := galleryFiles[i]
					it.Actions = func(c *ui.Context) {
						action.IconButton(c, icons.Undo, "Discard Changes", action.IconOptions{})
						action.IconButton(c, icons.Plus, "Stage Changes", action.IconOptions{})
					}
					return it
				})
			})
			ui.Column(c).Grow(1).Basis(0).Height(token.RowHeight.Px() * float32(len(galleryFiles))).Children(func() {
				list.Tree(c, &g.tree, galleryTree[""], func(k string) []string { return galleryTree[k] }, list.ViewOptions{Label: "Files"},
					func(k string) list.Item {
						icon := icons.File
						if galleryTree[k] != nil {
							icon = icons.Folder
						}
						return list.Item{Label: baseName(k), Icon: icon}
					})
			})
		})
	})
}

func baseName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}

func galleryBadges(c *ui.Context) {
	gallerySection(c, "Badges and labels", func() {
		Inline(c, token.GapControl, func() {
			badge.Count(c, 3, badge.CountOptions{})
			badge.Count(c, 1200, badge.CountOptions{Accent: true})
			badge.Tag(c, "draft", badge.TagOptions{})
			badge.Tag(c, "bug", badge.TagOptions{Tone: token.ToneError})
			badge.Tag(c, "ready", badge.TagOptions{Tone: token.ToneSuccess, Icon: icons.Check})
			badge.Avatar(c, "Ada Lovelace", token.AvatarSize)
			badge.Avatar(c, "Grace Hopper", token.AvatarSize)
			badge.Keys(c, "⌘K ⌘S")
		})
		Inline(c, token.GapInline, func() {
			for s := token.GitAdded; s <= token.GitSubmodule; s++ {
				badge.Decoration(c, s)
			}
		})
	})
}

func galleryFeedback(c *ui.Context) {
	gallerySection(c, "Feedback", func() {
		feedback.ProgressBar(c, "Loading")
		Inline(c, token.GapControl, func() {
			feedback.Spinner(c, "Loading")
			Text(c, "Body text", token.TextBody)
			Text(c, "Label text", token.TextLabel)
			Text(c, "Caption text", token.TextCaption)
			Text(c, "const x = 1", token.TextMono)
		})
		for _, t := range []token.Tone{token.ToneInfo, token.ToneWarning, token.ToneError} {
			feedback.Notice(c, feedback.NoticeOptions{Tone: t, Text: "Git is not installed. Install it to use Source Control.", Action: "Learn More", OnAction: func() {}, OnDismiss: func() {}})
		}
		ui.Row(c).FillWidth().Gap(token.GapGroup.Px()).Children(func() {
			feedback.Empty(c, feedback.EmptyOptions{Icon: icons.GitBranch, Title: "No source control providers", Message: "Open a folder that contains a Git repository.", Action: "Open Folder", OnAction: func() {}}).Grow(1).Basis(0)
			feedback.Error(c, "Could not load pull requests", errors.New("network is unreachable"), func() {}).Grow(1).Basis(0)
		})
		feedback.Loading(c, "Loading pull requests...")
	})
}
