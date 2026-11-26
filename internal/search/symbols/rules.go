package symbols

import (
	"regexp"
)

const (
	langGo         = "Go"
	langRust       = "Rust"
	langPython     = "Python"
	langJavaScript = "JavaScript"
	langTypeScript = "TypeScript"
	langJava       = "Java"
	langKotlin     = "Kotlin"
	langC          = "C"
	langCPP        = "C++"
	langCSharp     = "C#"
	langSwift      = "Swift"
	langRuby       = "Ruby"
	langPHP        = "PHP"
	langLua        = "Lua"
	langShell      = "Shell"
	langMarkdown   = "Markdown"
	langYAML       = "YAML"
	langTOML       = "TOML"
	langJSON       = "JSON"
	langCSS        = "CSS"
	langSQL        = "SQL"
	langZig        = "Zig"
	langDart       = "Dart"
	langElixir     = "Elixir"
	langHaskell    = "Haskell"
	langScala      = "Scala"
	langMakefile   = "Makefile"
	langDockerfile = "Dockerfile"
	langProtobuf   = "Protobuf"
	langGraphQL    = "GraphQL"
	langR          = "R"
)

const (
	indentClosers = ")]}"
	lineSlashes   = "//"
	blockCStart   = "/*"
	blockCEnd     = "*/"
	jsonBaseDepth = 1
)

const (
	identPat     = `[A-Za-z_][A-Za-z0-9_]*`
	jsIdentPat   = `[A-Za-z_$][A-Za-z0-9_$]*`
	namePat      = `(?P<name>` + identPat + `)`
	jsNamePat    = `(?P<name>` + jsIdentPat + `)`
	containerPat = `(?P<container>` + identPat + `)`
	typeTailPat  = `\b[\w\s:,<>]*(?:\{.*)?$`
	typeTokenPat = `(?:[\w.<>\[\]?,&@]+\s+)`
	sqlNamePat   = `(?:[\w"\[\]` + "`" + `]+\.)?["\[` + "`" + `]?(?P<name>\w+)`
)

type scope uint8

const (
	scopeNone scope = iota
	scopeType
	scopeNamespace
	scopeBody
)

type rule struct {
	re        *regexp.Regexp
	kind      Kind
	hint      string
	scope     scope
	hidden    bool
	member    bool
	name      int
	container int
}

type group struct {
	open string
	kind Kind
	item *regexp.Regexp
}

type delim struct {
	start, end string
}

type lang struct {
	rules       []*rule
	groups      []group
	blocks      []delim
	lineComment string
	braces      bool
	brackets    bool
	baseDepth   int
	indent      bool
	singleQuote bool
}

func newRule(kind Kind, hint, pattern string) *rule {
	re := regexp.MustCompile(pattern)
	return &rule{
		re:        re,
		kind:      kind,
		hint:      hint,
		name:      re.SubexpIndex("name"),
		container: re.SubexpIndex("container"),
	}
}

func (r *rule) typeScope() *rule { r.scope = scopeType; return r }

func (r *rule) namespaceScope() *rule { r.scope = scopeNamespace; return r }

func (r *rule) bodyScope() *rule { r.scope = scopeBody; return r }

func (r *rule) hide() *rule { r.hidden = true; return r }

func (r *rule) inType() *rule { r.member = true; return r }

var cBlocks = []delim{{blockCStart, blockCEnd}}

func braceLang(rules []*rule, blocks []delim) *lang {
	return &lang{rules: rules, blocks: blocks, braces: true, lineComment: lineSlashes}
}

func jsLang(rules []*rule) *lang {
	l := braceLang(rules, cBlocks)
	l.singleQuote = true
	return l
}

var languages = map[string]*lang{
	langGo:         {rules: goRules(), groups: goGroups(), blocks: cBlocks},
	langRust:       braceLang(rustRules(), cBlocks),
	langPython:     {rules: pythonRules(), indent: true, blocks: []delim{{`"""`, `"""`}, {"'''", "'''"}}},
	langJavaScript: jsLang(jsRules(false)),
	langTypeScript: jsLang(jsRules(true)),
	langJava:       braceLang(javaRules(), cBlocks),
	langKotlin:     braceLang(kotlinRules(), cBlocks),
	langScala:      braceLang(scalaRules(), cBlocks),
	langC:          braceLang(cRules(), cBlocks),
	langCPP:        braceLang(cppRules(), cBlocks),
	langCSharp:     braceLang(csharpRules(), cBlocks),
	langSwift:      braceLang(swiftRules(), cBlocks),
	langDart:       jsLang(dartRules()),
	langZig:        braceLang(zigRules(), nil),
	langPHP:        jsLang(phpRules()),
	langProtobuf:   braceLang(protobufRules(), cBlocks),
	langRuby:       {rules: rubyRules(), indent: true, blocks: []delim{{"=begin", "=end"}}},
	langElixir:     {rules: elixirRules(), indent: true, blocks: []delim{{`"""`, `"""`}}},
	langLua:        {rules: luaRules(), blocks: []delim{{"--[[", "]]"}}},
	langShell:      {rules: shellRules()},
	langMarkdown:   {rules: markdownRules(), blocks: []delim{{"```", "```"}, {"~~~", "~~~"}}},
	langYAML:       {rules: yamlRules()},
	langTOML:       {rules: tomlRules()},
	langJSON:       {rules: jsonRules(), braces: true, brackets: true, baseDepth: jsonBaseDepth},
	langCSS:        {rules: cssRules(), blocks: cBlocks, braces: true},
	langSQL:        {rules: sqlRules(), blocks: cBlocks},
	langHaskell:    {rules: haskellRules(), blocks: []delim{{"{-", "-}"}}},
	langMakefile:   {rules: makefileRules()},
	langDockerfile: {rules: dockerfileRules()},
	langGraphQL:    {rules: graphqlRules()},
	langR:          {rules: rRules()},
}

func goRules() []*rule {
	typeParams := `(?:\[.*?\])?`
	return []*rule{
		newRule(Method, "func", `^func\s*\(\s*(?:`+identPat+`\s+)?\*?\s*`+containerPat+`(?:\[[^\]]*\])?\s*\)\s*`+namePat),
		newRule(Function, "func", `^func\s+`+namePat),
		newRule(Struct, "struct", `^type\s+`+namePat+typeParams+`\s+struct\b`),
		newRule(Interface, "interface", `^type\s+`+namePat+typeParams+`\s+interface\b`),
		newRule(Type, "type", `^type\s+`+namePat),
		newRule(Const, "const", `^const\s+`+namePat),
		newRule(Var, "var", `^var\s+`+namePat),
	}
}

func goGroups() []group {
	item := regexp.MustCompile(`^\t` + namePat)
	typeItem := regexp.MustCompile(`^\t` + namePat + `\s`)
	return []group{
		{open: "const (", kind: Const, item: item},
		{open: "var (", kind: Var, item: item},
		{open: "type (", kind: Type, item: typeItem},
	}
}

func rustRules() []*rule {
	vis := `(?:pub(?:\([^)]*\))?\s+)?`
	return []*rule{
		newRule(Class, "impl", `^\s*(?:unsafe\s+)?impl\b(?:\s*<.*?>)?\s+(?:[^{]*?\s+for\s+)?`+namePat).typeScope().hide(),
		newRule(Interface, "trait", `^\s*`+vis+`(?:unsafe\s+)?trait\s+`+namePat).typeScope(),
		newRule(Function, "fn", `^\s*`+vis+`(?:(?:const|async|unsafe|default)\s+)*(?:extern\s+(?:"[^"]*"\s+)?)?fn\s+`+namePat),
		newRule(Struct, "struct", `^\s*`+vis+`struct\s+`+namePat),
		newRule(Struct, "union", `^\s*`+vis+`union\s+`+namePat),
		newRule(Enum, "enum", `^\s*`+vis+`enum\s+`+namePat),
		newRule(Type, "type", `^\s*`+vis+`type\s+`+namePat),
		newRule(Const, "const", `^\s*`+vis+`const\s+`+namePat),
		newRule(Var, "static", `^\s*`+vis+`static\s+(?:mut\s+)?`+namePat),
		newRule(Module, "mod", `^\s*`+vis+`mod\s+`+namePat).namespaceScope(),
		newRule(Function, "macro_rules!", `^\s*macro_rules!\s+`+namePat),
	}
}

func pythonRules() []*rule {
	return []*rule{
		newRule(Class, "class", `^\s*class\s+`+namePat).typeScope(),
		newRule(Function, "def", `^\s*(?:async\s+)?def\s+`+namePat).bodyScope(),
		newRule(Const, "=", `^(?P<name>[A-Z][A-Z0-9_]*)\s*(?::[^=]+)?=[^=]`),
	}
}

func jsRules(typescript bool) []*rule {
	declare := `(?:export\s+)?(?:declare\s+)?`
	members := `(?:(?:public|private|protected|static|readonly|override|abstract|async|declare|accessor)\s+)*(?:(?:get|set)\s+)?\*?\s*`
	rules := []*rule{
		newRule(Class, "class", `^\s*`+declare+`(?:default\s+)?(?:abstract\s+)?class\s+`+jsNamePat).typeScope(),
		newRule(Function, "function", `^\s*`+declare+`(?:default\s+)?(?:async\s+)?function\s*\*?\s*`+jsNamePat),
		newRule(Function, "=>", `^\s*(?:export\s+)?(?:const|let|var)\s+`+jsNamePat+`\s*(?::[^=]+)?=\s*(?:async\s+)?(?:function\b|(?:\([^)]*\)|`+jsIdentPat+`)\s*(?::[^=]+)?=>)`),
	}
	if typescript {
		rules = append(rules,
			newRule(Enum, "enum", `^\s*`+declare+`(?:const\s+)?enum\s+`+jsNamePat),
			newRule(Interface, "interface", `^\s*`+declare+`interface\s+`+jsNamePat).typeScope(),
			newRule(Type, "type", `^\s*`+declare+`type\s+`+jsNamePat+`\s*(?:<|=)`),
			newRule(Module, "", `^\s*`+declare+`(?:namespace|module)\s+`+jsNamePat).namespaceScope(),
		)
	}
	return append(rules,
		newRule(Const, "const", `^\s*(?:export\s+)?const\s+`+jsNamePat),
		newRule(Var, "", `^\s*(?:export\s+)?(?:let|var)\s+`+jsNamePat),
		newRule(Function, "(", `^\s+`+members+jsNamePat+`\s*\??\s*(?:<[^>(]*>)?\s*\(`).inType(),
		newRule(Function, "=>", `^\s+`+members+jsNamePat+`\s*[?!]?\s*(?::[^=]+)?=\s*(?:async\s+)?(?:\([^)]*\)|`+jsIdentPat+`)\s*(?::[^=]+)?=>`).inType(),
		newRule(Var, ";", `^\s+`+members+jsNamePat+`[?!]?\s*(?::[^;=]*)?(?:=.*)?;\s*(?://.*)?$`).inType(),
	)
}

func javaRules() []*rule {
	annotations := `(?:@[A-Za-z_][\w.]*(?:\([^)]*\))?\s+)*`
	mods := `(?:(?:public|private|protected|static|final|abstract|sealed|non-sealed|strictfp)\s+)*`
	head := `^\s*` + annotations + mods
	return []*rule{
		newRule(Class, "class", head+`class\s+`+namePat).typeScope(),
		newRule(Interface, "interface", head+`@?interface\s+`+namePat).typeScope(),
		newRule(Enum, "enum", head+`enum\s+`+namePat).typeScope(),
		newRule(Class, "record", head+`record\s+`+namePat).typeScope(),
		newRule(Function, "(", `^\s*`+annotations+`(?:`+typeTokenPat+`)+`+namePat+`\s*\(`).inType(),
		newRule(Var, ";", `^\s*`+annotations+`(?:`+typeTokenPat+`)+`+namePat+`\s*(?:=[^;]*)?;`).inType(),
	}
}

func kotlinRules() []*rule {
	mods := `(?:(?:public|private|protected|internal|open|abstract|override|final|suspend|inline|operator|infix|tailrec|external|lateinit|data|sealed|annotation|value|inner|expect|actual|@[A-Za-z_]\w*(?:\([^)]*\))?)\s+)*`
	head := `^\s*` + mods
	return []*rule{
		newRule(Enum, "enum", head+`enum\s+class\s+`+namePat).typeScope(),
		newRule(Class, "class", head+`class\s+`+namePat).typeScope(),
		newRule(Interface, "interface", head+`(?:fun\s+)?interface\s+`+namePat).typeScope(),
		newRule(Class, "object", head+`object\s+`+namePat).typeScope(),
		newRule(Type, "typealias", head+`typealias\s+`+namePat),
		newRule(Function, "fun", head+`fun\s+(?:<[^>]*>\s+)?(?:[\w.<>?,]+\.)?`+namePat+`\s*\(`),
		newRule(Const, "const", head+`const\s+val\s+`+namePat),
		newRule(Var, "", head+`(?:val|var)\s+`+namePat),
	}
}

func scalaRules() []*rule {
	mods := `(?:(?:private|protected|override|abstract|sealed|final|implicit|lazy|case|inline|open|transparent|infix)(?:\[\w+\])?\s+)*`
	head := `^\s*` + mods
	return []*rule{
		newRule(Class, "class", head+`class\s+`+namePat).typeScope(),
		newRule(Interface, "trait", head+`trait\s+`+namePat).typeScope(),
		newRule(Class, "object", head+`object\s+`+namePat).typeScope(),
		newRule(Enum, "enum", head+`enum\s+`+namePat).typeScope(),
		newRule(Type, "type", head+`type\s+`+namePat),
		newRule(Function, "def", head+`def\s+`+namePat),
		newRule(Const, "val", head+`val\s+`+namePat),
		newRule(Var, "var", head+`var\s+`+namePat),
	}
}

func cDefineRules() []*rule {
	return []*rule{
		newRule(Function, "define", `^\s*#\s*define\s+`+namePat+`\(`),
		newRule(Const, "define", `^\s*#\s*define\s+`+namePat),
	}
}

func cTypeRules() []*rule {
	tail := typeTailPat
	return []*rule{
		newRule(Struct, "struct", `^\s*(?:typedef\s+)?struct\s+`+namePat+tail).typeScope(),
		newRule(Struct, "union", `^\s*(?:typedef\s+)?union\s+`+namePat+tail).typeScope(),
		newRule(Enum, "enum", `^\s*(?:typedef\s+)?enum\s+(?:class\s+|struct\s+)?`+namePat+tail),
	}
}

func cRules() []*rule {
	rules := append(cDefineRules(), cTypeRules()...)
	return append(rules, newRule(Function, "(", `^(?:[\w*]+\s+)+[*\s]*`+namePat+`\s*\(`))
}

func cppRules() []*rule {
	rules := append(cDefineRules(),
		newRule(Module, "namespace", `^\s*(?:inline\s+)?namespace\s+(?P<name>`+identPat+`(?:::`+identPat+`)*)`).namespaceScope(),
		newRule(Module, "", `^\s*(?:namespace|extern\s+"C(?:\+\+)?")\s*\{`).namespaceScope().hide(),
		newRule(Class, "class", `^\s*(?:template\s*<.*>\s*)?class\s+`+namePat+typeTailPat).typeScope(),
		newRule(Class, "@", `^@(?:interface|implementation)\s+`+namePat),
		newRule(Method, "", `^[-+]\s*\([^)]*\)\s*`+namePat),
		newRule(Type, "using", `^\s*using\s+`+namePat+`\s*=`),
	)
	rules = append(rules, cTypeRules()...)
	return append(rules,
		newRule(Method, "::", `^(?:[\w<>,:*&]+\s+)*(?:`+identPat+`::)*`+containerPat+`::~?`+namePat+`\s*\(`),
		newRule(Function, "(", `^\s*(?:[\w<>,:*&]+\s+)+[*&\s]*~?`+namePat+`\s*\(`),
	)
}

func csharpRules() []*rule {
	mods := `(?:(?:public|private|protected|internal|static|abstract|sealed|partial|readonly|virtual|override|async|extern|unsafe|new|file|required)\s+)*`
	head := `^\s*` + mods
	return []*rule{
		newRule(Module, "namespace", `^\s*namespace\s+(?P<name>[\w.]+)`).namespaceScope(),
		newRule(Class, "class", head+`(?:record\s+)?class\s+`+namePat).typeScope(),
		newRule(Struct, "struct", head+`(?:record\s+)?struct\s+`+namePat).typeScope(),
		newRule(Class, "record", head+`record\s+`+namePat).typeScope(),
		newRule(Interface, "interface", head+`interface\s+`+namePat).typeScope(),
		newRule(Enum, "enum", head+`enum\s+`+namePat).typeScope(),
		newRule(Function, "(", `^\s*(?:`+typeTokenPat+`)+`+namePat+`\s*(?:<[^>]*>)?\s*\(`).inType(),
		newRule(Var, "", `^\s*(?:`+typeTokenPat+`)+`+namePat+`\s*(?:\{|=>|=|;|$)`).inType(),
	}
}

func swiftRules() []*rule {
	mods := `(?:(?:public|private|fileprivate|internal|open|final|static|class|override|mutating|nonmutating|required|convenience|lazy|weak|unowned|indirect|nonisolated|dynamic|@[A-Za-z_]\w*(?:\([^)]*\))?)\s+)*`
	head := `^\s*` + mods
	return []*rule{
		newRule(Function, "func", head+`func\s+`+namePat),
		newRule(Function, "init", head+`(?P<name>init)[?!]?\s*[(<]`),
		newRule(Class, "class", head+`class\s+`+namePat).typeScope(),
		newRule(Struct, "struct", head+`struct\s+`+namePat).typeScope(),
		newRule(Enum, "enum", head+`enum\s+`+namePat).typeScope(),
		newRule(Interface, "protocol", head+`protocol\s+`+namePat).typeScope(),
		newRule(Class, "actor", head+`actor\s+`+namePat).typeScope(),
		newRule(Class, "extension", head+`extension\s+`+namePat).typeScope().hide(),
		newRule(Type, "typealias", head+`typealias\s+`+namePat),
		newRule(Const, "let", head+`let\s+`+namePat),
		newRule(Var, "var", head+`var\s+`+namePat),
	}
}

func dartRules() []*rule {
	tokens := `(?:[\w<>?,\[\]@]+\s+)`
	return []*rule{
		newRule(Class, "class", `^\s*(?:(?:abstract|final|base|sealed|interface|mixin)\s+)*class\s+`+namePat).typeScope(),
		newRule(Class, "mixin", `^\s*mixin\s+`+namePat).typeScope(),
		newRule(Enum, "enum", `^\s*enum\s+`+namePat).typeScope(),
		newRule(Type, "typedef", `^\s*typedef\s+`+namePat),
		newRule(Function, "(", `^\s*`+tokens+`*(?:`+identPat+`\.)?`+namePat+`\s*(?:<[^>(]*>)?\s*\(`),
	}
}

func zigRules() []*rule {
	declared := `^\s*(?:pub\s+)?(?:const|var)\s+` + namePat + `\s*=\s*(?:packed\s+|extern\s+)*`
	return []*rule{
		newRule(Function, "fn", `^\s*(?:pub\s+)?(?:(?:inline|noinline|export)\s+|extern\s+(?:"[^"]*"\s+)?)*fn\s+`+namePat),
		newRule(Struct, "struct", declared+`struct\b`).typeScope(),
		newRule(Struct, "union", declared+`union\b`).typeScope(),
		newRule(Enum, "enum", declared+`enum\b`).typeScope(),
		newRule(Const, "const", `^\s*(?:pub\s+)?(?:export\s+)?const\s+`+namePat+`\s*(?::[^=]+)?=\s*[^@\s]`),
		newRule(Var, "var", `^\s*(?:pub\s+)?(?:export\s+)?var\s+`+namePat),
	}
}

func phpRules() []*rule {
	mods := `(?:(?:abstract|final|public|private|protected|static|readonly)\s+)*`
	return []*rule{
		newRule(Class, "class", `^\s*(?:(?:abstract|final|readonly)\s+)*class\s+`+namePat).typeScope(),
		newRule(Interface, "interface", `^\s*interface\s+`+namePat).typeScope(),
		newRule(Class, "trait", `^\s*trait\s+`+namePat).typeScope(),
		newRule(Enum, "enum", `^\s*enum\s+`+namePat).typeScope(),
		newRule(Module, "namespace", `^\s*namespace\s+(?P<name>[\w\\]+)`),
		newRule(Function, "function", `^\s*`+mods+`function\s+&?\s*`+namePat),
		newRule(Const, "const", `^\s*`+mods+`const\s+(?:`+identPat+`\s+)?`+namePat+`\s*=`),
		newRule(Var, "$", `^\s*(?:public|private|protected)\s+(?:static\s+)?(?:readonly\s+)?(?:\??[\w\\|]+\s+)?\$(?P<name>\w+)`).inType(),
	}
}

func protobufRules() []*rule {
	return []*rule{
		newRule(Struct, "message", `^\s*message\s+`+namePat).typeScope(),
		newRule(Enum, "enum", `^\s*enum\s+`+namePat),
		newRule(Interface, "service", `^\s*service\s+`+namePat).typeScope(),
		newRule(Function, "rpc", `^\s*rpc\s+`+namePat).inType(),
	}
}

func rubyRules() []*rule {
	return []*rule{
		newRule(Class, "class", `^\s*class\s+(?P<name>[A-Z]\w*(?:::\w+)*)`).typeScope(),
		newRule(Module, "module", `^\s*module\s+(?P<name>[A-Z]\w*(?:::\w+)*)`).typeScope(),
		newRule(Function, "def", `^\s*def\s+(?:self\.)?(?P<name>[A-Za-z_]\w*[?!=]?)`).bodyScope(),
		newRule(Const, "=", `^\s*(?P<name>[A-Z][A-Z0-9_]*)\s*=[^=~>]`),
	}
}

func elixirRules() []*rule {
	return []*rule{
		newRule(Module, "defmodule", `^\s*defmodule\s+(?P<name>[A-Z][\w.]*)`).namespaceScope(),
		newRule(Interface, "defprotocol", `^\s*defprotocol\s+(?P<name>[A-Z][\w.]*)`).namespaceScope(),
		newRule(Function, "def", `^\s*(?:defp?|defmacrop?|defguardp?|defdelegate)\s+(?P<name>[a-z_]\w*[?!]?)`).bodyScope(),
	}
}

func luaRules() []*rule {
	qualified := `(?:(?P<container>[A-Za-z_][\w.]*)[.:])?`
	return []*rule{
		newRule(Function, "function", `^\s*(?:local\s+)?function\s+`+qualified+namePat+`\s*\(`),
		newRule(Function, "function", `^\s*(?:local\s+)?(?:(?P<container>[A-Za-z_][\w.]*)\.)?`+namePat+`\s*=\s*function\s*\(`),
	}
}

func shellRules() []*rule {
	shellName := `(?P<name>[A-Za-z_][\w:.-]*)`
	return []*rule{
		newRule(Function, "function", `^\s*function\s+`+shellName),
		newRule(Function, "(", `^\s*`+shellName+`\s*\(\s*\)`),
	}
}

func markdownRules() []*rule {
	return []*rule{newRule(Heading, "#", `^ {0,3}#{1,6}\s+(?P<name>\S.*?)(?:\s+#+)?\s*$`)}
}

func yamlRules() []*rule {
	return []*rule{newRule(Key, ":", `^(?P<name>[A-Za-z_][\w.-]*)\s*:(?:\s|$)`)}
}

func tomlRules() []*rule {
	return []*rule{newRule(Table, "[", `^\s*\[\[?(?P<name>[^\]\s][^\]]*?)\]\]?\s*(?:#.*)?$`)}
}

func jsonRules() []*rule {
	return []*rule{newRule(Key, `"`, `^\s*"(?P<name>[^"\\]+)"\s*:`)}
}

func cssRules() []*rule {
	return []*rule{
		newRule(Module, "@", `^\s*(?P<name>@(?:media|supports|layer|container)\b[^{]*?)\s*\{`).namespaceScope(),
		newRule(Selector, "@", `^\s*@(?:-\w+-)?keyframes\s+(?P<name>[\w-]+)`),
		newRule(Selector, "{", `^\s*(?P<name>[^\s@/}{*][^{};]*?)\s*\{\s*(?:/\*.*)?$`),
	}
}

func sqlRules() []*rule {
	create := `(?i)^\s*create\s+(?:or\s+replace\s+)?`
	return []*rule{
		newRule(Table, "", create+`(?:(?:temp|temporary|unlogged|virtual|materialized)\s+)?(?:table|view)\s+(?:if\s+not\s+exists\s+)?`+sqlNamePat),
		newRule(Function, "", create+`(?:definer\s*=\s*\S+\s+)?(?:function|procedure)\s+(?:if\s+not\s+exists\s+)?`+sqlNamePat),
		newRule(Type, "", create+`type\s+`+sqlNamePat),
	}
}

func haskellRules() []*rule {
	return []*rule{
		newRule(Module, "module", `^module\s+(?P<name>[A-Z][\w.]*)`),
		newRule(Type, "", `^(?:data|newtype)\s+(?:family\s+)?(?P<name>[A-Z][\w']*)`),
		newRule(Type, "type", `^type\s+(?:family\s+)?(?P<name>[A-Z][\w']*)`),
		newRule(Interface, "class", `^class\s+(?:[^=]*=>\s*)?(?P<name>[A-Z][\w']*)`),
		newRule(Function, "::", `^(?P<name>[a-z_][\w']*)\s*::`),
	}
}

func makefileRules() []*rule {
	return []*rule{newRule(Target, ":", `^(?P<name>[A-Za-z0-9_%][A-Za-z0-9_.%/+-]*)(?:\s+[^\s:=#]+)*\s*:(?:[^=:]|:[^=]|$)`)}
}

func dockerfileRules() []*rule {
	return []*rule{newRule(Target, "", `(?i)^\s*FROM\s+(?:--\S+\s+)*\S+\s+AS\s+(?P<name>[\w.-]+)`)}
}

func graphqlRules() []*rule {
	return []*rule{
		newRule(Struct, "type", `^\s*(?:extend\s+)?type\s+`+namePat),
		newRule(Struct, "input", `^\s*(?:extend\s+)?input\s+`+namePat),
		newRule(Interface, "interface", `^\s*(?:extend\s+)?interface\s+`+namePat),
		newRule(Enum, "enum", `^\s*(?:extend\s+)?enum\s+`+namePat),
		newRule(Type, "union", `^\s*(?:extend\s+)?union\s+`+namePat),
		newRule(Type, "scalar", `^\s*scalar\s+`+namePat),
	}
}

func rRules() []*rule {
	return []*rule{newRule(Function, "function", `^(?P<name>[A-Za-z_.][\w.]*)\s*(?:<-|=)\s*function\b`)}
}
