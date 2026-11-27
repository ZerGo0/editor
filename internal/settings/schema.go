package settings

import "encoding/json"

type Kind int

const (
	Bool Kind = iota
	Int
	Number
	String
	Enum
	IntList
	StringList
	JSON
)

type Def struct {
	Key         string
	Title       string
	Description string
	Category    string
	Kind        Kind
	Default     any
	Options     []string
	Min, Max    float64
	AppWide     bool
}

const (
	CategoryEditor    = "Text Editor"
	CategoryFiles     = "Files"
	CategorySearch    = "Search"
	CategoryWorkbench = "Workbench"
	CategoryWindow    = "Window"
	CategoryGit       = "Git"
	CategoryDiff      = "Diff Editor"
	CategoryGitHub    = "GitHub Issues"
	CategoryMarkdown  = "Markdown"
)

var Categories = []string{CategoryEditor, CategoryFiles, CategorySearch, CategoryWorkbench, CategoryWindow, CategoryGit, CategoryDiff, CategoryGitHub, CategoryMarkdown}

const (
	FontFamily           = "editor.fontFamily"
	FontSize             = "editor.fontSize"
	ScmInputFontSize     = "scm.inputFontSize"
	MarkdownFontSize     = "markdown.preview.fontSize"
	LineHeight           = "editor.lineHeight"
	TabSize              = "editor.tabSize"
	InsertSpaces         = "editor.insertSpaces"
	DetectIndentation    = "editor.detectIndentation"
	WordWrap             = "editor.wordWrap"
	WordWrapColumn       = "editor.wordWrapColumn"
	DiffWordWrap         = "diffEditor.wordWrap"
	LineNumbers          = "editor.lineNumbers"
	RenderWhitespace     = "editor.renderWhitespace"
	Rulers               = "editor.rulers"
	IndentGuides         = "editor.guides.indentation"
	ScrollBeyondLastLine = "editor.scrollBeyondLastLine"
	AutoClosingBrackets  = "editor.autoClosingBrackets"
	Folding              = "editor.folding"
	BracketColors        = "editor.bracketPairColorization.enabled"
	LineHighlight        = "editor.renderLineHighlight"
	CursorStyle          = "editor.cursorStyle"
	AutoSave             = "files.autoSave"
	AutoSaveDelay        = "files.autoSaveDelay"
	TrimTrailing         = "files.trimTrailingWhitespace"
	FinalNewline         = "files.insertFinalNewline"
	HotExit              = "files.hotExit"
	Exclude              = "files.exclude"
	SearchExclude        = "search.exclude"
	SearchUseIgnoreFiles = "search.useIgnoreFiles"
	SearchSmartCase      = "search.smartCase"
	EnablePreview        = "workbench.editor.enablePreview"
	ColorTheme           = "workbench.colorTheme"
	RestoreWindows       = "window.restoreWindows"
	ZoomLevel            = "window.zoomLevel"
	GitAutoFetch         = "git.autofetch"
	GitPullMode          = "git.pullMode"
	GitLogPageSize       = "git.logPageSize"
	DiffIgnoreWhitespace = "diff.ignoreWhitespace"
	DiffShowMoves        = "diff.showMoves"
	IssueBranchTitle     = "githubIssues.issueBranchTitle"
	IssueAssignWorking   = "githubIssues.assignWhenWorking"
	IssueCommitFormat    = "githubIssues.workingIssueFormatScm"
	IssueQueries         = "githubIssues.queries"
	IssueLinkBranch      = "githubIssues.linkBranchOnGitHub"
)

const (
	ThemeSystem = "System"
	ThemeLight  = "Light"
	ThemeDark   = "Dark"

	WrapOff = "off"
	WrapOn  = "on"

	WrapColumn  = "wordWrapColumn"
	WrapBounded = "bounded"
	WrapInherit = "inherit"

	NumbersOn       = "on"
	NumbersOff      = "off"
	NumbersRelative = "relative"

	WhitespaceNone      = "none"
	WhitespaceSelection = "selection"
	WhitespaceTrailing  = "trailing"
	WhitespaceAll       = "all"

	CursorLine      = "line"
	CursorBlock     = "block"
	CursorUnderline = "underline"

	AutoSaveOff         = "off"
	AutoSaveAfterDelay  = "afterDelay"
	AutoSaveFocusChange = "onFocusChange"

	HotExitOff     = "off"
	HotExitOnExit  = "onExit"
	HotExitOnClose = "onExitAndWindowClose"

	PullMerge  = "merge"
	PullRebase = "rebase"
	PullFFOnly = "ff-only"

	DiffWhitespaceNone        = "none"
	DiffWhitespaceTrim        = "trim"
	DiffWhitespaceIgnore      = "ignore"
	DiffWhitespaceIgnoreBlank = "ignoreBlankLines"
)

const (
	defaultFontSize      = 16
	minFontSize          = 6
	maxFontSize          = 72
	defaultLineHeight    = 1.5
	minLineHeight        = 1
	maxLineHeight        = 3
	defaultTabSize       = 4
	defaultWrapColumn    = 80
	minWrapColumn        = 1
	maxWrapColumn        = 1000
	maxTabSize           = 16
	defaultAutoSaveDelay = 1000
	minAutoSaveDelay     = 100
	maxAutoSaveDelay     = 60000
	maxZoomLevel         = 8
	maxAutoFetchMinutes  = 1440
	defaultLogPageSize   = 300
	minLogPageSize       = 50
	maxLogPageSize       = 5000
)

var Defs = []Def{
	{Key: FontFamily, Title: "Font Family", Description: "Font families of the text editor, separated by commas, for example 'Hack', Menlo. The first installed family is used. Empty, or no installed family, uses the bundled Hack.", Category: CategoryEditor, Kind: String, Default: ""},
	{Key: FontSize, Title: "Font Size", Description: "The font size of the text editor, in points.", Category: CategoryEditor, Kind: Int, Default: defaultFontSize, Min: minFontSize, Max: maxFontSize},
	{Key: LineHeight, Title: "Line Height", Description: "The height of a line as a multiple of the font size.", Category: CategoryEditor, Kind: Number, Default: defaultLineHeight, Min: minLineHeight, Max: maxLineHeight},
	{Key: DetectIndentation, Title: "Detect Indentation", Description: "Use the indentation of the language and the file. When off, Tab Size and Insert Spaces apply.", Category: CategoryEditor, Kind: Bool, Default: true},
	{Key: TabSize, Title: "Tab Size", Description: "The number of columns a tab takes.", Category: CategoryEditor, Kind: Int, Default: defaultTabSize, Min: 1, Max: maxTabSize},
	{Key: InsertSpaces, Title: "Insert Spaces", Description: "Insert spaces when Tab is pressed.", Category: CategoryEditor, Kind: Bool, Default: true},
	{Key: WordWrap, Title: "Word Wrap", Description: "Wrap long lines: off, at the width of the editor, at Word Wrap Column, or at the narrower of the two (bounded).", Category: CategoryEditor, Kind: Enum, Default: WrapOff, Options: []string{WrapOff, WrapOn, WrapColumn, WrapBounded}},
	{Key: WordWrapColumn, Title: "Word Wrap Column", Description: "The column at which lines wrap when Word Wrap is wordWrapColumn or bounded.", Category: CategoryEditor, Kind: Int, Default: defaultWrapColumn, Min: minWrapColumn, Max: maxWrapColumn},
	{Key: DiffWordWrap, Title: "Diff Editor Word Wrap", Description: "Wrap long lines in diff editors: off, on, or inherit the Word Wrap setting.", Category: CategoryEditor, Kind: Enum, Default: WrapInherit, Options: []string{WrapOff, WrapOn, WrapInherit}},
	{Key: LineNumbers, Title: "Line Numbers", Description: "Show line numbers in the gutter.", Category: CategoryEditor, Kind: Enum, Default: NumbersOn, Options: []string{NumbersOn, NumbersRelative, NumbersOff}},
	{Key: RenderWhitespace, Title: "Render Whitespace", Description: "Where spaces and tabs show as dots and arrows.", Category: CategoryEditor, Kind: Enum, Default: WhitespaceSelection, Options: []string{WhitespaceNone, WhitespaceSelection, WhitespaceTrailing, WhitespaceAll}},
	{Key: Rulers, Title: "Rulers", Description: "Columns at which vertical guides show, for example 80, 120.", Category: CategoryEditor, Kind: IntList, Default: []int{}},
	{Key: IndentGuides, Title: "Indent Guides", Description: "Show guides at each indentation level.", Category: CategoryEditor, Kind: Bool, Default: true},
	{Key: ScrollBeyondLastLine, Title: "Scroll Beyond Last Line", Description: "Let the editor scroll until the last line is at the top.", Category: CategoryEditor, Kind: Bool, Default: true},
	{Key: AutoClosingBrackets, Title: "Auto Closing Brackets", Description: "Close brackets and quotes as they are typed.", Category: CategoryEditor, Kind: Bool, Default: true},
	{Key: Folding, Title: "Folding", Description: "Show fold markers in the gutter.", Category: CategoryEditor, Kind: Bool, Default: true},
	{Key: BracketColors, Title: "Bracket Pair Colorization", Description: "Color matching brackets by how deep they nest.", Category: CategoryEditor, Kind: Bool, Default: true},
	{Key: LineHighlight, Title: "Highlight Current Line", Description: "Highlight the lines that have a cursor.", Category: CategoryEditor, Kind: Bool, Default: true},
	{Key: CursorStyle, Title: "Cursor Style", Description: "The shape of the cursor.", Category: CategoryEditor, Kind: Enum, Default: CursorLine, Options: []string{CursorLine, CursorBlock, CursorUnderline}},
	{Key: AutoSave, Title: "Auto Save", Description: "Save modified files automatically: after a delay, or when the editor loses focus.", Category: CategoryFiles, Kind: Enum, Default: AutoSaveOff, Options: []string{AutoSaveOff, AutoSaveAfterDelay, AutoSaveFocusChange}},
	{Key: AutoSaveDelay, Title: "Auto Save Delay", Description: "Milliseconds to wait after a change before a file saves, when Auto Save is afterDelay.", Category: CategoryFiles, Kind: Int, Default: defaultAutoSaveDelay, Min: minAutoSaveDelay, Max: maxAutoSaveDelay},
	{Key: TrimTrailing, Title: "Trim Trailing Whitespace", Description: "Remove spaces and tabs at the end of lines when a file saves.", Category: CategoryFiles, Kind: Bool, Default: false},
	{Key: FinalNewline, Title: "Insert Final Newline", Description: "End the file with a newline when it saves.", Category: CategoryFiles, Kind: Bool, Default: false},
	{Key: HotExit, Title: "Hot Exit", Description: "Controls whether unsaved files are remembered between sessions, allowing the save prompt when exiting the editor to be skipped. off disables hot exit: a prompt shows when a window with unsaved changes closes. onExit triggers hot exit when the app quits (Quit command); a window that closes alone still shows the prompt. onExitAndWindowClose triggers hot exit when the app quits, and also when any window closes.", Category: CategoryFiles, Kind: Enum, Default: HotExitOnExit, Options: []string{HotExitOff, HotExitOnExit, HotExitOnClose}},
	{Key: Exclude, Title: "Exclude", Description: "Glob patterns of files and folders to hide, for example **/node_modules.", Category: CategoryFiles, Kind: StringList, Default: []string{"**/.git", "**/.DS_Store"}},
	{Key: SearchExclude, Title: "Exclude", Description: "Glob patterns of files and folders that Search skips, besides Files: Exclude, for example **/node_modules.", Category: CategorySearch, Kind: StringList, Default: []string{"**/node_modules", "**/bower_components"}},
	{Key: SearchUseIgnoreFiles, Title: "Use Ignore Files", Description: "Skip the files that .gitignore lists when Search uses the exclude settings.", Category: CategorySearch, Kind: Bool, Default: true},
	{Key: SearchSmartCase, Title: "Smart Case", Description: "Search ignoring case when the text has no capital letters, and matching case when it has.", Category: CategorySearch, Kind: Bool, Default: false},
	{Key: ColorTheme, Title: "Color Theme", Description: "The appearance of the windows. System follows the macOS appearance.", Category: CategoryWorkbench, Kind: Enum, Default: ThemeSystem, Options: []string{ThemeSystem, ThemeLight, ThemeDark}, AppWide: true},
	{Key: EnablePreview, Title: "Enable Preview", Description: "A single click opens a file in a preview tab, which the next file replaces until you edit or pin it.", Category: CategoryWorkbench, Kind: Bool, Default: true},
	{Key: RestoreWindows, Title: "Restore Windows", Description: "Open the projects of the last session when the app starts.", Category: CategoryWindow, Kind: Bool, Default: true, AppWide: true},
	{Key: ZoomLevel, Title: "Zoom Level", Description: "The zoom of all windows. 0 is 100%, each step up or down zooms 20% larger or smaller.", Category: CategoryWindow, Kind: Int, Default: 0, Min: -maxZoomLevel, Max: maxZoomLevel, AppWide: true},
	{Key: GitAutoFetch, Title: "Autofetch", Description: "Minutes between automatic fetches of all remotes. 0 turns autofetch off.", Category: CategoryGit, Kind: Int, Default: 0, Min: 0, Max: maxAutoFetchMinutes},
	{Key: GitPullMode, Title: "Pull Mode", Description: "How Pull and Update Project join the remote changes: merge, rebase, or fast-forward only.", Category: CategoryGit, Kind: Enum, Default: PullMerge, Options: []string{PullMerge, PullRebase, PullFFOnly}},
	{Key: GitLogPageSize, Title: "Log Page Size", Description: "The number of commits the Git log loads at a time.", Category: CategoryGit, Kind: Int, Default: defaultLogPageSize, Min: minLogPageSize, Max: maxLogPageSize},
	{Key: ScmInputFontSize, Title: "Input Font Size", Description: "The font size of the commit message input, in points.", Category: CategoryGit, Kind: Int, Default: defaultFontSize, Min: minFontSize, Max: maxFontSize},
	{Key: DiffIgnoreWhitespace, Title: "Ignore Whitespace", Description: "Which whitespace changes diffs hide: none, at line ends (trim), all (ignore), or all plus blank lines.", Category: CategoryDiff, Kind: Enum, Default: DiffWhitespaceNone, Options: []string{DiffWhitespaceNone, DiffWhitespaceTrim, DiffWhitespaceIgnore, DiffWhitespaceIgnoreBlank}, AppWide: true},
	{Key: DiffShowMoves, Title: "Show Moved Code", Description: "Mark blocks of code that moved to another place in a diff.", Category: CategoryDiff, Kind: Bool, Default: true, AppWide: true},
	{Key: IssueBranchTitle, Title: "Issue Branch Title", Description: "The branch name for Start Working on Issue. Variables: ${user}, ${issueNumber}, ${issueTitle}, ${sanitizedIssueTitle}, ${owner}, ${repository}. Empty uses ${user}/issue${issueNumber}.", Category: CategoryGitHub, Kind: String, Default: ""},
	{Key: IssueAssignWorking, Title: "Assign When Working", Description: "Assign the issue to you when you start working on it.", Category: CategoryGitHub, Kind: Bool, Default: true},
	{Key: IssueCommitFormat, Title: "Working Issue Commit Message", Description: "The commit message template while you work on an issue. Variables: ${issueTitle}, ${issueNumber}. Empty uses the issue title and a Fixes line.", Category: CategoryGitHub, Kind: String, Default: ""},
	{Key: IssueQueries, Title: "Queries", Description: "The issue queries of the GitHub view: a list of objects with label, query and an optional groupBy. Empty uses My Issues, Created Issues and Recent Issues.", Category: CategoryGitHub, Kind: JSON, Default: json.RawMessage(nil)},
	{Key: IssueLinkBranch, Title: "Link Branch on GitHub", Description: "Create the issue branch on GitHub so that the issue links to it.", Category: CategoryGitHub, Kind: Bool, Default: false},
	{Key: MarkdownFontSize, Title: "Preview Font Size", Description: "The font size of the Markdown preview, in points.", Category: CategoryMarkdown, Kind: Int, Default: defaultFontSize, Min: minFontSize, Max: maxFontSize},
}

func DefFor(key string) *Def {
	for i := range Defs {
		if Defs[i].Key == key {
			return &Defs[i]
		}
	}
	return nil
}
