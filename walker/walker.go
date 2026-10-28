package walker

import (
	"errors"
	"sort"
	"strings"

	"github.com/csstree/csstree/ast"
	"github.com/csstree/csstree/internal/js"
)

// Action is what a visitor asks the walker to do next.
type Action int

// Continue walks on, Skip does not enter the children of the node, Break stops the walk.
const (
	Continue Action = iota
	Skip
	Break
)

// Context holds the walk root and the nearest enclosing node of each kind that sets a walk context.
type Context struct {
	Root, Stylesheet, Atrule, AtrulePrelude, Rule, Selector, Block, Declaration, Function *js.Object
}

func (c *Context) field(name string) **js.Object {
	switch name {
	case "stylesheet":
		return &c.Stylesheet
	case "atrule":
		return &c.Atrule
	case "atrulePrelude":
		return &c.AtrulePrelude
	case "rule":
		return &c.Rule
	case "selector":
		return &c.Selector
	case "block":
		return &c.Block
	case "declaration":
		return &c.Declaration
	case "function":
		return &c.Function
	}
	return nil
}

// Visitor is called for a node with the list item and list holding it (nil for a node held by a field).
type Visitor func(ctx *Context, node *js.Object, item *ast.ListItem, list *ast.List) Action

// Options configure a walk: Enter and Leave visitors, reverse order, and Visit to call the visitors for one
// node type only.
type Options struct {
	Enter, Leave Visitor
	Reverse      bool
	Visit        string
}

type walkField struct {
	name     string
	list     bool
	nullable bool
}

type iterator struct {
	context string
	fields  []walkField
}

var naturalIterators, reverseIterators = buildIterators()

func buildIterators() (map[string]*iterator, map[string]*iterator) {
	natural, reverse := map[string]*iterator{}, map[string]*iterator{}
	for _, nt := range NodeTypes {
		var fields []walkField
		for _, f := range nt.Fields {
			w := walkField{name: f.Name}
			kind := ""
			for _, ft := range f.Types {
				switch ft.Kind {
				case "null":
					w.nullable = true
				case "node", "list":
					kind = ft.Kind
				}
			}
			if kind != "" {
				w.list = kind == "list"
				fields = append(fields, w)
			}
		}
		if len(fields) == 0 {
			continue
		}
		natural[nt.Name] = &iterator{context: nt.WalkContext, fields: fields}
		reversed := make([]walkField, len(fields))
		for i, f := range fields {
			reversed[len(fields)-1-i] = f
		}
		reverse[nt.Name] = &iterator{context: nt.WalkContext, fields: reversed}
	}
	return natural, reverse
}

var fastTraversal = map[string][]string{
	"Atrule":      {"StyleSheet", "Atrule", "Rule", "Block"},
	"Rule":        {"StyleSheet", "Atrule", "Rule", "Block"},
	"Declaration": {"StyleSheet", "Atrule", "Rule", "Block", "DeclarationList"},
}

type walk struct {
	ctx          *Context
	enter, leave Visitor
	iterators    map[string]*iterator
	reverse      bool
}

func (w *walk) node(data any, item *ast.ListItem, list *ast.List) bool {
	node, _ := data.(*js.Object)
	switch w.enter(w.ctx, node, item, list) {
	case Break:
		return true
	case Skip:
		return false
	}
	if it, ok := w.iterators[node.Str("type")]; ok && node != nil && w.iterate(it, node) {
		return true
	}
	return w.leave(w.ctx, node, item, list) == Break
}

// iterate walks the node and list fields of a node; on a break the walk context is left as it is.
func (w *walk) iterate(it *iterator, node *js.Object) bool {
	var slot **js.Object
	var prev *js.Object
	if it.context != "" {
		slot = w.ctx.field(it.context)
		prev = *slot
		*slot = node
	}
	for _, f := range it.fields {
		ref := node.Get(f.name)
		if f.nullable && !js.Truthy(ref) {
			continue
		}
		if f.list {
			if w.list(ref) {
				return true
			}
		} else if w.node(ref, nil, nil) {
			return true
		}
	}
	if slot != nil {
		*slot = prev
	}
	return false
}

func (w *walk) list(ref any) bool {
	stop := false
	step := func(data any, item *ast.ListItem, list *ast.List) {
		if !stop {
			stop = w.node(data, item, list)
		}
	}
	switch l := ref.(type) {
	case *ast.List:
		if w.reverse {
			l.ForEachRight(step)
		} else {
			l.ForEach(step)
		}
	case []any:
		for i := range l {
			j := i
			if w.reverse {
				j = len(l) - 1 - i
			}
			step(l[j], nil, nil)
		}
	}
	return stop
}

func continueVisitor(*Context, *js.Object, *ast.ListItem, *ast.List) Action { return Continue }

func forType(v Visitor, typ string) Visitor {
	return func(ctx *Context, node *js.Object, item *ast.ListItem, list *ast.List) Action {
		if node.Str("type") == typ {
			return v(ctx, node, item, list)
		}
		return Continue
	}
}

// Walk visits root and its descendants in document order (or reverse order), calling Enter before a node's
// children and Leave after them.
func Walk(root any, opts Options) error {
	w := &walk{enter: opts.Enter, leave: opts.Leave, iterators: naturalIterators, reverse: opts.Reverse}
	if w.enter == nil {
		w.enter = continueVisitor
	}
	if w.leave == nil {
		w.leave = continueVisitor
	}
	if opts.Reverse {
		w.iterators = reverseIterators
	}
	if opts.Visit != "" {
		if names, ok := fastTraversal[opts.Visit]; ok {
			fast := map[string]*iterator{}
			for _, name := range names {
				fast[name] = w.iterators[name]
			}
			w.iterators = fast
		} else if LookupNodeType(opts.Visit) == nil {
			var types []string
			for _, nt := range NodeTypes {
				types = append(types, nt.Name)
			}
			sort.Strings(types)
			return errors.New("Bad value `" + opts.Visit + "` for `visit` option (should be: " + strings.Join(types, ", ") + ")")
		}
		w.enter = forType(w.enter, opts.Visit)
		w.leave = forType(w.leave, opts.Visit)
	}
	if opts.Enter == nil && opts.Leave == nil {
		return errors.New("Neither `enter` nor `leave` walker handler is set or both aren't a function")
	}
	w.run(root)
	return nil
}

func (w *walk) run(root any) {
	rootNode, _ := root.(*js.Object)
	w.ctx = &Context{Root: rootNode}
	w.node(root, nil, nil)
}

// Visit walks root with the given visitors (either may be nil) in document order; unlike Walk it takes no
// options that can be invalid, so it cannot fail.
func Visit(root any, enter, leave Visitor) {
	w := &walk{enter: enter, leave: leave, iterators: naturalIterators}
	if w.enter == nil {
		w.enter = continueVisitor
	}
	if w.leave == nil {
		w.leave = continueVisitor
	}
	w.run(root)
}

// walkEnter walks root with an enter visitor only; it cannot fail.
func walkEnter(root any, reverse bool, enter Visitor) {
	w := &walk{enter: enter, leave: continueVisitor, iterators: naturalIterators, reverse: reverse}
	if reverse {
		w.iterators = reverseIterators
	}
	w.run(root)
}

// Find returns the first node in document order for which fn returns true, or nil.
func Find(root any, fn func(ctx *Context, node *js.Object, item *ast.ListItem, list *ast.List) bool) *js.Object {
	var found *js.Object
	walkEnter(root, false, func(ctx *Context, node *js.Object, item *ast.ListItem, list *ast.List) Action {
		if fn(ctx, node, item, list) {
			found = node
			return Break
		}
		return Continue
	})
	return found
}

// FindLast returns the last node in document order for which fn returns true, or nil.
func FindLast(root any, fn func(ctx *Context, node *js.Object, item *ast.ListItem, list *ast.List) bool) *js.Object {
	var found *js.Object
	walkEnter(root, true, func(ctx *Context, node *js.Object, item *ast.ListItem, list *ast.List) Action {
		if fn(ctx, node, item, list) {
			found = node
			return Break
		}
		return Continue
	})
	return found
}

// FindAll returns every node for which fn returns true, in document order.
func FindAll(root any, fn func(ctx *Context, node *js.Object, item *ast.ListItem, list *ast.List) bool) []*js.Object {
	found := []*js.Object{}
	walkEnter(root, false, func(ctx *Context, node *js.Object, item *ast.ListItem, list *ast.List) Action {
		if fn(ctx, node, item, list) {
			found = append(found, node)
		}
		return Continue
	})
	return found
}
