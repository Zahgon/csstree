package walker_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/csstree/csstree/ast"
	"github.com/csstree/csstree/convertor"
	"github.com/csstree/csstree/internal/fixtureset"
	"github.com/csstree/csstree/js"
	"github.com/csstree/csstree/syntax"
	"github.com/csstree/csstree/walker"
)

func fixtureASTs(t *testing.T) map[string]*js.Object {
	t.Helper()
	tests, err := fixtureset.Load("../fixtures/ast")
	if err != nil {
		t.Fatal(err)
	}
	asts := map[string]*js.Object{}
	for _, test := range tests {
		if test.Data.Has("error") {
			continue
		}
		tree, err := syntax.Parse(test.Data.Str("source"), syntax.ParseOptions{Context: test.Scope, Positions: true,
			Atrule: test.Data.Obj("options").Str("atrule")})
		if err == nil {
			asts[test.Name] = tree
		}
	}
	return asts
}

// visit records the type of every node a walk enters and leaves.
func visit(t *testing.T, root *js.Object, opts walker.Options) (enter, leave []*js.Object) {
	t.Helper()
	opts.Enter = func(_ *walker.Context, node *js.Object, _ *ast.ListItem, _ *ast.List) walker.Action {
		enter = append(enter, node)
		return walker.Continue
	}
	opts.Leave = func(_ *walker.Context, node *js.Object, _ *ast.ListItem, _ *ast.List) walker.Action {
		leave = append(leave, node)
		return walker.Continue
	}
	if err := walker.Walk(root, opts); err != nil {
		t.Fatal(err)
	}
	return enter, leave
}

func TestWalkOrder(t *testing.T) {
	for name, tree := range fixtureASTs(t) {
		enter, leave := visit(t, tree, walker.Options{})
		if len(enter) != len(leave) || enter[0] != tree || leave[len(leave)-1] != tree {
			t.Fatalf("%s: enter %d nodes, leave %d nodes, root first and last", name, len(enter), len(leave))
		}
		reverseEnter, _ := visit(t, tree, walker.Options{Reverse: true})
		slices.Reverse(leave)
		if !slices.Equal(reverseEnter, leave) {
			t.Fatalf("%s: a reverse walk must enter nodes in the reverse of the natural leave order", name)
		}
		for _, typ := range []string{"Declaration", "Rule", "Atrule", "Identifier"} {
			filtered, _ := visit(t, tree, walker.Options{Visit: typ})
			var want []*js.Object
			for _, n := range enter {
				if n.Str("type") == typ {
					want = append(want, n)
				}
			}
			// Atrule, Rule and Declaration use the fast traversal, which does not enter preludes and values.
			if typ == "Identifier" && !slices.Equal(filtered, want) || !isSubsequence(filtered, want) {
				t.Fatalf("%s: visit %s entered %d nodes, want %d", name, typ, len(filtered), len(want))
			}
		}
	}
}

func isSubsequence(sub, of []*js.Object) bool {
	i := 0
	for _, n := range of {
		if i < len(sub) && sub[i] == n {
			i++
		}
	}
	return i == len(sub)
}

func TestWalkContext(t *testing.T) {
	tree, err := syntax.Parse("@media x { a { color: rgb(1, 2, 3) } }", syntax.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var seen []string
	err = walker.Walk(tree, walker.Options{Enter: func(ctx *walker.Context, node *js.Object, _ *ast.ListItem, _ *ast.List) walker.Action {
		if node.Str("type") == "Number" {
			for name, v := range map[string]*js.Object{"root": ctx.Root, "stylesheet": ctx.Stylesheet, "atrule": ctx.Atrule,
				"rule": ctx.Rule, "block": ctx.Block, "declaration": ctx.Declaration, "function": ctx.Function} {
				if v != nil {
					seen = append(seen, name+":"+v.Str("type"))
				}
			}
		}
		return walker.Continue
	}})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(seen)
	want := "atrule:Atrule block:Block declaration:Declaration function:Function root:StyleSheet rule:Rule stylesheet:StyleSheet"
	if got := strings.Join(slices.Compact(seen), " "); got != want {
		t.Fatalf("context %q, want %q", got, want)
	}
}

func TestSkipAndBreak(t *testing.T) {
	tree, err := syntax.Parse("a { color: red } b { color: blue }", syntax.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var types []string
	err = walker.Walk(tree, walker.Options{Enter: func(_ *walker.Context, node *js.Object, _ *ast.ListItem, _ *ast.List) walker.Action {
		types = append(types, node.Str("type"))
		switch node.Str("type") {
		case "SelectorList":
			return walker.Skip
		case "Identifier":
			return walker.Break
		}
		return walker.Continue
	}})
	want := "StyleSheet Rule SelectorList Block Declaration Value Identifier"
	if got := strings.Join(types, " "); err != nil || got != want {
		t.Fatalf("got %q (%v), want %q", got, err, want)
	}
}

func TestWalkErrors(t *testing.T) {
	err := walker.Walk(js.NewObject("type", "Value"), walker.Options{})
	if err == nil || err.Error() != "Neither `enter` nor `leave` walker handler is set or both aren't a function" {
		t.Fatalf("got %v", err)
	}
	enter := func(*walker.Context, *js.Object, *ast.ListItem, *ast.List) walker.Action { return walker.Continue }
	err = walker.Walk(js.NewObject("type", "Value"), walker.Options{Enter: enter, Visit: "Foo"})
	if err == nil || !strings.HasPrefix(err.Error(), "Bad value `Foo` for `visit` option (should be: AnPlusB, Atrule,") {
		t.Fatalf("got %v", err)
	}
}

func TestFind(t *testing.T) {
	tree, err := syntax.Parse(".a { color: red } .b { color: blue }", syntax.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	isClass := func(_ *walker.Context, node *js.Object, _ *ast.ListItem, _ *ast.List) bool {
		return node.Str("type") == "ClassSelector"
	}
	if got := walker.Find(tree, isClass).Str("name"); got != "a" {
		t.Fatalf("find: %q, want a", got)
	}
	if got := walker.FindLast(tree, isClass).Str("name"); got != "b" {
		t.Fatalf("findLast: %q, want b", got)
	}
	if got := len(walker.FindAll(tree, isClass)); got != 2 {
		t.Fatalf("findAll: %d nodes, want 2", got)
	}
}

func TestConvertRoundTrip(t *testing.T) {
	for name, tree := range fixtureASTs(t) {
		before := js.Stringify(tree)
		plain := convertor.ToPlainObject(tree).(*js.Object)
		if _, isList := plain.Get("children").(*ast.List); isList {
			t.Fatalf("%s: toPlainObject left a list", name)
		}
		back := convertor.FromPlainObject(plain)
		if js.Stringify(back) != before {
			t.Fatalf("%s: round trip changed the tree", name)
		}
		clone, err := ast.Clone(tree)
		if err != nil || clone == tree || js.Stringify(clone) != before {
			t.Fatalf("%s: clone %v", name, err)
		}
	}
}
