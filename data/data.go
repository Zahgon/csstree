// Package data provides the value definition syntaxes the lexer validates against: the CSS types, properties
// and at-rules of mdn-data 2.27.1 (copied unchanged into mdn/) with CSSTree's own patch.json applied.
package data

import (
	"embed"
	"fmt"

	"github.com/csstree/csstree/js"
)

//go:embed patch.json mdn/at-rules.json mdn/properties.json mdn/syntaxes.json
var files embed.FS

func readObject(name string) (*js.Object, error) {
	text, err := files.ReadFile(name)
	if err != nil {
		return nil, err
	}
	o, err := js.ParseObject(string(text))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return o, nil
}

// Load returns {types, atrules, properties}: the mdn-data syntaxes, at-rules and properties patched with
// patch.json, every dictionary in the original's key order.
func Load() (*js.Object, error) {
	patch, err := readObject("patch.json")
	if err != nil {
		return nil, err
	}
	atrules, err := readObject("mdn/at-rules.json")
	if err != nil {
		return nil, err
	}
	properties, err := readObject("mdn/properties.json")
	if err != nil {
		return nil, err
	}
	syntaxes, err := readObject("mdn/syntaxes.json")
	if err != nil {
		return nil, err
	}
	preprocessed, err := preprocessAtrules(atrules)
	if err != nil {
		return nil, err
	}
	return js.NewObject(
		"types", patchDictionary(syntaxes, patch.Obj("types")),
		"atrules", patchAtrules(preprocessed, patch.Obj("atrules")),
		"properties", patchDictionary(properties, patch.Obj("properties")),
	), nil
}

// removeBlock removes the first "{" ... last "}" span holding at least one character (the original's
// /\{(.|\s)+\}/ replacement).
func removeBlock(s string) string {
	units := js.UTF16(s)
	last := -1
	for i := len(units) - 1; i >= 0; i-- {
		if units[i] == '}' {
			last = i
			break
		}
	}
	for i, u := range units {
		if u == '{' && last > i+1 {
			return js.FromUTF16(append(append([]uint16{}, units[:i]...), units[last+1:]...))
		}
	}
	return s
}

// atrulePrelude extracts the prelude from an at-rule syntax such as "@media <media-query-list> { ... }".
func atrulePrelude(syntax string) (any, error) {
	runes := []rune(removeBlock(js.Trim(syntax)))
	i := 0
	if len(runes) == 0 || runes[0] != '@' {
		return nil, fmt.Errorf("unexpected at-rule syntax %q", syntax)
	}
	i++
	start := i
	for i < len(runes) && !js.IsWhiteSpace(runes[i]) {
		i++
	}
	wsStart := i
	for i < len(runes) && js.IsWhiteSpace(runes[i]) {
		i++
	}
	if i == start || i == wsStart {
		return nil, fmt.Errorf("unexpected at-rule syntax %q", syntax)
	}
	end := i
	for end < len(runes) && runes[end] != ';' && runes[end] != '{' {
		end++
	}
	if prelude := js.Trim(string(runes[i:end])); prelude != "" {
		return prelude, nil
	}
	return nil, nil
}

func preprocessAtrules(dict *js.Object) (*js.Object, error) {
	result := js.NewObject()
	for _, name := range dict.Keys() {
		atrule := dict.Obj(name)
		var descriptors any
		if d := atrule.Obj("descriptors"); js.Truthy(atrule.Get("descriptors")) && d != nil {
			m := js.NewObject()
			for _, dn := range d.Keys() {
				m.Set(dn, d.Obj(dn).Get("syntax"))
			}
			descriptors = m
		}
		prelude, err := atrulePrelude(atrule.Str("syntax"))
		if err != nil {
			return nil, err
		}
		result.Set(js.Substr(name, 1, js.Len(name)), js.NewObject("prelude", prelude, "descriptors", descriptors))
	}
	return result, nil
}

// extendPrefix returns the length of a leading "|" with the white space around it (the original's /^\s*\|\s*/),
// or -1 when the syntax does not extend another one.
func extendPrefix(s string) int {
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

func patchDictionary(dict, patch *js.Object) *js.Object {
	result := js.NewObject()
	for _, key := range dict.Keys() {
		value := dict.Get(key)
		if !js.Truthy(value) {
			continue
		}
		if o, ok := value.(*js.Object); ok {
			if syntax := o.Get("syntax"); js.Truthy(syntax) {
				value = syntax
			}
		}
		result.Set(key, value)
	}
	for _, key := range patch.Keys() {
		entry := patch.Obj(key)
		syntax, hasSyntax := entry.Get("syntax").(string)
		hasSyntax = hasSyntax && syntax != ""
		if dict.Has(key) {
			switch {
			case hasSyntax && extendPrefix(syntax) >= 0:
				result.Set(key, js.ToString(lookupUndefined(result, key))+" "+js.Trim(syntax))
			case hasSyntax:
				result.Set(key, syntax)
			default:
				result.Delete(key)
			}
		} else if hasSyntax {
			if n := extendPrefix(syntax); n >= 0 {
				syntax = syntax[n:]
			}
			result.Set(key, syntax)
		}
	}
	return result
}

func lookupUndefined(o *js.Object, key string) any {
	if v, ok := o.Lookup(key); ok {
		return v
	}
	return js.Undefined
}

func patchDescriptors(declarations *js.Object) *js.Object {
	result := js.NewObject()
	for _, key := range declarations.Keys() {
		value := declarations.Get(key)
		if s, ok := value.(string); ok {
			value = js.NewObject("syntax", s)
		}
		result.Set(key, value)
	}
	return result
}

func patchAtrules(dict, patch *js.Object) *js.Object {
	result := js.NewObject()
	for _, key := range dict.Keys() {
		entry, inPatch := patch.Lookup(key)
		if inPatch && entry == nil {
			continue
		}
		atrulePatch, _ := entry.(*js.Object)
		if atrulePatch == nil {
			atrulePatch = js.NewObject()
		}
		var prelude any = dict.Obj(key).Get("prelude")
		if !js.Truthy(prelude) {
			prelude = nil
		}
		if inPatch && atrulePatch.Has("prelude") {
			prelude = atrulePatch.Get("prelude")
		}
		descriptors := dict.Obj(key).Obj("descriptors")
		if descriptors == nil {
			descriptors = js.NewObject()
		}
		result.Set(key, js.NewObject("prelude", prelude,
			"descriptors", patchDictionary(descriptors, patchDescriptors(atrulePatch.Obj("descriptors")))))
	}
	for _, key := range patch.Keys() {
		atrulePatch := patch.Obj(key)
		if atrulePatch == nil || dict.Has(key) {
			continue
		}
		var prelude, descriptors any
		if p := atrulePatch.Get("prelude"); js.Truthy(p) {
			prelude = p
		}
		if d := atrulePatch.Obj("descriptors"); js.Truthy(atrulePatch.Get("descriptors")) && d != nil {
			descriptors = patchDictionary(js.NewObject(), patchDescriptors(d))
		}
		result.Set(key, js.NewObject("prelude", prelude, "descriptors", descriptors))
	}
	return result
}
