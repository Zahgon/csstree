package lexer

import (
	"errors"
	"strings"

	"github.com/csstree/csstree/definitionsyntax"
	"github.com/csstree/csstree/generator"
	"github.com/csstree/csstree/js"
	"github.com/csstree/csstree/tokenizer"
)

var cssWideKeywords = []string{"initial", "inherit", "unset", "revert", "revert-layer"}

// defaultUnits are the unit groups in the order of the original's units module namespace.
var defaultUnits = []struct {
	group string
	units []string
}{
	{"angle", []string{"deg", "grad", "rad", "turn"}},
	{"decibel", []string{"db"}},
	{"flex", []string{"fr"}},
	{"frequency", []string{"hz", "khz"}},
	{"length", []string{"cm", "mm", "q", "in", "pt", "pc", "px", "em", "rem", "ex", "rex", "cap", "rcap", "ch",
		"rch", "ic", "ric", "lh", "rlh", "vw", "svw", "lvw", "dvw", "vh", "svh", "lvh", "dvh", "vi", "svi", "lvi",
		"dvi", "vb", "svb", "lvb", "dvb", "vmin", "svmin", "lvmin", "dvmin", "vmax", "svmax", "lvmax", "dvmax", "cqw",
		"cqh", "cqi", "cqb", "cqmin", "cqmax"}},
	{"resolution", []string{"dpi", "dpcm", "dppx", "x"}},
	{"semitones", []string{"st"}},
	{"time", []string{"s", "ms"}},
}

// Descriptor is a type, property, at-rule prelude or at-rule descriptor definition; its grammar is parsed and
// its match graph built on first use, like the original's lazy getters.
type Descriptor struct {
	typ, name   string
	parent      any
	source      any
	syntax      *js.Object
	parsed      bool
	graph       *gnode
	refGraph    *gnode
	refBuilt    bool
	ref         *js.Object
	genericFunc genericFn
}

func newDescriptor(syntax any, typ, name string, parent any) *Descriptor {
	d := &Descriptor{typ: typ, name: name, parent: parent, source: syntax,
		ref: js.NewObject("type", typ, "name", name)}
	switch s := syntax.(type) {
	case genericFn:
		d.genericFunc = s
		d.graph = buildMatchGraph(s, d.ref)
		d.parsed = true
	case *js.Object:
		d.syntax, d.parsed = s, true
	case string:
	default:
		d.parsed = true
	}
	return d
}

// Type returns the kind of definition: Type, Property, AtrulePrelude or AtruleDescriptor.
func (d *Descriptor) Type() string { return d.typ }

// Name returns the definition's name.
func (d *Descriptor) Name() string { return d.name }

// Parent returns the name of the at-rule an at-rule descriptor belongs to, and nil for other definitions.
func (d *Descriptor) Parent() any { return d.parent }

// Serializable reports whether the definition was given as grammar text or as a parsed grammar rather than as a
// generic matcher, like the original's serializable flag.
func (d *Descriptor) Serializable() bool {
	switch s := d.source.(type) {
	case string:
		return true
	case *js.Object:
		if s == nil {
			return false
		}
		_, ok := s.Get("type").(string)
		return ok
	}
	return false
}

// Syntax returns the parsed grammar, parsing grammar text on first use; it is nil for generic types. Grammar text
// that does not parse returns its syntax error.
func (d *Descriptor) Syntax() (syntax *js.Object, err error) {
	defer recoverError(&err)
	return d.grammar(), nil
}

// grammar returns the parsed grammar (nil for generic types); a grammar that does not parse raises its error
// on every use, as the original's getter does.
func (d *Descriptor) grammar() *js.Object {
	if !d.parsed {
		tree, err := definitionsyntax.Parse(d.source.(string))
		if err != nil {
			panic(&graphError{err})
		}
		d.syntax, d.parsed = tree, true
	}
	return d.syntax
}

func (d *Descriptor) matchGraph() *gnode {
	if d.graph == nil {
		grammar := d.grammar()
		if grammar == nil {
			panic(&matchError{errors.New("Cannot read properties of null (reading 'type')")})
		}
		d.graph = buildMatchGraph(grammar, d.ref)
	}
	return d.graph
}

func (d *Descriptor) matchRefGraph() *gnode {
	if d.typ != "Property" || d.genericFunc != nil {
		return nil
	}
	if !d.refBuilt {
		syntax := d.grammar()
		terms, _ := syntax.Get("terms").([]any)
		if syntax.Get("explicit") == false && len(terms) == 1 {
			if first, _ := terms[0].(*js.Object); first.Str("type") == "Multiplier" && first.Get("comma") == true {
				copied := js.NewObject()
				for _, k := range syntax.Keys() {
					v, _ := syntax.Lookup(k)
					copied.Set(k, v)
				}
				copied.Set("terms", []any{first.Get("term")})
				d.refGraph = buildMatchGraph(copied, d.ref)
			}
		}
		d.refBuilt = true
	}
	return d.refGraph
}

// dict is a dictionary of descriptors in JavaScript key order.
type dict struct{ o *js.Object }

func newDict() dict { return dict{js.NewObject()} }

func (m dict) set(name string, d *Descriptor) { m.o.Set(name, d) }

func (m dict) lookup(name string) *Descriptor {
	d, _ := m.o.Get(name).(*Descriptor)
	return d
}

func (m dict) keys() []string { return m.o.Keys() }

// Atrule is an at-rule definition: an optional prelude descriptor and optional descriptors.
type Atrule struct {
	name        string
	prelude     *Descriptor
	descriptors *dict
}

// Name returns the at-rule's name.
func (a *Atrule) Name() string { return a.name }

// Prelude returns the definition of the at-rule's prelude, or nil when it has none.
func (a *Atrule) Prelude() *Descriptor { return a.prelude }

// Descriptor returns the definition of one of the at-rule's descriptors, or nil when it has no such descriptor.
func (a *Atrule) Descriptor(name string) *Descriptor {
	if a.descriptors == nil {
		return nil
	}
	return a.descriptors.lookup(name)
}

// DescriptorNames returns the names of the at-rule's descriptors in definition order.
func (a *Atrule) DescriptorNames() []string {
	if a.descriptors == nil {
		return nil
	}
	return a.descriptors.keys()
}

// Lexer matches values against the definitions of a syntax.
type Lexer struct {
	cssWideKeywords any
	generic         bool
	units           *js.Object
	types           dict
	properties      dict
	atrules         *js.Object
	cssWideSyntax   *gnode
}

// New builds a lexer from a config with the original's keys: generic, cssWideKeywords, units, types, atrules
// and properties.
func New(config *js.Object) (lexer *Lexer, err error) {
	defer recoverError(&err)
	l := &Lexer{cssWideKeywords: toAnySlice(cssWideKeywords), units: js.NewObject(), types: newDict(),
		properties: newDict(), atrules: js.NewObject()}
	for _, u := range defaultUnits {
		l.units.Set(u.group, toAnySlice(u.units))
	}
	if config != nil {
		if v := config.Get("cssWideKeywords"); js.Truthy(v) {
			l.cssWideKeywords = v
		}
		if units := config.Obj("units"); units != nil {
			for _, u := range defaultUnits {
				if list, ok := units.Get(u.group).([]any); ok {
					l.units.Set(u.group, list)
				}
			}
		}
		if types := config.Obj("types"); types != nil {
			for _, name := range types.Keys() {
				l.addType(name, types.Get(name))
			}
		}
		if js.Truthy(config.Get("generic")) {
			l.generic = true
			for _, g := range createGenericTypes(l.units) {
				l.addType(g.name, g.fn)
			}
		}
		if atrules := config.Obj("atrules"); atrules != nil {
			for _, name := range atrules.Keys() {
				l.addAtrule(name, atrules.Get(name))
			}
		}
		if properties := config.Obj("properties"); properties != nil {
			for _, name := range properties.Keys() {
				l.addProperty(name, properties.Get(name))
			}
		}
	}
	l.cssWideSyntax = buildMatchGraph(joinKeywords(l.cssWideKeywords), nil)
	return l, nil
}

func toAnySlice(list []string) []any {
	out := make([]any, len(list))
	for i, s := range list {
		out[i] = s
	}
	return out
}

func joinKeywords(v any) string {
	list, _ := v.([]any)
	parts := make([]string, len(list))
	for i, k := range list {
		if k != nil && k != js.Undefined {
			parts[i] = js.ToString(k)
		}
	}
	return strings.Join(parts, " |  ")
}

func (l *Lexer) addType(name string, syntax any) {
	if js.Truthy(syntax) {
		l.types.set(name, newDescriptor(syntax, "Type", name, nil))
	}
}

func (l *Lexer) addProperty(name string, syntax any) {
	if js.Truthy(syntax) {
		l.properties.set(name, newDescriptor(syntax, "Property", name, nil))
	}
}

func (l *Lexer) addAtrule(name string, value any) {
	syntax, _ := value.(*js.Object)
	if !js.Truthy(value) || syntax == nil {
		return
	}
	entry := &Atrule{name: name}
	if p := syntax.Get("prelude"); js.Truthy(p) {
		entry.prelude = newDescriptor(p, "AtrulePrelude", name, nil)
	}
	if descs := syntax.Obj("descriptors"); descs != nil {
		m := newDict()
		for _, dn := range descs.Keys() {
			m.set(dn, newDescriptor(descs.Get(dn), "AtruleDescriptor", dn, name))
		}
		entry.descriptors = &m
	}
	l.atrules.Set(name, entry)
}

// recoverError turns a panic raised while building a graph or matching into the error the original throws.
func recoverError(err *error) {
	if r := recover(); r != nil {
		switch e := r.(type) {
		case *graphError:
			*err = e.err
		case *matchError:
			*err = e.err
		default:
			panic(r)
		}
	}
}

// prepareTokens turns a value (CSS text or an AST node) into the tokens the matcher reads.
func prepareTokens(value any) ([]*token, error) {
	var tokens []*token
	if s, ok := value.(string); ok {
		units := js.UTF16(s)
		tokenizer.TokenizeUnits(units, func(typ, start, end int) {
			tokens = append(tokens, &token{typ: typ, value: js.FromUTF16(units[start:end])})
		})
		return tokens, nil
	}
	generated, err := generator.Tokens(value)
	if err != nil {
		return nil, err
	}
	for _, g := range generated {
		tokens = append(tokens, &token{typ: g.Type, value: g.Value, node: g.Node})
	}
	return tokens, nil
}
