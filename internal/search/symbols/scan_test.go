package symbols

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func render(symbols []Symbol) []string {
	out := make([]string, 0, len(symbols))
	for _, s := range symbols {
		name := s.Name
		if s.Container != "" {
			name = s.Container + "." + name
		}
		out = append(out, fmt.Sprintf("%d %s %s", s.Line, s.Kind, name))
	}
	return out
}

func TestScanLanguages(t *testing.T) {
	tests := []struct {
		language string
		text     string
		want     []string
	}{
		{langGo, "package x\n\n/*\nfunc Hidden() {}\n*/\nfunc Main() {}\nfunc (s *Server) Start() error { return nil }\nfunc (Server) Stop() {}\ntype Config struct {\n\tName string\n}\ntype Runner interface {\n\tRun()\n}\ntype ID int\nconst Max = 1\nvar ErrX = errors.New(\"x\")\nconst (\n\tA = iota\n\tB\n)\nvar (\n\tc, d = 1, 2\n)\nfunc Map[T any](x T) T { return x }\n", []string{
			"5 function Main", "6 method Server.Start", "7 method Server.Stop", "8 struct Config", "11 interface Runner", "14 type ID", "15 constant Max", "16 variable ErrX", "18 constant A", "19 constant B", "22 variable c", "24 function Map",
		}},
		{langRust, "/* fn hidden() {} */\npub struct Foo { x: i32 }\nimpl<T> Foo<T> {\n    pub fn new() -> Self { Foo { x: 0 } }\n    fn helper<'a>(&self, s: &'a str) -> &'a str {\n        fn inner() {}\n        s\n    }\n}\nimpl fmt::Display for Foo {\n    fn fmt(&self) {}\n}\npub trait Shape { fn area(&self) -> f64; }\nenum Color { Red }\nconst MAX: u32 = 1;\nstatic mut COUNT: u32 = 0;\ntype Alias = u32;\npub mod util {\n    pub fn free() {}\n}\nmacro_rules! square { () => {} }\npub async fn run() {}\n", []string{
			"1 struct Foo", "3 method Foo.new", "4 method Foo.helper", "10 method Foo.fmt", "12 interface Shape", "13 enum Color", "14 constant MAX", "15 variable COUNT", "16 type Alias", "17 module util", "18 function util.free", "20 function square", "21 function run",
		}},
		{langPython, "import os\n\nMAX_SIZE = 10\n\"\"\"\nclass Hidden:\n\"\"\"\nclass Animal(Base):\n    def speak(self):\n        def inner():\n            pass\n    async def run(self):\n        pass\n\nclass Other:\n    pass\n\ndef main(\n    a,\n):\n    pass\n", []string{
			"2 constant MAX_SIZE", "6 class Animal", "7 method Animal.speak", "8 function inner", "10 method Animal.run", "13 class Other", "16 function main",
		}},
		{langJavaScript, "/**\n * function hidden() {}\n */\nexport class Store extends Base {\n  count = 0;\n  constructor(a) { super(a) }\n  async load(id) {\n    const local = () => 1;\n    return id;\n  }\n  static create = () => new Store();\n}\nexport function helper(a) {}\nconst add = (a, b) => a + b;\nconst LIMIT = 5;\nlet state = null;\nconst msg = \"it's { here\";\nfunction after() {}\n", []string{
			"3 class Store", "4 field Store.count", "5 method Store.constructor", "6 method Store.load", "10 method Store.create", "12 function helper", "13 function add", "14 constant LIMIT", "15 variable state", "16 constant msg", "17 function after",
		}},
		{langTypeScript, "export interface Props {\n  name: string;\n  onClick(e: Event): void;\n}\nexport type Id = string | number;\nenum Mode { A, B }\nexport abstract class Base<T> {\n  private cache: Map<string, T> = new Map();\n  public abstract run(): void;\n}\ndeclare namespace NS {\n  function f(): void;\n}\nexport const useThing = async (x: number): Promise<void> => {};\n", []string{
			"0 interface Props", "1 field Props.name", "2 method Props.onClick", "4 type Id", "5 enum Mode", "6 class Base", "7 field Base.cache", "8 method Base.run", "10 module NS", "11 function NS.f", "13 function useThing",
		}},
		{langJava, "package a.b;\n\n/* class Hidden {} */\n@Entity\npublic class User extends Base {\n    private final String name;\n    public User(String name) { this.name = name; }\n    @Override\n    public static <T> List<T> all(Map<String, T> m) {\n        return null;\n    }\n    enum Role { ADMIN(1), USER(2); }\n}\ninterface Repo {\n    User find(int id);\n}\npublic record Point(int x, int y) {}\n", []string{
			"4 class User", "5 field User.name", "6 method User.User", "8 method User.all", "11 enum User.Role", "13 interface Repo", "14 method Repo.find", "16 class Point",
		}},
		{langKotlin, "package x\n\n/* fun hidden() {} */\nopen class Base(val a: Int) {\n    fun greet(name: String): String = name\n    private val secret = 1\n}\ndata class Item(val id: Int)\nenum class Color { RED }\nfun interface Handler { fun handle() }\nobject Registry {\n    suspend fun load() {}\n}\nfun String.shout(): String = this\nconst val MAX = 1\ninterface Shape\ntypealias Names = List<String>\n", []string{
			"3 class Base", "4 method Base.greet", "5 field Base.secret", "7 class Item", "8 enum Color", "9 interface Handler", "10 class Registry", "11 method Registry.load", "13 function shout", "14 constant MAX", "15 interface Shape", "16 type Names",
		}},
		{langC, "#include <stdio.h>\n#define MAX 10\n#define SQUARE(x) ((x)*(x))\n/*\nint hidden(void) {}\n*/\nstruct point {\n    int x;\n};\ntypedef enum color { RED } color_t;\nstatic int add(int a, int b) {\n    return a + b;\n}\nchar *dup(const char *s);\nint main(void)\n{\n    struct point p;\n    return add(1, 2);\n}\n", []string{
			"1 constant MAX", "2 function SQUARE", "6 struct point", "9 enum color", "10 function add", "13 function dup", "14 function main",
		}},
		{langCPP, "namespace app {\nclass Widget : public Base {\npublic:\n    Widget();\n    virtual void draw() const;\n    static int count();\n};\nvoid Widget::draw() const {\n    helper();\n}\n}\nenum class Mode { A };\nusing Ids = std::vector<int>;\nextern \"C\" {\nint c_api(int x);\n}\n@interface Foo : NSObject\n- (void)bar:(int)x;\n", []string{
			"0 module app", "1 class app.Widget", "4 method Widget.draw", "5 method Widget.count", "7 method Widget.draw", "11 enum Mode", "12 type Ids", "14 function c_api", "16 class Foo", "17 method bar",
		}},
		{langCSharp, "using System;\nnamespace Demo.App\n{\n    /* class Hidden {} */\n    public sealed class Service : IService\n    {\n        private readonly int _count;\n        public string Name { get; set; }\n        public Service(int c) { }\n        public async Task<List<int>> RunAsync(int a)\n        {\n            var x = 1;\n            return null;\n        }\n    }\n    public interface IService { void Run(); }\n    public struct P { }\n    public enum E { A }\n    public record R(int X);\n}\n", []string{
			"1 module Demo.App", "4 class Demo.App.Service", "6 field Service._count", "7 field Service.Name", "8 method Service.Service", "9 method Service.RunAsync", "15 interface Demo.App.IService", "16 struct Demo.App.P", "17 enum Demo.App.E", "18 class Demo.App.R",
		}},
		{langSwift, "import Foundation\n/* func hidden() {} */\npublic struct Point {\n    var x: Int\n    let y = 2\n    init(x: Int) { self.x = x }\n    mutating func move() {}\n}\nfinal class Model {\n    static func make() -> Model { Model() }\n}\nextension Model {\n    func extra() {}\n}\nprotocol Drawable { func draw() }\nenum Dir { case up }\ntypealias Callback = () -> Void\nfunc free() {}\n", []string{
			"2 struct Point", "3 field Point.x", "4 constant Point.y", "5 method Point.init", "6 method Point.move", "8 class Model", "9 method Model.make", "12 method Model.extra", "14 interface Drawable", "15 enum Dir", "16 type Callback", "17 function free",
		}},
		{langRuby, "=begin\nclass Hidden\n=end\nmodule Billing\n  VERSION = 1\n  class Invoice < Base\n    def total\n      1\n    end\n    def self.build(x)\n    end\n    def paid?\n    end\n  end\nend\ndef top\nend\n", []string{
			"3 module Billing", "4 constant Billing.VERSION", "5 class Billing.Invoice", "6 method Invoice.total", "9 method Invoice.build", "11 method Invoice.paid?", "15 function top",
		}},
		{langPHP, "<?php\nnamespace App;\n/* class Hidden {} */\nabstract class Repo extends Base {\n    const TABLE = 'x';\n    private ?string $name;\n    public static function find(int $id): ?self {\n        return null;\n    }\n}\ninterface Shape {}\ntrait Loud {}\nenum Suit {}\nfunction helper() {}\n", []string{
			"1 module App", "3 class Repo", "4 constant Repo.TABLE", "5 field Repo.name", "6 method Repo.find", "10 interface Shape", "11 class Loud", "12 enum Suit", "13 function helper",
		}},
		{langLua, "--[[\nfunction hidden() end\n]]\nlocal function local_fn(a)\nend\nfunction M.exported(a)\nend\nfunction M:method()\nend\nlocal cb = function()\nend\nM.handler = function() end\n", []string{
			"3 function local_fn", "5 method M.exported", "7 method M.method", "9 function cb", "11 method M.handler",
		}},
		{langShell, "#!/bin/bash\nfunction build {\n  echo hi\n}\ndeploy() {\n  echo ok\n}\nclean-up () { :; }\necho $(date)\n", []string{
			"1 function build", "4 function deploy", "7 function clean-up",
		}},
		{langMarkdown, "# Title\n\n```sh\n# not a heading\n```\n## Section Two ##\ntext #notheading\n    # indented code\n###### Deep\n#nospace\n", []string{"0 heading Title", "5 heading Section Two", "8 heading Deep"}},
		{langYAML, "name: app\nversion: 1\nservices:\n  web:\n    image: x\n# comment: no\nlist:\n  - a: 1\n", []string{"0 key name", "1 key version", "2 key services", "6 key list"}},
		{langTOML, "title = \"x\"\n[package]\nname = \"a\"\n[[bin]]\n[dependencies.serde]\n# [commented]\n", []string{"1 table package", "3 table bin", "4 table dependencies.serde"}},
		{langJSON, "{\n  \"name\": \"x\",\n  \"scripts\": {\n    \"build\": \"y\"\n  },\n  \"files\": [\n    {\"a\": 1}\n  ],\n  \"last\": true\n}\n", []string{"1 key name", "2 key scripts", "5 key files", "8 key last"}},
		{langCSS, "/* .hidden { } */\n:root {\n  --a: 1;\n}\n.btn:hover, .card > p {\n  color: red;\n}\n@media (max-width: 600px) {\n  .nav {\n    display: none;\n  }\n  .nav a { x: y; }\n}\n@keyframes spin {\n  from { x: 0 }\n}\n.a {\n  &:hover {\n    x: y;\n  }\n}\n", []string{
			"1 selector :root", "4 selector .btn:hover, .card > p", "7 module @media (max-width: 600px)", "8 selector @media (max-width: 600px)..nav", "13 selector spin", "16 selector .a",
		}},
		{langSQL, "-- create table hidden (id int);\n/* create table hidden2 (id int); */\nCREATE TABLE IF NOT EXISTS public.users (\n  id int\n);\ncreate or replace function get_user(id int) returns int as $$ select 1 $$;\nCREATE VIEW v_active AS SELECT 1;\nCREATE PROCEDURE do_it() BEGIN END;\nCREATE TYPE mood AS ENUM ('a');\n", []string{
			"2 table users", "5 function get_user", "6 table v_active", "7 function do_it", "8 type mood",
		}},
		{langZig, "const std = @import(\"std\");\npub const Point = struct {\n    x: i32,\n    pub fn len(self: Point) i32 { return self.x; }\n};\nconst Color = enum { red };\npub fn main() !void {\n    const local = 1;\n}\nconst limit: u32 = 10;\nvar counter: u32 = 0;\nexport fn api() void {}\n", []string{
			"1 struct Point", "3 method Point.len", "5 enum Color", "6 function main", "9 constant limit", "10 variable counter", "11 function api",
		}},
		{langDart, "/* class Hidden {} */\nabstract class Shape {\n  final double size;\n  Shape(this.size);\n  factory Shape.unit() => Square(1);\n  double area();\n  Future<void> load() async {\n    print('x { y');\n  }\n}\nmixin Walker {}\nenum Level { low }\ntypedef Cb = void Function();\nvoid main() {\n  runApp();\n}\n", []string{
			"1 class Shape", "3 method Shape.Shape", "4 method Shape.unit", "5 method Shape.area", "6 method Shape.load", "10 class Walker", "11 enum Level", "12 type Cb", "13 function main",
		}},
		{langElixir, "defmodule MyApp.Worker do\n  @moduledoc \"\"\"\n  def hidden(x)\n  \"\"\"\n  def start(arg) do\n    :ok\n  end\n\n  defp helper(x), do: x\n  def valid?(x), do: true\nend\n", []string{
			"0 module MyApp.Worker", "4 function MyApp.Worker.start", "8 function MyApp.Worker.helper", "9 function MyApp.Worker.valid?",
		}},
		{langHaskell, "module Data.Util where\n{-\nfoo :: Int\n-}\ndata Tree a = Leaf | Node\nnewtype Wrap = Wrap Int\ntype Name = String\nclass (Eq a) => Container a where\n  empty :: a\nmapTree :: (a -> b) -> Tree a -> Tree b\nmapTree f t = t\n", []string{
			"0 module Data.Util", "4 type Tree", "5 type Wrap", "6 type Name", "7 interface Container", "9 function mapTree",
		}},
		{langScala, "package x\n/* class Hidden */\ntrait Shape {\n  def area: Double\n}\ncase class Circle(r: Double) extends Shape {\n  def area = 3.14 * r\n  val pi = 3\n  var count = 0\n}\nobject Main {\n  def main(args: Array[String]): Unit = {}\n}\nsealed abstract class Base\ntype Id = Int\n", []string{
			"2 interface Shape", "3 method Shape.area", "5 class Circle", "6 method Circle.area", "7 constant Circle.pi", "8 field Circle.count", "10 class Main", "11 method Main.main", "13 class Base", "14 type Id",
		}},
		{langMakefile, ".PHONY: build test\nCC := gcc\nVERSION = 1\nbuild: main.o\n\t$(CC) -o app main.o\nmain.o util.o: %.o: %.c\n\tcc -c $<\n# comment: x\nclean:\n\trm -f app\nfoo:=bar\n", []string{"3 target build", "5 target main.o", "8 target clean"}},
		{langDockerfile, "# FROM x AS hidden\nFROM golang:1.22 AS build\nRUN go build\nfrom --platform=linux/amd64 alpine AS final\nFROM scratch\n", []string{"1 target build", "3 target final"}},
		{langProtobuf, "syntax = \"proto3\";\nmessage Req {\n  message Inner {}\n  string q = 1;\n}\nenum Kind { A = 0; }\nservice Api {\n  rpc Get(Req) returns (Res);\n}\n", []string{"1 struct Req", "2 struct Req.Inner", "5 enum Kind", "6 interface Api", "7 method Api.Get"}},
		{langGraphQL, "type Query {\n  a: Int\n}\nenum Color { RED }\nscalar Date\nextend type User { b: Int }\n", []string{"0 struct Query", "3 enum Color", "4 type Date", "5 struct User"}},
		{langR, "add <- function(a, b) a + b\nx <- 1\nmul = function(a) a\n", []string{"0 function add", "2 function mul"}},
	}
	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			got := render(Scan(tt.language, tt.text))
			if !slices.Equal(got, tt.want) {
				t.Errorf("got  %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestScanEveryLanguageHasNamedRules(t *testing.T) {
	for language, l := range languages {
		for _, r := range l.rules {
			if r.name < 0 && !r.hidden {
				t.Errorf("%s: rule %q has no name group", language, r.re)
			}
		}
		if !Supports(language) {
			t.Errorf("%s: Supports is false", language)
		}
	}
}

func TestScanUnknownLanguage(t *testing.T) {
	if got := Scan("Brainfuck", "func main() {}"); got != nil {
		t.Errorf("got %v, want nil", got)
	}
	if Supports("Brainfuck") {
		t.Error("Supports returned true for unknown language")
	}
}

func TestScanPosition(t *testing.T) {
	got := Scan(langGo, "\n\nfunc (s *T) Do() {}\r\nfunc  Tail()")
	want := []Symbol{
		{Name: "Do", Container: "T", Kind: Method, Line: 2, Col: 12},
		{Name: "Tail", Kind: Function, Line: 3, Col: 6},
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestScanSkipsLongLines(t *testing.T) {
	long := "func " + strings.Repeat("a", maxLineBytes) + "() {}\nfunc Short() {}\n"
	got := render(Scan(langGo, long))
	if !slices.Equal(got, []string{"1 function Short"}) {
		t.Errorf("got %q", got)
	}
}

func TestScanBlockCommentInString(t *testing.T) {
	got := render(Scan(langGo, "var glob = \"**/*.go\"\nfunc After() {}\n"))
	if !slices.Equal(got, []string{"0 variable glob", "1 function After"}) {
		t.Errorf("got %q", got)
	}
}

func TestKindString(t *testing.T) {
	if Function.String() != "function" || Target.String() != "target" || Kind(200).String() != unknownKindLabel {
		t.Error("unexpected kind labels")
	}
	for k := Function; k <= Target; k++ {
		if k.String() == unknownKindLabel {
			t.Errorf("kind %d has no label", k)
		}
	}
}
