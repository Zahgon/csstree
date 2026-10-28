// Package utils holds CSSTree's name helpers (keyword and property descriptors, vendor prefixes, custom
// properties) and the encoders and decoders of identifiers, strings and urls.
package utils

import (
	"sync"

	"github.com/csstree/csstree/js"
)

// Descriptor describes a keyword or a property name: its lowercased name, the name without a vendor prefix, the
// prefix and, for properties, a leading hack character.
type Descriptor struct {
	Basename string
	Name     string
	Hack     string
	Vendor   string
	Prefix   string
	Custom   bool
	property bool
}

// JSObject returns the descriptor with the original's keys: basename, name, prefix, vendor, custom for a keyword
// and basename, name, hack, vendor, prefix, custom for a property.
func (d Descriptor) JSObject() *js.Object {
	if d.property {
		return js.NewObject("basename", d.Basename, "name", d.Name, "hack", d.Hack, "vendor", d.Vendor,
			"prefix", d.Prefix, "custom", d.Custom)
	}
	return js.NewObject("basename", d.Basename, "name", d.Name, "prefix", d.Prefix, "vendor", d.Vendor,
		"custom", d.Custom)
}

const hyphenMinus = 45

var (
	cacheMu    sync.Mutex
	keywords   = map[string]Descriptor{}
	properties = map[string]Descriptor{}
)

// IsCustomProperty reports whether str starts with "--" at offset.
func IsCustomProperty(str string, offset int) bool {
	units := js.UTF16(str)
	return len(units)-offset >= 2 && offset >= 0 && units[offset] == hyphenMinus && units[offset+1] == hyphenMinus
}

// VendorPrefix returns the vendor prefix ("-webkit-") of str at offset, or "".
func VendorPrefix(str string, offset int) string {
	units := js.UTF16(str)
	if len(units)-offset >= 3 && offset >= 0 {
		if units[offset] == hyphenMinus && units[offset+1] != hyphenMinus {
			if secondDashIndex := js.IndexOf(str, "-", offset+2); secondDashIndex != -1 {
				return js.Substring(str, offset, secondDashIndex+1)
			}
		}
	}
	return ""
}

// Keyword returns the descriptor of a keyword name; results are cached by the given spelling.
func Keyword(keyword string) Descriptor {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if d, ok := keywords[keyword]; ok {
		return d
	}
	name := js.ToLowerCase(keyword)
	d, ok := keywords[name]
	if !ok {
		custom := IsCustomProperty(name, 0)
		vendor := ""
		if !custom {
			vendor = VendorPrefix(name, 0)
		}
		d = Descriptor{
			Basename: js.Substr(name, js.Len(vendor), js.Len(name)),
			Name:     name,
			Prefix:   vendor,
			Vendor:   vendor,
			Custom:   custom,
		}
	}
	keywords[keyword] = d
	return d
}

// Property returns the descriptor of a property name, recognising the hacks "_", "*", "$", "#", "+", "&", "/"
// and "//"; results are cached by the given spelling.
func Property(property string) Descriptor {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if d, ok := properties[property]; ok {
		return d
	}
	units := js.UTF16(property)
	name := property
	hack := ""
	if len(units) > 0 {
		switch units[0] {
		case '/':
			hack = "/"
			if len(units) > 1 && units[1] == '/' {
				hack = "//"
			}
		case '_', '*', '$', '#', '+', '&':
			hack = string(rune(units[0]))
		}
	}
	custom := IsCustomProperty(name, len(hack))
	if !custom {
		name = js.ToLowerCase(name)
		if d, ok := properties[name]; ok {
			properties[property] = d
			return d
		}
	}
	vendor := ""
	if !custom {
		vendor = VendorPrefix(name, len(hack))
	}
	prefix := js.Substr(name, 0, len(hack)+js.Len(vendor))
	d := Descriptor{
		Basename: js.Substr(name, js.Len(prefix), js.Len(name)),
		Name:     js.Substr(name, len(hack), js.Len(name)),
		Hack:     hack,
		Vendor:   vendor,
		Prefix:   prefix,
		Custom:   custom,
		property: true,
	}
	properties[property] = d
	return d
}
