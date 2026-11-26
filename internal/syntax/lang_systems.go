package syntax

import "github.com/egoist/mygo/ui/editor"

var dlang = &editor.Language{
	Name: "D", Extensions: []string{".d", ".di"}, Interpreters: []string{"rdmd"},
	LineComment: "//", BlockComment: [2]string{"/*", "*/"}, Brackets: brackets, AutoClose: autoClose("\"", "'", "`"), TabSize: 4,
	Tokenizer: &spec{
		lineComments: slash, blocks: []blockComment{{open: "/+", close: "+/", nested: true}, {open: "/*", close: "*/"}},
		strings: strs(dq, charLit, stringSpec{open: "`", close: "`", multiline: true}),
		words: words(
			kw("abstract alias align asm assert auto body break case cast catch class const continue debug default delegate delete deprecated do else enum export extern final finally for foreach foreach_reverse function goto if immutable import in inout interface invariant is lazy macro mixin module new nothrow out override package pragma private protected public pure ref return scope shared static struct super switch synchronized template this throw try typeid typeof union unittest version while with"),
			types("void bool byte ubyte short ushort int uint long ulong float double real char wchar dchar string size_t"),
			consts("true false null")),
		capitalTypes: true, calls: true,
	},
}

var fortran = &editor.Language{
	Name: "Fortran", Extensions: []string{".f", ".for", ".f77", ".f90", ".f95", ".f03", ".f08"},
	LineComment: "!", Brackets: brackets, AutoClose: autoClose("\"", "'"), TabSize: 2,
	IncreaseIndent: re(`(?i)^\s*(program|subroutine|function|module|interface|type\b[^(]|do\b|else|if\b.*\bthen\s*$)`),
	DecreaseIndent: re(`(?i)^\s*(end|else|contains)\b`),
	Tokenizer: &spec{
		lineComments: bang, strings: strs(rawDQ, rawSQ), foldCase: true,
		words: words(
			kw("program end subroutine function module use implicit none dimension allocatable intent in out inout parameter if then else elseif endif do while enddo select case call return stop contains interface type public private pointer target allocate deallocate print read write open close"),
			types("integer real complex logical character double precision")),
	},
}

var pascal = &editor.Language{
	Name: "Pascal", Extensions: []string{".pas", ".pp", ".dpr", ".lpr"},
	LineComment: "//", BlockComment: [2]string{"{", "}"}, Brackets: brackets[:2], AutoClose: autoClose("'"), TabSize: 2,
	IncreaseIndent: re(`(?i)\b(begin|record|case|repeat)\s*$`),
	DecreaseIndent: re(`(?i)^\s*(end|until)\b`),
	Tokenizer: &spec{
		lineComments: slash, blocks: []blockComment{{open: "{", close: "}"}, {open: "(*", close: "*)"}},
		strings: strs(rawSQ), foldCase: true,
		words: words(
			kw("absolute and array as asm begin case class const constructor destructor div do downto else end except exports file finalization finally for function goto if implementation in inherited initialization inline interface is label library mod nil not object of on operator or packed procedure program property raise record repeat set shl shr then threadvar to try type unit until uses var while with xor"),
			types("integer real boolean char byte word longint shortint cardinal int64 double single extended string pointer"),
			consts("true false nil")),
	},
}

var assembly = &editor.Language{
	Name: "Assembly", Extensions: []string{".s", ".asm", ".nasm"},
	LineComment: ";", BlockComment: [2]string{"/*", "*/"}, Brackets: brackets[:2], TabSize: 8,
	Tokenizer: &spec{
		lineComments: []string{";", "//"}, blocks: cBlock, strings: strs(dq, charLit), foldCase: true,
		words: words(
			kw("mov movl movq movb movzx movsx add sub mul imul div idiv inc dec cmp test jmp je jne jz jnz jg jge jl jle ja jae jb jbe call ret push pop lea and or xor not neg shl shr sal sar nop int syscall leave cmov ldr str ldp stp ldrb strb b bl bx blr cbz cbnz adr adrp svc mvn orr eor lsl lsr asr cmn tst beq bne blt bgt ble bge"),
			vars("rax rbx rcx rdx rsi rdi rsp rbp r8 r9 r10 r11 r12 r13 r14 r15 eax ebx ecx edx esi edi esp ebp ax bx cx dx al bl cl dl ah bh ch dh sp lr pc xzr wzr "+numbered("x", 0, 30, 1)+numbered("w", 0, 30, 1)+numbered("r", 0, 7, 1))),
		prefixes: map[byte]editor.TokenKind{'.': editor.TokenMeta, '%': editor.TokenVariable},
	},
}

var solidity = &editor.Language{
	Name: "Solidity", Extensions: []string{".sol"},
	LineComment: "//", BlockComment: [2]string{"/*", "*/"}, Brackets: brackets, AutoClose: autoClose("\"", "'"), TabSize: 4,
	Tokenizer: &spec{
		lineComments: slash, blocks: cBlock, strings: strs(dq, sq),
		words: words(
			kw("abstract anonymous as assembly break calldata catch constant constructor continue contract delete do else emit enum error event external fallback for function if immutable import indexed interface internal is library mapping memory modifier new override payable pragma private public pure receive return returns revert require assert storage struct throw try type unchecked using view virtual while"),
			types("address bool string bytes byte uint int "+numbered("uint", 8, 256, 8)+numbered("int", 8, 256, 8)+numbered("bytes", 1, 32, 1)),
			consts("true false wei gwei ether this super msg block tx")),
		capitalTypes: true, calls: true,
	},
}

var verilog = &editor.Language{
	Name: "Verilog", Extensions: []string{".v", ".vh", ".sv", ".svh"},
	LineComment: "//", BlockComment: [2]string{"/*", "*/"}, Brackets: brackets, AutoClose: autoClose("\""), TabSize: 2,
	IncreaseIndent: re(`\b(begin|module|case|casex|casez|function|task|generate|fork|class|interface|package)\b[^;]*$`),
	DecreaseIndent: re(`^\s*(end\w*|join\w*)\b`),
	Tokenizer: &spec{
		lineComments: slash, blocks: cBlock, strings: strs(dq),
		words: words(
			kw("always and assign automatic begin buf case casex casez default defparam disable edge else end endcase endfunction endgenerate endmodule endprimitive endtask event for force forever fork function generate genvar if ifnone initial inout input integer join localparam module nand negedge nor not or output parameter posedge primitive real realtime reg release repeat signed supply0 supply1 table task time tri unsigned wait wand while wire wor xnor xor logic bit byte int class endclass interface endinterface package endpackage typedef struct enum return")),
		prefixes:    map[byte]editor.TokenKind{'$': editor.TokenFunction, '`': editor.TokenMeta},
		numberChars: "'",
	},
}

var vhdl = &editor.Language{
	Name: "VHDL", Extensions: []string{".vhd", ".vhdl"},
	LineComment: "--", Brackets: brackets[:2], AutoClose: autoClose("\""), TabSize: 2,
	IncreaseIndent: re(`(?i)\b(is|begin|then|loop|generate)\s*$`),
	DecreaseIndent: re(`(?i)^\s*(end|else|elsif|begin)\b`),
	Tokenizer: &spec{
		lineComments: dash, strings: strs(rawDQ, charLit), foldCase: true,
		words: words(
			kw("abs access after alias all and architecture array assert attribute begin block body buffer bus case component configuration constant disconnect downto else elsif end entity exit file for function generate generic group guarded if impure in inertial inout is label library linkage literal loop map mod nand new next nor not null of on open or others out package port postponed procedure process pure range record register reject rem report return rol ror select severity shared signal sla sll sra srl subtype then to transport type unaffected units until use variable wait when while with xnor xor"),
			types("std_logic std_logic_vector std_ulogic integer natural positive boolean bit bit_vector string character real time unsigned signed")),
	},
}
