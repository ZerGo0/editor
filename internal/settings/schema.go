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
	Labels      map[string]string
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
	ControlCharacters    = "editor.renderControlCharacters"
	UnicodeInvisible     = "editor.unicodeHighlight.invisibleCharacters"
	UnicodeAmbiguous     = "editor.unicodeHighlight.ambiguousCharacters"
	UnicodeNonBasicASCII = "editor.unicodeHighlight.nonBasicASCII"
	UnicodeAllowedChars  = "editor.unicodeHighlight.allowedCharacters"
	UnicodeAllowedLocale = "editor.unicodeHighlight.allowedLocales"
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
	AutoDetectContrast   = "window.autoDetectHighContrast"
	GitAutoFetch         = "git.autofetch"
	GitPullMode          = "git.pullMode"
	GitLogPageSize       = "git.logPageSize"
	DiffIgnoreWhitespace = "diff.ignoreWhitespace"
	DiffShowMoves        = "diff.showMoves"
	DiffSideBySide       = "diffEditor.renderSideBySide"
	IssueBranchTitle     = "githubIssues.issueBranchTitle"
	IssueAssignWorking   = "githubIssues.assignWhenWorking"
	IssueCommitFormat    = "githubIssues.workingIssueFormatScm"
	IssueQueries         = "githubIssues.queries"
	IssueLinkBranch      = "githubIssues.linkBranchOnGitHub"
)

const (
	ThemeSystem    = "System"
	ThemeLight     = "Light"
	ThemeDark      = "Dark"
	ThemeHighDark  = "High Contrast Dark"
	ThemeHighLight = "High Contrast Light"

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

	UnicodeOn        = "true"
	UnicodeOff       = "false"
	UnicodeUntrusted = "inUntrustedWorkspace"
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
	{Key: FontFamily, Title: "Font Family", Description: "Controls the font families of the text editor, separated by commas, for example 'Hack', Menlo. The first installed family is used. Empty, or no installed family, uses the bundled Hack.", Category: CategoryEditor, Kind: String, Default: ""},
	{Key: FontSize, Title: "Font Size", Description: "Controls the font size of the text editor, in points.", Category: CategoryEditor, Kind: Int, Default: defaultFontSize, Min: minFontSize, Max: maxFontSize},
	{Key: LineHeight, Title: "Line Height", Description: "Controls the height of a line as a multiple of the font size.", Category: CategoryEditor, Kind: Number, Default: defaultLineHeight, Min: minLineHeight, Max: maxLineHeight},
	{Key: DetectIndentation, Title: "Detect Indentation", Description: "Controls whether to use the indentation of the language and the file. When off, Tab Size and Insert Spaces apply.", Category: CategoryEditor, Kind: Bool, Default: true},
	{Key: TabSize, Title: "Tab Size", Description: "Controls the number of columns a tab takes.", Category: CategoryEditor, Kind: Int, Default: defaultTabSize, Min: 1, Max: maxTabSize},
	{Key: InsertSpaces, Title: "Insert Spaces", Description: "Controls whether to insert spaces when Tab is pressed.", Category: CategoryEditor, Kind: Bool, Default: true},
	{Key: WordWrap, Title: "Word Wrap", Description: "Controls how long lines wrap. Wrap at Column wraps at Word Wrap Column. Bounded wraps at the narrower of the editor width and Word Wrap Column.", Category: CategoryEditor, Kind: Enum, Default: WrapOff, Options: []string{WrapOff, WrapOn, WrapColumn, WrapBounded}, Labels: map[string]string{WrapOff: "Off", WrapOn: "On", WrapColumn: "Wrap at Column", WrapBounded: "Bounded"}},
	{Key: WordWrapColumn, Title: "Word Wrap Column", Description: "Controls the column at which lines wrap when Word Wrap is Wrap at Column or Bounded.", Category: CategoryEditor, Kind: Int, Default: defaultWrapColumn, Min: minWrapColumn, Max: maxWrapColumn},
	{Key: DiffWordWrap, Title: "Diff Editor Word Wrap", Description: "Controls how long lines wrap in diff editors. Inherit uses the Word Wrap setting.", Category: CategoryEditor, Kind: Enum, Default: WrapInherit, Options: []string{WrapOff, WrapOn, WrapInherit}, Labels: map[string]string{WrapOff: "Off", WrapOn: "On", WrapInherit: "Inherit"}},
	{Key: LineNumbers, Title: "Line Numbers", Description: "Controls whether and how line numbers show in the gutter.", Category: CategoryEditor, Kind: Enum, Default: NumbersOn, Options: []string{NumbersOn, NumbersRelative, NumbersOff}, Labels: map[string]string{NumbersOn: "On", NumbersRelative: "Relative", NumbersOff: "Off"}},
	{Key: RenderWhitespace, Title: "Render Whitespace", Description: "Controls where spaces and tabs show as dots and arrows.", Category: CategoryEditor, Kind: Enum, Default: WhitespaceSelection, Options: []string{WhitespaceNone, WhitespaceSelection, WhitespaceTrailing, WhitespaceAll}, Labels: map[string]string{WhitespaceNone: "None", WhitespaceSelection: "Selection", WhitespaceTrailing: "Trailing", WhitespaceAll: "All"}},
	{Key: Rulers, Title: "Rulers", Description: "Controls the columns at which vertical guides show, for example 80, 120.", Category: CategoryEditor, Kind: IntList, Default: []int{}},
	{Key: IndentGuides, Title: "Indent Guides", Description: "Controls whether guides show at each indentation level.", Category: CategoryEditor, Kind: Bool, Default: true},
	{Key: ScrollBeyondLastLine, Title: "Scroll Beyond Last Line", Description: "Controls whether the editor scrolls until the last line is at the top.", Category: CategoryEditor, Kind: Bool, Default: true},
	{Key: AutoClosingBrackets, Title: "Auto Closing Brackets", Description: "Controls whether brackets and quotes close as they are typed.", Category: CategoryEditor, Kind: Bool, Default: true},
	{Key: Folding, Title: "Folding", Description: "Controls whether fold markers show in the gutter.", Category: CategoryEditor, Kind: Bool, Default: true},
	{Key: BracketColors, Title: "Bracket Pair Colorization", Description: "Controls whether matching brackets are colored by how deep they nest.", Category: CategoryEditor, Kind: Bool, Default: true},
	{Key: ControlCharacters, Title: "Render Control Characters", Description: "Controls whether control characters, such as the bidirectional text controls of Trojan Source attacks, show as visible marks.", Category: CategoryEditor, Kind: Bool, Default: true},
	{Key: UnicodeInvisible, Title: "Highlight Invisible Characters", Description: "Controls whether characters that take space or change the text but show nothing, such as zero-width spaces, are boxed.", Category: CategoryEditor, Kind: Bool, Default: true},
	{Key: UnicodeAmbiguous, Title: "Highlight Ambiguous Characters", Description: "Controls whether characters that look like basic ASCII characters, such as the Cyrillic с, are boxed. Words in one script are left alone.", Category: CategoryEditor, Kind: Bool, Default: true},
	{Key: UnicodeNonBasicASCII, Title: "Highlight Non-Basic ASCII Characters", Description: "Controls whether every character that is not basic ASCII is boxed. In Untrusted Workspace is off, because every folder is trusted.", Category: CategoryEditor, Kind: Enum, Default: UnicodeUntrusted, Options: []string{UnicodeOn, UnicodeOff, UnicodeUntrusted}},
	{Key: UnicodeAllowedChars, Title: "Allowed Characters", Description: "Controls the characters that are never highlighted, as an object such as {\"с\": true}.", Category: CategoryEditor, Kind: JSON, Default: json.RawMessage("{}")},
	{Key: UnicodeAllowedLocale, Title: "Allowed Locales", Description: "Controls the locales whose letters are not ambiguous, as an object such as {\"ru\": true}. _os is the languages of macOS; _vscode is the language of the app.", Category: CategoryEditor, Kind: JSON, Default: json.RawMessage(`{"_os": true, "_vscode": true}`)},
	{Key: LineHighlight, Title: "Highlight Current Line", Description: "Controls whether the lines that have a cursor are highlighted.", Category: CategoryEditor, Kind: Bool, Default: true},
	{Key: CursorStyle, Title: "Cursor Style", Description: "Controls the shape of the cursor.", Category: CategoryEditor, Kind: Enum, Default: CursorLine, Options: []string{CursorLine, CursorBlock, CursorUnderline}, Labels: map[string]string{CursorLine: "Line", CursorBlock: "Block", CursorUnderline: "Underline"}},
	{Key: AutoSave, Title: "Auto Save", Description: "Controls when modified files save automatically: after a delay, or when the editor loses focus.", Category: CategoryFiles, Kind: Enum, Default: AutoSaveOff, Options: []string{AutoSaveOff, AutoSaveAfterDelay, AutoSaveFocusChange}, Labels: map[string]string{AutoSaveOff: "Off", AutoSaveAfterDelay: "After Delay", AutoSaveFocusChange: "On Focus Change"}},
	{Key: AutoSaveDelay, Title: "Auto Save Delay", Description: "Controls the milliseconds to wait after a change before a file saves, when Auto Save is After Delay.", Category: CategoryFiles, Kind: Int, Default: defaultAutoSaveDelay, Min: minAutoSaveDelay, Max: maxAutoSaveDelay},
	{Key: TrimTrailing, Title: "Trim Trailing Whitespace", Description: "Controls whether spaces and tabs at the end of lines are removed when a file saves.", Category: CategoryFiles, Kind: Bool, Default: false},
	{Key: FinalNewline, Title: "Insert Final Newline", Description: "Controls whether a file ends with a newline when it saves.", Category: CategoryFiles, Kind: Bool, Default: false},
	{Key: HotExit, Title: "Hot Exit", Description: "Controls whether unsaved files are remembered between sessions, so the save prompt is skipped on exit. On Exit applies when the app quits. On Exit and Window Close also applies when a single window closes.", Category: CategoryFiles, Kind: Enum, Default: HotExitOnExit, Options: []string{HotExitOff, HotExitOnExit, HotExitOnClose}, Labels: map[string]string{HotExitOff: "Off", HotExitOnExit: "On Exit", HotExitOnClose: "On Exit and Window Close"}},
	{Key: Exclude, Title: "Excluded Files and Folders", Description: "Controls the glob patterns of files and folders to hide, for example **/node_modules.", Category: CategoryFiles, Kind: StringList, Default: []string{"**/.git", "**/.DS_Store"}},
	{Key: SearchExclude, Title: "Skipped Files and Folders", Description: "Controls the glob patterns of files and folders that Search skips, besides Excluded Files and Folders, for example **/node_modules.", Category: CategorySearch, Kind: StringList, Default: []string{"**/node_modules", "**/bower_components"}},
	{Key: SearchUseIgnoreFiles, Title: "Use Ignore Files", Description: "Controls whether Search skips the files that .gitignore lists when it uses the exclude settings.", Category: CategorySearch, Kind: Bool, Default: true},
	{Key: SearchSmartCase, Title: "Smart Case", Description: "Controls whether Search ignores case when the text has no capital letters, and matches case when it has.", Category: CategorySearch, Kind: Bool, Default: false},
	{Key: ColorTheme, Title: "Color Theme", Description: "Controls the appearance of the windows. System follows the macOS appearance.", Category: CategoryWorkbench, Kind: Enum, Default: ThemeSystem, Options: []string{ThemeSystem, ThemeLight, ThemeDark, ThemeHighDark, ThemeHighLight}, AppWide: true},
	{Key: EnablePreview, Title: "Enable Preview", Description: "Controls whether a single click opens a file in a preview tab, which the next file replaces until you edit or pin it.", Category: CategoryWorkbench, Kind: Bool, Default: true},
	{Key: RestoreWindows, Title: "Restore Windows", Description: "Controls whether the projects of the last session open when the app starts.", Category: CategoryWindow, Kind: Bool, Default: true, AppWide: true},
	{Key: AutoDetectContrast, Title: "Auto Detect High Contrast", Description: "Use a High Contrast theme while macOS Increase Contrast is on.", Category: CategoryWindow, Kind: Bool, Default: true, AppWide: true},
	{Key: ZoomLevel, Title: "Zoom Level", Description: "Controls the zoom of all windows. 0 is 100%, each step up or down zooms 20% larger or smaller.", Category: CategoryWindow, Kind: Int, Default: 0, Min: -maxZoomLevel, Max: maxZoomLevel, AppWide: true},
	{Key: GitAutoFetch, Title: "Autofetch", Description: "Controls the minutes between automatic fetches of all remotes. 0 turns autofetch off.", Category: CategoryGit, Kind: Int, Default: 0, Min: 0, Max: maxAutoFetchMinutes},
	{Key: GitPullMode, Title: "Pull Mode", Description: "Controls how Pull and Update Project join the remote changes.", Category: CategoryGit, Kind: Enum, Default: PullMerge, Options: []string{PullMerge, PullRebase, PullFFOnly}, Labels: map[string]string{PullMerge: "Merge", PullRebase: "Rebase", PullFFOnly: "Fast-Forward Only"}},
	{Key: GitLogPageSize, Title: "Log Page Size", Description: "Controls the number of commits the Git log loads at a time.", Category: CategoryGit, Kind: Int, Default: defaultLogPageSize, Min: minLogPageSize, Max: maxLogPageSize},
	{Key: ScmInputFontSize, Title: "Input Font Size", Description: "Controls the font size of the commit message input, in points.", Category: CategoryGit, Kind: Int, Default: defaultFontSize, Min: minFontSize, Max: maxFontSize},
	{Key: DiffIgnoreWhitespace, Title: "Ignore Whitespace", Description: "Controls which whitespace changes diffs hide.", Category: CategoryDiff, Kind: Enum, Default: DiffWhitespaceNone, Options: []string{DiffWhitespaceNone, DiffWhitespaceTrim, DiffWhitespaceIgnore, DiffWhitespaceIgnoreBlank}, AppWide: true, Labels: map[string]string{DiffWhitespaceNone: "None", DiffWhitespaceTrim: "Trailing Whitespace", DiffWhitespaceIgnore: "All Whitespace", DiffWhitespaceIgnoreBlank: "All Whitespace and Blank Lines"}},
	{Key: DiffSideBySide, Title: "Render Side by Side", Description: "Controls whether diffs show side by side or inline (unified). The layout you last chose in a diff overrides it.", Category: CategoryDiff, Kind: Bool, Default: true},
	{Key: DiffShowMoves, Title: "Show Moved Code", Description: "Controls whether blocks of code that moved to another place are marked in a diff.", Category: CategoryDiff, Kind: Bool, Default: true, AppWide: true},
	{Key: IssueBranchTitle, Title: "Issue Branch Title", Description: "Controls the branch name for Start Working on Issue. Variables: ${user}, ${issueNumber}, ${issueTitle}, ${sanitizedIssueTitle}, ${owner}, ${repository}. Empty uses ${user}/issue${issueNumber}.", Category: CategoryGitHub, Kind: String, Default: ""},
	{Key: IssueAssignWorking, Title: "Assign When Working", Description: "Controls whether the issue is assigned to you when you start working on it.", Category: CategoryGitHub, Kind: Bool, Default: true},
	{Key: IssueCommitFormat, Title: "Working Issue Commit Message", Description: "Controls the commit message template while you work on an issue. Variables: ${issueTitle}, ${issueNumber}. Empty uses the issue title and a Fixes line.", Category: CategoryGitHub, Kind: String, Default: ""},
	{Key: IssueQueries, Title: "Queries", Description: "Controls the issue queries of the GitHub view: a list of objects with label, query and an optional groupBy. Empty uses My Issues, Created Issues and Recent Issues.", Category: CategoryGitHub, Kind: JSON, Default: json.RawMessage(nil)},
	{Key: IssueLinkBranch, Title: "Link Branch on GitHub", Description: "Controls whether the issue branch is created on GitHub so that the issue links to it.", Category: CategoryGitHub, Kind: Bool, Default: false},
	{Key: MarkdownFontSize, Title: "Preview Font Size", Description: "Controls the font size of the Markdown preview, in points.", Category: CategoryMarkdown, Kind: Int, Default: defaultFontSize, Min: minFontSize, Max: maxFontSize},
}

func (d *Def) OptionLabel(value string) string {
	if label, ok := d.Labels[value]; ok {
		return label
	}
	return value
}

func DefFor(key string) *Def {
	for i := range Defs {
		if Defs[i].Key == key {
			return &Defs[i]
		}
	}
	return nil
}
