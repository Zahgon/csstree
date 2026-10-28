// Package js reproduces the JavaScript value semantics that CSSTree's observable output depends on: plain objects
// with JavaScript property order, JSON.stringify and JSON.parse, Number to string conversion, string to Number
// conversion, toLowerCase with full Unicode case mapping, and strings measured in UTF-16 code units.
//
// Strings are Go strings in WTF-8: UTF-8 that may also carry lone surrogate code points (encoded like any other
// three-byte sequence), so every JavaScript string, including one cut through a surrogate pair, has a Go form.
package js

import (
	"strconv"
	"strings"
)

// Undefined is the JavaScript undefined value when it has to be stored in an Object or an array. JSON output
// skips object properties holding it and writes it as null inside arrays.
var Undefined = undefinedValue{}

type undefinedValue struct{}

// Object is a JavaScript plain object: string keys mapped to values, enumerated in JavaScript property order
// (array-index keys in ascending numeric order first, then every other key in insertion order).
//
// Values are nil (null), Undefined, bool, float64 or int (number), string, *Object, []any (array) or any value
// implementing JSONer.
type Object struct {
	keys []string
	vals map[string]any
}

// JSONer is implemented by values that JSON.stringify serialises through their toJSON method.
type JSONer interface {
	ToJSON() any
}

// NewObject returns an object holding the given key and value pairs in order; kv alternates string keys and values.
func NewObject(kv ...any) *Object {
	o := &Object{vals: make(map[string]any, len(kv)/2)}
	for i := 0; i+1 < len(kv); i += 2 {
		o.Set(kv[i].(string), kv[i+1])
	}
	return o
}

// Set assigns a property; a new key is appended, an existing key keeps its position. A nil *Object is stored
// as null.
func (o *Object) Set(key string, value any) {
	if v, ok := value.(*Object); ok && v == nil {
		value = nil
	}
	if o.vals == nil {
		o.vals = map[string]any{}
	}
	if _, ok := o.vals[key]; !ok {
		o.keys = append(o.keys, key)
	}
	o.vals[key] = value
}

// Lookup returns the value of an own property and whether it exists.
func (o *Object) Lookup(key string) (any, bool) {
	if o == nil {
		return nil, false
	}
	v, ok := o.vals[key]
	return v, ok
}

// Get returns the value of an own property, or nil when it is absent.
func (o *Object) Get(key string) any {
	v, _ := o.Lookup(key)
	if v == Undefined {
		return nil
	}
	return v
}

// Has reports whether the object has an own property named key.
func (o *Object) Has(key string) bool {
	_, ok := o.Lookup(key)
	return ok
}

// Delete removes an own property.
func (o *Object) Delete(key string) {
	if o == nil {
		return
	}
	if _, ok := o.vals[key]; !ok {
		return
	}
	delete(o.vals, key)
	for i, k := range o.keys {
		if k == key {
			o.keys = append(o.keys[:i:i], o.keys[i+1:]...)
			break
		}
	}
}

// Len returns the number of own properties.
func (o *Object) Len() int {
	if o == nil {
		return 0
	}
	return len(o.keys)
}

// Keys returns the own property keys in JavaScript order: array-index keys ascending, then insertion order.
func (o *Object) Keys() []string {
	if o == nil {
		return nil
	}
	var index []string
	var other []string
	for _, k := range o.keys {
		if IsArrayIndex(k) {
			index = append(index, k)
		} else {
			other = append(other, k)
		}
	}
	if len(index) == 0 {
		return other
	}
	for i := 1; i < len(index); i++ {
		for j := i; j > 0 && arrayIndexLess(index[j], index[j-1]); j-- {
			index[j], index[j-1] = index[j-1], index[j]
		}
	}
	return append(index, other...)
}

// Str returns a string property, or "" when the property is absent or not a string.
func (o *Object) Str(key string) string {
	s, _ := o.Get(key).(string)
	return s
}

// Obj returns an object property, or nil when the property is absent or not an object.
func (o *Object) Obj(key string) *Object {
	v, _ := o.Get(key).(*Object)
	return v
}

// Bool returns a boolean property, or false when the property is absent or not a boolean.
func (o *Object) Bool(key string) bool {
	b, _ := o.Get(key).(bool)
	return b
}

// Num returns a number property as float64 and whether the property holds a number.
func (o *Object) Num(key string) (float64, bool) {
	switch v := o.Get(key).(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	}
	return 0, false
}

// Truthy reports whether a JavaScript value converts to true.
func Truthy(v any) bool {
	switch x := v.(type) {
	case nil, undefinedValue:
		return false
	case bool:
		return x
	case float64:
		return x != 0 && x == x
	case int:
		return x != 0
	case string:
		return x != ""
	}
	return true
}

// IsArrayIndex reports whether key is a canonical array index (an integer from 0 to 2^32-2 without leading zeros).
func IsArrayIndex(key string) bool {
	if key == "" || len(key) > 10 || (len(key) > 1 && key[0] == '0') {
		return false
	}
	for i := 0; i < len(key); i++ {
		if key[i] < '0' || key[i] > '9' {
			return false
		}
	}
	n, err := strconv.ParseUint(key, 10, 64)
	return err == nil && n < 4294967295
}

func arrayIndexLess(a, b string) bool {
	if len(a) != len(b) {
		return len(a) < len(b)
	}
	return a < b
}

// ToString is String(value) for the values an Object holds.
func ToString(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case undefinedValue:
		return "undefined"
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		return NumberToString(x)
	case int:
		return strconv.Itoa(x)
	case string:
		return x
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			if e != nil && e != Undefined {
				parts[i] = ToString(e)
			}
		}
		return strings.Join(parts, ",")
	}
	return "[object Object]"
}
