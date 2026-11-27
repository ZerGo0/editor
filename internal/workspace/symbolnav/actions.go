package symbolnav

type Kind uint8

const (
	Definitions Kind = iota
	Declarations
	TypeDefinitions
	Implementations
	References
)

type Mode uint8

const (
	ModeGo Mode = iota
	ModeSide
	ModePeek
	ModeFindAll
	ModeNext
	ModePrevious
	ModeClose
)

type Action struct {
	ID    string
	Title string
	Kind  Kind
	Mode  Mode
}

var Actions = []Action{
	{ID: "editor.action.revealDefinition", Title: "Go to Definition", Kind: Definitions, Mode: ModeGo},
	{ID: "editor.action.revealDefinitionAside", Title: "Open Definition to the Side", Kind: Definitions, Mode: ModeSide},
	{ID: "editor.action.peekDefinition", Title: "Peek Definition", Kind: Definitions, Mode: ModePeek},
	{ID: "editor.action.revealDeclaration", Title: "Go to Declaration", Kind: Declarations, Mode: ModeGo},
	{ID: "editor.action.peekDeclaration", Title: "Peek Declaration", Kind: Declarations, Mode: ModePeek},
	{ID: "editor.action.goToTypeDefinition", Title: "Go to Type Definition", Kind: TypeDefinitions, Mode: ModeGo},
	{ID: "editor.action.peekTypeDefinition", Title: "Peek Type Definition", Kind: TypeDefinitions, Mode: ModePeek},
	{ID: "editor.action.goToImplementation", Title: "Go to Implementations", Kind: Implementations, Mode: ModeGo},
	{ID: "editor.action.peekImplementation", Title: "Peek Implementations", Kind: Implementations, Mode: ModePeek},
	{ID: "editor.action.goToReferences", Title: "Go to References", Kind: References, Mode: ModeGo},
	{ID: "editor.action.referenceSearch.trigger", Title: "Peek References", Kind: References, Mode: ModePeek},
	{ID: "references-view.findReferences", Title: "Find All References", Kind: References, Mode: ModeFindAll},
	{ID: "goToNextReference", Title: "Go to Next Reference", Mode: ModeNext},
	{ID: "goToPreviousReference", Title: "Go to Previous Reference", Mode: ModePrevious},
	{ID: "closeReferenceSearch", Title: "Close Reference Search", Mode: ModeClose},
}

var kindTitles = [...]string{"Definitions", "Declarations", "Type Definitions", "Implementations", "References"}

var kindNouns = [...]string{"definition", "declaration", "type definition", "implementation", "references"}

func (k Kind) Title() string { return kindTitles[k] }

func (k Kind) Missing(word string) string {
	msg := "No " + kindNouns[k] + " found"
	if word == "" {
		return msg
	}
	return msg + " for '" + word + "'"
}
