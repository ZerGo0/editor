package symbols

type Kind uint8

const (
	Function Kind = iota
	Method
	Class
	Struct
	Interface
	Enum
	Type
	Const
	Var
	Field
	Module
	Heading
	Key
	Selector
	Table
	Target
)

var kindLabels = [...]string{
	Function:  "function",
	Method:    "method",
	Class:     "class",
	Struct:    "struct",
	Interface: "interface",
	Enum:      "enum",
	Type:      "type",
	Const:     "constant",
	Var:       "variable",
	Field:     "field",
	Module:    "module",
	Heading:   "heading",
	Key:       "key",
	Selector:  "selector",
	Table:     "table",
	Target:    "target",
}

const unknownKindLabel = "unknown"

func (k Kind) String() string {
	if int(k) >= len(kindLabels) {
		return unknownKindLabel
	}
	return kindLabels[k]
}
