package token

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

type Syntax struct {
	Keyword     ui.Color
	Type        ui.Color
	Function    ui.Color
	Property    ui.Color
	Constant    ui.Color
	Number      ui.Color
	String      ui.Color
	Escape      ui.Color
	Regexp      ui.Color
	Comment     ui.Color
	Operator    ui.Color
	Punctuation ui.Color
	Tag         ui.Color
	Attribute   ui.Color
	Meta        ui.Color
	Heading     ui.Color
}

func darkSyntax() Syntax {
	h := ui.Hex
	return Syntax{
		Keyword: h("#569CD6"), Type: h("#4EC9B0"), Function: h("#DCDCAA"), Property: h("#9CDCFE"), Constant: h("#4FC1FF"),
		Number: h("#B5CEA8"), String: h("#CE9178"), Escape: h("#D7BA7D"), Regexp: h("#D16969"), Comment: h("#6A9955"),
		Tag: h("#7EE787"), Attribute: h("#9CDCFE"), Meta: h("#C586C0"), Heading: h("#569CD6"),
	}
}

func lightSyntax() Syntax {
	h := ui.Hex
	return Syntax{
		Keyword: h("#0000FF"), Type: h("#267F99"), Function: h("#795E26"), Property: h("#001080"), Constant: h("#0070C1"),
		Number: h("#098658"), String: h("#A31515"), Escape: h("#EE0000"), Regexp: h("#811F3F"), Comment: h("#008000"),
		Tag: h("#800000"), Attribute: h("#E50000"), Meta: h("#AF00DB"), Heading: h("#800000"),
	}
}

func (s *Syntax) colors() []*ui.Color {
	return []*ui.Color{&s.Keyword, &s.Type, &s.Function, &s.Property, &s.Constant, &s.Number, &s.String, &s.Escape, &s.Regexp,
		&s.Comment, &s.Operator, &s.Punctuation, &s.Tag, &s.Attribute, &s.Meta, &s.Heading}
}

func (p *Palette) enforceSyntax() {
	p.Syntax.Operator, p.Syntax.Punctuation = p.Text.Primary, p.Text.Secondary
	for _, c := range p.Syntax.colors() {
		*c = ensure(*c, ContrastText, p.Layer.Editor)
	}
}

func (p *Palette) strengthenSyntax() {
	for _, c := range p.Syntax.colors() {
		*c = c.Mix(p.Text.Primary, contrastTextMix)
	}
}

func (p *Palette) CodeTheme() *editor.Theme {
	window := p.Theme()
	registry.Lock()
	defer registry.Unlock()
	if t, ok := registry.code[p]; ok {
		return t
	}
	t := editor.DefaultTheme(window)
	s := p.Syntax
	for kind, color := range map[editor.TokenKind]ui.Color{
		editor.TokenKeyword: s.Keyword, editor.TokenType: s.Type, editor.TokenFunction: s.Function, editor.TokenProperty: s.Property,
		editor.TokenConstant: s.Constant, editor.TokenNumber: s.Number, editor.TokenString: s.String, editor.TokenEscape: s.Escape,
		editor.TokenRegexp: s.Regexp, editor.TokenComment: s.Comment, editor.TokenOperator: s.Operator, editor.TokenPunctuation: s.Punctuation,
		editor.TokenTag: s.Tag, editor.TokenAttribute: s.Attribute, editor.TokenMeta: s.Meta, editor.TokenHeading: s.Heading,
		editor.TokenLink: p.Text.Link, editor.TokenCode: s.String, editor.TokenInserted: p.Git.Added, editor.TokenDeleted: p.Git.Deleted,
		editor.TokenInvalid: ensure(p.Severity.Error, ContrastText, p.Layer.Editor), editor.TokenVariable: p.Text.Primary,
	} {
		t.Tokens[kind].Color = color
	}
	registry.code[p] = t
	return t
}
