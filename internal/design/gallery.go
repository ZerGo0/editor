package design

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/chrome"
	"github.com/ZerGo0/editor/internal/design/pane"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	galleryBack    = "workbench.action.navigateBack"
	galleryForward = "workbench.action.navigateForward"
	galleryOpen    = "workbench.action.quickOpen"
	galleryRefresh = "workbench.action.refresh"
	galleryTreeRow = 3
)

type gallery struct {
	checked, unchecked, on, off      bool
	matchCase, wholeWord             bool
	textValue, invalid, query, notes string
	choice                           string
	tab, activity                    int
	list                             ui.ListState
	tree                             ui.OutlineState[string]
	stack                            pane.StackState
	sectionOpen, sectionClosed       bool
	selectedRow, treeRow             int
	features                         featureState
}

func newGallery() *gallery {
	g := &gallery{checked: true, on: true, matchCase: true, invalid: "main.go/", query: "design", textValue: "Fix layout",
		notes: "Adds the design system.\nTokens and components.", choice: "dark", sectionOpen: true, selectedRow: 1, treeRow: galleryTreeRow}
	g.list.Selected = &g.selectedRow
	g.tree.Open.Add("internal")
	g.tree.Open.Add("internal/design")
	g.tree.List.Selected = &g.treeRow
	return g
}

func Gallery(c *ui.Context) ui.Element {
	root := ui.Row(c.Key("design-gallery")).Fill().Gap(0)
	root.Children(func() {
		for _, p := range []*token.Palette{token.Dark(), token.Light()} {
			name := "light"
			if p.Dark {
				name = "dark"
			}
			col := ui.Themed(c.Key(name), p.Theme()).Grow(1).Basis(0).FillHeight().MinWidth(0).Background(p.Layer.SideBar)
			g := *ui.Local(col, "gallery", newGallery)
			col.Children(func() { galleryColumn(c, g) })
		}
	})
	return root
}

func galleryColumn(c *ui.Context, g *gallery) {
	ui.Column(c).Fill().Gap(0).Children(func() {
		galleryChrome(c, g)
		ui.Column(c).FillWidth().Padding(token.PadPanel.Px()).Gap(token.GapSection.Px()).Children(func() {
			galleryButtons(c, g)
			galleryInputs(c, g)
			galleryLists(c, g)
			galleryBadges(c)
			galleryFeedback(c)
			galleryFeatures(c, g)
		})
	})
}

func gallerySection(c *ui.Context, title string, body func()) {
	Stack(c, token.GapStack, func() {
		Text(c, title, token.TextTitle)
		body()
	})
}

func galleryChrome(c *ui.Context, g *gallery) {
	p := token.Of(c)
	ui.Row(c).FillWidth().Height(token.TitleBarHeight.Px()).Padding(0, token.PadRow.Px()).Gap(token.GapTight.Px()).
		AlignItems(ui.Center).Background(p.Layer.TitleBar).BorderWidth(0, 0, token.BorderWidth.Px(), 0).BorderColor(p.Border.Default).
		Children(func() {
			chrome.NavButton(c, icons.ArrowLeft, galleryBack, galleryHistory)
			chrome.NavButton(c, icons.ArrowRight, galleryForward, galleryHistory)
			Spacer(c)
			chrome.CommandCenter(c, "editor", galleryOpen)
			Spacer(c)
			action.IconButton(c, icons.SidebarRight, "Toggle Secondary Side Bar", action.IconOptions{Keys: "⌥⌘B"})
		})
	ui.Column(c).FillWidth().Background(p.Layer.SideBar).Children(func() {
		chrome.ActivityBar(c, "Views", &g.activity, []chrome.Activity{
			{Icon: icons.Explorer, Label: "Explorer", Keys: "⇧⌘E"},
			{Icon: icons.Search, Label: "Search", Keys: "⇧⌘F"},
			{Icon: icons.GitBranch, Label: "Source Control", Keys: "⌃⇧G", Badge: 3},
			{Icon: icons.PullRequest, Label: "Pull Requests"},
		})
		pane.ViewTitle(c, "Explorer", pane.TitleOptions{ShowActions: true, Actions: func(c *ui.Context) {
			action.IconButton(c, icons.FileAdd, "New File", action.IconOptions{})
			action.IconButton(c, icons.FolderAdd, "New Folder", action.IconOptions{})
			action.CommandIconButton(c, icons.Refresh, galleryRefresh)
			action.IconButton(c, icons.CollapseAll, "Collapse Folders in Explorer", action.IconOptions{})
		}})
		pane.Header(c, "Open Editors", &g.sectionOpen, pane.SectionOptions{Count: 2})
		pane.Header(c, "Outline", &g.sectionClosed, pane.SectionOptions{Border: true, Busy: true})
	})
	chrome.TabStrip(c, &g.tab, []chrome.Tab{
		{Label: "gallery.go", Icon: icons.File, Git: token.GitModified},
		{Label: "tokens.md", Icon: icons.File, Dirty: true},
		{Label: "Preview", Icon: icons.Preview, Preview: true},
		{Label: "main.go", Icon: icons.File, Pinned: true},
	}, chrome.TabStripOptions{Label: "Editors", Actions: func(c *ui.Context) {
		action.IconButton(c, icons.Split, "Split Editor Right", action.IconOptions{Keys: "⌘\\"})
		action.IconButton(c, icons.More, "More Actions...", action.IconOptions{})
	}})
	ui.Column(c).FillWidth().Background(p.Layer.Editor).Children(func() {
		chrome.Breadcrumbs(c, "Breadcrumbs", []chrome.Crumb{{Label: "internal"}, {Label: "design"}, {Label: "gallery.go", Icon: icons.File}})
	})
	ui.Row(c).FillWidth().Height(token.StatusBarHeight.Px()).Background(p.Layer.StatusBar).AlignItems(ui.Center).
		BorderWidth(token.BorderWidth.Px(), 0, 0, 0).BorderColor(p.Border.Default).Children(func() {
		chrome.StatusItem(c, "main", chrome.StatusOptions{Icon: icons.GitBranch, Tooltip: "Checkout Branch"})
		chrome.StatusItem(c, "2", chrome.StatusOptions{Icon: icons.Error, Tooltip: "Problems"})
		chrome.StatusItem(c, "1", chrome.StatusOptions{Icon: icons.Warning, Tone: token.ToneWarning, Tooltip: "Warnings"})
		Spacer(c)
		chrome.StatusItem(c, "Ln 12, Col 4", chrome.StatusOptions{Tooltip: "Go to Line/Column"})
		chrome.StatusItem(c, "UTF-8", chrome.StatusOptions{Tooltip: "Select Encoding"})
		chrome.StatusItem(c, "Go", chrome.StatusOptions{Tooltip: "Select Language Mode"})
	})
}

func galleryHistory(m *ui.Menu) {
	action.Item(m, "gallery.go", action.ItemOptions{Icon: icons.File})
	action.Item(m, "tokens.md", action.ItemOptions{Icon: icons.File})
}
