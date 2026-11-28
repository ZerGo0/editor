package token

import "github.com/egoist/mygo/ui"

type Palette struct {
	Dark     bool
	Layer    Layers
	Text     Texts
	Icon     ui.Color
	Accent   Accents
	Control  Controls
	Border   Borders
	List     Lists
	Tab      Tabs
	Bar      Bars
	Menu     Menus
	Badge    Badges
	Key      Keys
	Severity Severities
	Git      GitColors
	Diff     DiffColors
	Syntax   Syntax
	Shadow   ui.Color
	Avatar   [avatarColors]ui.Color
	OnAvatar ui.Color
	skeleton bool
}

type Layers struct {
	TitleBar      ui.Color
	ActivityBar   ui.Color
	SideBar       ui.Color
	Editor        ui.Color
	Panel         ui.Color
	SectionHeader ui.Color
	StatusBar     ui.Color
	TabBar        ui.Color
	Input         ui.Color
	Menu          ui.Color
	Widget        ui.Color
	QuickInput    ui.Color
}

type Texts struct {
	Primary   ui.Color
	Strong    ui.Color
	Secondary ui.Color
	Muted     ui.Color
	Disabled  ui.Color
	Link      ui.Color
	Selection ui.Color
}

type Accents struct {
	Base  ui.Color
	Hover ui.Color
	Text  ui.Color
	Focus ui.Color
}

type Controls struct {
	Hover              ui.Color
	Pressed            ui.Color
	Toggled            ui.Color
	ToggleActive       ui.Color
	ToggleActiveBorder ui.Color
	ButtonBorder       ui.Color
	Secondary          ui.Color
	SecondaryHover     ui.Color
	SecondaryText      ui.Color
	Checkbox           ui.Color
	Scrollbar          ui.Color
}

type Borders struct {
	Default ui.Color
	Input   ui.Color
	Widget  ui.Color
	Section ui.Color
}

type Lists struct {
	Hover                    ui.Color
	ActiveSelection          ui.Color
	ActiveSelectionText      ui.Color
	ActiveSelectionSecondary ui.Color
	InactiveSelection        ui.Color
	FocusOutline             ui.Color
	Drop                     ui.Color
	Highlight                ui.Color
	FocusHighlight           ui.Color
	IndentGuide              ui.Color
	PickSelection            ui.Color
	PickSelectionText        ui.Color
	PickHighlight            ui.Color
	SelectionOutline         ui.Color
	HoverOutline             ui.Color
}

type Tabs struct {
	Active          ui.Color
	Inactive        ui.Color
	Hover           ui.Color
	Border          ui.Color
	ActiveBorderTop ui.Color
	ActiveText      ui.Color
	InactiveText    ui.Color
}

type Bars struct {
	StatusText          ui.Color
	StatusHover         ui.Color
	CommandCenter       ui.Color
	CommandCenterHover  ui.Color
	CommandCenterBorder ui.Color
	ActivityActive      ui.Color
	ActivityInactive    ui.Color
	ActivityIndicator   ui.Color
}

type Menus struct {
	Selection     ui.Color
	SelectionText ui.Color
	Separator     ui.Color
	Border        ui.Color
}

type Badges struct {
	Background ui.Color
	Text       ui.Color
	Accent     ui.Color
	AccentText ui.Color
}

type Keys struct {
	Background ui.Color
	Border     ui.Color
	Bottom     ui.Color
	Text       ui.Color
}

type Severities struct {
	Error             ui.Color
	Warning           ui.Color
	Info              ui.Color
	Success           ui.Color
	ErrorBackground   ui.Color
	ErrorBorder       ui.Color
	WarningBackground ui.Color
	WarningBorder     ui.Color
	InfoBackground    ui.Color
	InfoBorder        ui.Color
}

type GitColors struct {
	Added     ui.Color
	Modified  ui.Color
	Deleted   ui.Color
	Renamed   ui.Color
	Untracked ui.Color
	Ignored   ui.Color
	Conflict  ui.Color
	Submodule ui.Color
}

type GitStatus uint8

const (
	GitNone GitStatus = iota
	GitAdded
	GitModified
	GitDeleted
	GitRenamed
	GitUntracked
	GitIgnored
	GitConflict
	GitSubmodule
	GitCopied
)

type gitStatusText struct{ letter, name string }

var gitStatusTexts = map[GitStatus]gitStatusText{
	GitAdded:     {"A", "Added"},
	GitModified:  {"M", "Modified"},
	GitDeleted:   {"D", "Deleted"},
	GitRenamed:   {"R", "Renamed"},
	GitUntracked: {"U", "Untracked"},
	GitIgnored:   {"I", "Ignored"},
	GitConflict:  {"!", "Conflict"},
	GitSubmodule: {"S", "Submodule"},
	GitCopied:    {"C", "Copied"},
}

func (s GitStatus) Letter() string { return gitStatusTexts[s].letter }

func (s GitStatus) Name() string { return gitStatusTexts[s].name }

func (p *Palette) GitColor(s GitStatus) ui.Color {
	switch s {
	case GitAdded, GitCopied:
		return p.Git.Added
	case GitModified:
		return p.Git.Modified
	case GitDeleted:
		return p.Git.Deleted
	case GitRenamed:
		return p.Git.Renamed
	case GitUntracked:
		return p.Git.Untracked
	case GitIgnored:
		return p.Git.Ignored
	case GitConflict:
		return p.Git.Conflict
	case GitSubmodule:
		return p.Git.Submodule
	}
	return p.Text.Primary
}

type DiffColors struct {
	InsertedLine   ui.Color
	RemovedLine    ui.Color
	InsertedText   ui.Color
	RemovedText    ui.Color
	GutterAdded    ui.Color
	GutterModified ui.Color
	GutterDeleted  ui.Color
}

func (p *Palette) Fill(hovered, pressed bool) ui.Color {
	switch {
	case pressed:
		return p.Control.Pressed
	case hovered:
		return p.Control.Hover
	}
	return ui.Transparent
}

func (p *Palette) Tone(t Tone) ui.Color {
	switch t {
	case ToneError:
		return p.Severity.Error
	case ToneWarning:
		return p.Severity.Warning
	case ToneInfo:
		return p.Severity.Info
	case ToneSuccess:
		return p.Severity.Success
	}
	return p.Text.Secondary
}

func (p *Palette) AvatarColor(name string) ui.Color {
	var h uint32
	for _, r := range name {
		h = h*avatarHashPrime + uint32(r)
	}
	return p.Avatar[h%avatarColors]
}

type Tone uint8

const (
	ToneNeutral Tone = iota
	ToneInfo
	ToneSuccess
	ToneWarning
	ToneError
)

func (t Tone) String() string {
	switch t {
	case ToneInfo:
		return "info"
	case ToneSuccess:
		return "success"
	case ToneWarning:
		return "warning"
	case ToneError:
		return "error"
	}
	return "neutral"
}
