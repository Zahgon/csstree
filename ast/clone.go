package ast

import (
	"errors"
	"strconv"

	"github.com/csstree/csstree/internal/js"
)

// ErrCloneNull is returned when Clone meets null or undefined where it expects an object, the original's
// "Cannot convert undefined or null to object" TypeError.
var ErrCloneNull = errors.New("Cannot convert undefined or null to object")

// Clone deep-copies a node: plain objects are copied key by key, Lists and arrays element by element, and every
// other value is shared. Like the original, an element that is not an object is copied through its own keys, so
// a number or boolean becomes {} and a string becomes an object of its characters.
func Clone(node any) (*js.Object, error) {
	result := js.NewObject()
	switch n := node.(type) {
	case nil:
		return nil, ErrCloneNull
	case *js.Object:
		if n == nil {
			return nil, ErrCloneNull
		}
		for _, key := range n.Keys() {
			value, _ := n.Lookup(key)
			cloned, err := cloneValue(value)
			if err != nil {
				return nil, err
			}
			result.Set(key, cloned)
		}
	case string:
		for i, unit := range js.UTF16(n) {
			result.Set(strconv.Itoa(i), js.FromUTF16([]uint16{unit}))
		}
	}
	if node == js.Undefined {
		return nil, ErrCloneNull
	}
	return result, nil
}

func cloneValue(value any) (any, error) {
	if !js.Truthy(value) {
		return value, nil
	}
	switch v := value.(type) {
	case *List:
		var err error
		mapped := v.Map(func(data any, _ *ListItem, _ *List) any {
			c, e := Clone(data)
			if e != nil && err == nil {
				err = e
			}
			return c
		})
		return mapped, err
	case []any:
		out := make([]any, len(v))
		for i, data := range v {
			c, err := Clone(data)
			if err != nil {
				return nil, err
			}
			out[i] = c
		}
		return out, nil
	case *js.Object:
		return Clone(v)
	}
	return value, nil
}
