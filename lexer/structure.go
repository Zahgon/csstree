package lexer

import (
	"math"

	"github.com/csstree/csstree/ast"
	"github.com/csstree/csstree/definitionsyntax"
	"github.com/csstree/csstree/js"
	"github.com/csstree/csstree/walker"
)

// Warning is a structure problem found by CheckStructure, with the node it is about.
type Warning struct {
	Node    any
	Message string
}

func isValidNumber(v any) bool {
	f, ok := v.(float64)
	if i, isInt := v.(int); isInt {
		f, ok = float64(i), true
	}
	return ok && !math.IsInf(f, 0) && !math.IsNaN(f) && math.Floor(f) == f && f >= 0
}

func isValidLocation(loc *js.Object) bool {
	return loc != nil && isValidNumber(loc.Get("offset")) && isValidNumber(loc.Get("line")) && isValidNumber(loc.Get("column"))
}

func checkField(node *js.Object, key string, types []walker.FieldType) bool {
	value := node.Get(key)
	for _, ft := range types {
		switch ft.Kind {
		case "string":
			if _, ok := value.(string); ok {
				return true
			}
		case "boolean":
			if _, ok := value.(bool); ok {
				return true
			}
		case "null":
			if v, ok := node.Lookup(key); ok && v == nil {
				return true
			}
		case "node":
			if o, ok := value.(*js.Object); ok && o.Str("type") == ft.Node && o.Has("type") {
				return true
			}
		case "list":
			if _, ok := value.(*ast.List); ok {
				return true
			}
		}
	}
	return false
}

func checkNode(nt *walker.NodeType, node *js.Object, warn func(any, string)) {
	fields := map[string][]walker.FieldType{}
	for _, f := range nt.Fields {
		fields[f.Name] = f.Types
	}
	for _, key := range node.Keys() {
		valid := true
		switch {
		case key == "type":
			if node.Str("type") != nt.Name {
				warn(node, "Wrong node type `"+js.ToString(node.Get("type"))+"`, expected `"+nt.Name+"`")
			}
		case key == "loc":
			loc := node.Get("loc")
			if loc == nil {
				continue
			}
			valid = false
			if l, ok := loc.(*js.Object); ok {
				if _, isString := l.Get("source").(string); !isString {
					key += ".source"
				} else if !isValidLocation(l.Obj("start")) {
					key += ".start"
				} else if !isValidLocation(l.Obj("end")) {
					key += ".end"
				} else {
					continue
				}
			}
		default:
			if types, ok := fields[key]; ok {
				valid = checkField(node, key, types)
			} else {
				warn(node, "Unknown field `"+key+"` for "+nt.Name+" node type")
			}
		}
		if !valid {
			warn(node, "Bad value for `"+nt.Name+"."+key+"`")
		}
	}
	for _, key := range append([]string{"type", "loc"}, fieldNames(nt)...) {
		if !node.Has(key) {
			warn(node, "Field `"+nt.Name+"."+key+"` is missed")
		}
	}
}

func fieldNames(nt *walker.NodeType) []string {
	var names []string
	for _, f := range nt.Fields {
		names = append(names, f.Name)
	}
	return names
}

// CheckStructure walks an AST and returns the structure problems of its nodes, or nil when there are none.
func (l *Lexer) CheckStructure(tree any) []Warning {
	var warnings []Warning
	warn := func(node any, message string) { warnings = append(warnings, Warning{node, message}) }
	walker.Visit(tree, func(_ *walker.Context, node *js.Object, _ *ast.ListItem, _ *ast.List) walker.Action {
		name, _ := node.Get("type").(string)
		if nt := walker.LookupNodeType(name); nt != nil && node.Has("type") {
			checkNode(nt, node, warn)
		} else {
			typ, ok := node.Lookup("type")
			if !ok {
				typ = js.Undefined
			}
			warn(node, "Unknown node type `"+js.ToString(typ)+"`")
		}
		return walker.Continue
	}, nil)
	return warnings
}

func syntaxRef(name string, isType bool) string {
	if isType {
		return "<" + name + ">"
	}
	return "<'" + name + "'>"
}

// Validate checks that every type and property refers only to defined, valid syntaxes; it returns
// {errors, types, properties} for the broken ones, or nil when all are valid.
func (l *Lexer) Validate() (result *js.Object, err error) {
	defer recoverError(&err)
	var errorsList []any
	brokenTypes, brokenProperties := &brokenMap{}, &brokenMap{}
	var validate func(name string, broken *brokenMap, d *Descriptor) bool
	validate = func(name string, broken *brokenMap, d *Descriptor) bool {
		if v, ok := broken.get(name); ok {
			return v
		}
		broken.set(name, false)
		if grammar := d.grammar(); grammar != nil {
			walkErr := definitionsyntax.Walk(grammar, func(node *js.Object) {
				typ := node.Str("type")
				if typ != "Type" && typ != "Property" {
					return
				}
				dict, brokenMapRef := l.types, brokenTypes
				if typ == "Property" {
					dict, brokenMapRef = l.properties, brokenProperties
				}
				ref := node.Str("name")
				if !dict.o.Has(ref) {
					errorsList = append(errorsList, syntaxRef(name, broken == brokenTypes)+" used missed syntax definition "+syntaxRef(ref, typ == "Type"))
					broken.set(name, true)
				} else if validate(ref, brokenMapRef, dict.lookup(ref)) {
					errorsList = append(errorsList, syntaxRef(name, broken == brokenTypes)+" used broken syntax definition "+syntaxRef(ref, typ == "Type"))
					broken.set(name, true)
				}
			}, nil)
			if walkErr != nil {
				panic(&matchError{walkErr})
			}
		}
		return false
	}
	for _, key := range l.types.keys() {
		validate(key, brokenTypes, l.types.lookup(key))
	}
	for _, key := range l.properties.keys() {
		validate(key, brokenProperties, l.properties.lookup(key))
	}
	types, properties := brokenTypes.brokenNames(), brokenProperties.brokenNames()
	if len(types) == 0 && len(properties) == 0 {
		return nil, nil
	}
	if errorsList == nil {
		errorsList = []any{}
	}
	return js.NewObject("errors", errorsList, "types", types, "properties", properties), nil
}

// brokenMap is the original's Map from a name to whether its syntax is broken, in insertion order.
type brokenMap struct {
	names  []string
	values map[string]bool
}

func (b *brokenMap) get(name string) (bool, bool) {
	v, ok := b.values[name]
	return v, ok
}

func (b *brokenMap) set(name string, v bool) {
	if b.values == nil {
		b.values = map[string]bool{}
	}
	if _, ok := b.values[name]; !ok {
		b.names = append(b.names, name)
	}
	b.values[name] = v
}

func (b *brokenMap) brokenNames() []any {
	out := []any{}
	for _, n := range b.names {
		if b.values[n] {
			out = append(out, n)
		}
	}
	return out
}
