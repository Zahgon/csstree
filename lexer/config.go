package lexer

import (
	"github.com/csstree/csstree/data"
	"github.com/csstree/csstree/definitionsyntax"
	"github.com/csstree/csstree/internal/js"
)

func (l *Lexer) dumpSyntax(d *descriptor, compact, syntaxAsAST bool) any {
	grammar := d.grammar()
	if grammar == nil {
		return nil
	}
	if syntaxAsAST {
		return grammar
	}
	text, err := definitionsyntax.Generate(grammar, definitionsyntax.GenerateOptions{Compact: compact})
	if err != nil {
		panic(&matchError{err})
	}
	return text
}

func (l *Lexer) dumpMap(m dict, compact, syntaxAsAST bool) *js.Object {
	result := js.NewObject()
	for _, name := range m.keys() {
		if v := l.dumpSyntax(m.lookup(name), compact, syntaxAsAST); v != nil {
			result.Set(name, v)
		}
	}
	return result
}

// Dump returns the lexer's definitions: generic, cssWideKeywords, units, and the types, properties and at-rules
// written as grammar text (compact unless pretty) or as grammar ASTs.
func (l *Lexer) Dump(syntaxAsAST, pretty bool) (result *js.Object, err error) {
	defer recoverError(&err)
	compact := !pretty
	atrules := js.NewObject()
	for _, name := range l.atrules.Keys() {
		entry := l.atrules.Get(name).(*atrule)
		var prelude, descriptors any
		if entry.prelude != nil {
			prelude = l.dumpSyntax(entry.prelude, compact, syntaxAsAST)
		}
		if entry.descriptors != nil {
			descriptors = l.dumpMap(*entry.descriptors, compact, syntaxAsAST)
		}
		atrules.Set(name, js.NewObject("prelude", prelude, "descriptors", descriptors))
	}
	return js.NewObject("generic", l.generic, "cssWideKeywords", l.cssWideKeywords, "units", l.units,
		"types", l.dumpMap(l.types, compact, syntaxAsAST), "properties", l.dumpMap(l.properties, compact, syntaxAsAST),
		"atrules", atrules), nil
}

// DefaultConfig returns the lexer config of the default syntax: generic types on, the CSS-wide keywords, and the
// data package's types, at-rules and properties, normalised by Mix as the original does.
func DefaultConfig() (*js.Object, error) {
	d, err := data.Load()
	if err != nil {
		return nil, err
	}
	raw := js.NewObject("generic", true, "cssWideKeywords", toAnySlice(cssWideKeywords),
		"types", d.Get("types"), "atrules", d.Get("atrules"), "properties", d.Get("properties"))
	return Mix(js.NewObject(), raw), nil
}

// extendLength returns the length of a leading "|" and the white space around it, or -1 when the syntax does
// not extend the previous one.
func extendLength(s string) int {
	runes := []rune(s)
	i := 0
	for i < len(runes) && js.IsWhiteSpace(runes[i]) {
		i++
	}
	if i >= len(runes) || runes[i] != '|' {
		return -1
	}
	i++
	for i < len(runes) && js.IsWhiteSpace(runes[i]) {
		i++
	}
	return len(string(runes[:i]))
}

// appendOrSet extends syntax a with b when b starts with "|", else replaces it (falsy b becomes null).
func appendOrSet(a, b any) any {
	if s, ok := b.(string); ok {
		leading := 0
		for _, r := range s {
			if !js.IsWhiteSpace(r) {
				break
			}
			leading++
		}
		if trimmed := []rune(s)[leading:]; len(trimmed) > 0 && trimmed[0] == '|' {
			if prev, isString := a.(string); isString {
				return prev + s
			}
			return s[extendLength(s):]
		}
	}
	if js.Truthy(b) {
		return b
	}
	return nil
}

func copyObject(o *js.Object) *js.Object {
	result := js.NewObject()
	for _, k := range o.Keys() {
		v, _ := o.Lookup(k)
		result.Set(k, v)
	}
	return result
}

func entries(v any) *js.Object {
	o, _ := v.(*js.Object)
	if o == nil {
		return js.NewObject()
	}
	return o
}

// Mix merges a syntax extension into a config the way the original's fork does: generic is coerced to a
// boolean, cssWideKeywords are appended, unit groups replaced, types and properties extended ("| ..." appends to
// the existing syntax) and at-rule preludes and descriptors merged.
func Mix(dest, src *js.Object) *js.Object {
	result := copyObject(dest)
	for _, prop := range src.Keys() {
		value := src.Get(prop)
		switch prop {
		case "generic":
			result.Set(prop, js.Truthy(value))
		case "cssWideKeywords":
			if prev, ok := dest.Get(prop).([]any); ok && js.Truthy(dest.Get(prop)) {
				result.Set(prop, append(append([]any{}, prev...), iterate(value)...))
			} else if js.Truthy(value) {
				result.Set(prop, value)
			} else {
				result.Set(prop, []any{})
			}
		case "units":
			units := copyObject(entries(dest.Get(prop)))
			for _, name := range entries(value).Keys() {
				if list, ok := entries(value).Get(name).([]any); ok {
					units.Set(name, list)
				} else {
					units.Set(name, []any{})
				}
			}
			result.Set(prop, units)
		case "atrules":
			result.Set(prop, mixAtrules(entries(dest.Get(prop)), entries(value)))
		case "types", "properties":
			merged := copyObject(entries(dest.Get(prop)))
			for _, name := range entries(value).Keys() {
				merged.Set(name, appendOrSet(lookupValue(merged, name), entries(value).Get(name)))
			}
			result.Set(prop, merged)
		}
	}
	return result
}

func lookupValue(o *js.Object, key string) any {
	if v, ok := o.Lookup(key); ok {
		return v
	}
	return js.Undefined
}

// iterate spreads a value the way [...value] does for arrays and strings.
func iterate(v any) []any {
	switch x := v.(type) {
	case []any:
		return x
	case string:
		var out []any
		for _, r := range x {
			out = append(out, string(r))
		}
		return out
	}
	return nil
}

func mixAtrules(dest, src *js.Object) *js.Object {
	result := copyObject(dest)
	for _, name := range src.Keys() {
		atrule := src.Get(name)
		exists := entries(result.Get(name))
		var prelude any
		if p := exists.Get("prelude"); js.Truthy(p) {
			prelude = p
		}
		descriptors := copyObject(entries(exists.Get("descriptors")))
		current := js.NewObject("prelude", prelude, "descriptors", descriptors)
		result.Set(name, current)
		a, _ := atrule.(*js.Object)
		if !js.Truthy(atrule) || a == nil {
			continue
		}
		if p := a.Get("prelude"); js.Truthy(p) {
			current.Set("prelude", appendOrSet(current.Get("prelude"), p))
		}
		for _, dn := range entries(a.Get("descriptors")).Keys() {
			dv := entries(a.Get("descriptors")).Get(dn)
			if js.Truthy(dv) {
				descriptors.Set(dn, appendOrSet(lookupValue(descriptors, dn), dv))
			} else {
				descriptors.Set(dn, nil)
			}
		}
		if descriptors.Len() == 0 {
			current.Set("descriptors", nil)
		}
	}
	return result
}
