// Package ast holds the data structures of the CSSTree AST: the doubly linked List used for node children and
// the deep Clone of a node. Nodes themselves are *js.Object values whose keys follow the original's object order.
package ast

import "errors"

// ListItem is one entry of a List.
type ListItem struct {
	Prev *ListItem
	Next *ListItem
	Data any
}

// cursor marks an iteration in progress so that removals and insertions during the iteration stay safe.
type cursor struct {
	prev, next *ListItem
	cursor     *cursor
}

// List is a doubly linked list whose iterations tolerate changes to the list made by their callbacks.
type List struct {
	Head   *ListItem
	Tail   *ListItem
	cursor *cursor
}

// Errors returned when an item passed to Insert or Remove is not part of the list.
var (
	ErrBeforeNotInList = errors.New("before doesn't belong to list")
	ErrItemNotInList   = errors.New("item doesn't belong to list")
)

// NewList returns an empty list.
func NewList() *List {
	return &List{}
}

// CreateItem returns a detached item holding data.
func CreateItem(data any) *ListItem {
	return &ListItem{Data: data}
}

// CreateItem returns a detached item holding data.
func (l *List) CreateItem(data any) *ListItem {
	return CreateItem(data)
}

func (l *List) allocateCursor(prev, next *ListItem) *cursor {
	c := &cursor{prev: prev, next: next, cursor: l.cursor}
	l.cursor = c
	return c
}

func (l *List) releaseCursor() {
	l.cursor = l.cursor.cursor
}

func (l *List) updateCursors(prevOld, prevNew, nextOld, nextNew *ListItem, updateNext bool) {
	for c := l.cursor; c != nil; c = c.cursor {
		if c.prev == prevOld {
			c.prev = prevNew
		}
		if updateNext && c.next == nextOld {
			c.next = nextNew
		}
	}
}

// Size returns the number of items.
func (l *List) Size() int {
	size := 0
	for c := l.Head; c != nil; c = c.Next {
		size++
	}
	return size
}

// IsEmpty reports whether the list has no items.
func (l *List) IsEmpty() bool {
	return l.Head == nil
}

// First returns the data of the first item, or nil.
func (l *List) First() any {
	if l.Head == nil {
		return nil
	}
	return l.Head.Data
}

// Last returns the data of the last item, or nil.
func (l *List) Last() any {
	if l.Tail == nil {
		return nil
	}
	return l.Tail.Data
}

// FromArray replaces the list's items with the values of array.
func (l *List) FromArray(array []any) *List {
	var c *ListItem
	l.Head = nil
	for _, data := range array {
		item := CreateItem(data)
		if c != nil {
			c.Next = item
		} else {
			l.Head = item
		}
		item.Prev = c
		c = item
	}
	l.Tail = c
	return l
}

// ToArray returns the data of every item in order.
func (l *List) ToArray() []any {
	result := []any{}
	for c := l.Head; c != nil; c = c.Next {
		result = append(result, c.Data)
	}
	return result
}

// ToJSON returns the array a JSON serialisation of the list writes.
func (l *List) ToJSON() any {
	return l.ToArray()
}

// ForEach calls fn for every item from first to last.
func (l *List) ForEach(fn func(data any, item *ListItem, list *List)) {
	c := l.allocateCursor(nil, l.Head)
	for c.next != nil {
		item := c.next
		c.next = item.Next
		fn(item.Data, item, l)
	}
	l.releaseCursor()
}

// ForEachRight calls fn for every item from last to first.
func (l *List) ForEachRight(fn func(data any, item *ListItem, list *List)) {
	c := l.allocateCursor(l.Tail, nil)
	for c.prev != nil {
		item := c.prev
		c.prev = item.Prev
		fn(item.Data, item, l)
	}
	l.releaseCursor()
}

// Reduce folds the items from first to last.
func (l *List) Reduce(fn func(acc, data any, item *ListItem, list *List) any, initial any) any {
	c := l.allocateCursor(nil, l.Head)
	acc := initial
	for c.next != nil {
		item := c.next
		c.next = item.Next
		acc = fn(acc, item.Data, item, l)
	}
	l.releaseCursor()
	return acc
}

// ReduceRight folds the items from last to first.
func (l *List) ReduceRight(fn func(acc, data any, item *ListItem, list *List) any, initial any) any {
	c := l.allocateCursor(l.Tail, nil)
	acc := initial
	for c.prev != nil {
		item := c.prev
		c.prev = item.Prev
		acc = fn(acc, item.Data, item, l)
	}
	l.releaseCursor()
	return acc
}

// Some reports whether fn returns true for any item.
func (l *List) Some(fn func(data any, item *ListItem, list *List) bool) bool {
	for c := l.Head; c != nil; c = c.Next {
		if fn(c.Data, c, l) {
			return true
		}
	}
	return false
}

// Map returns a new list of fn's results.
func (l *List) Map(fn func(data any, item *ListItem, list *List) any) *List {
	result := NewList()
	for c := l.Head; c != nil; c = c.Next {
		result.AppendData(fn(c.Data, c, l))
	}
	return result
}

// Filter returns a new list of the data for which fn returns true.
func (l *List) Filter(fn func(data any, item *ListItem, list *List) bool) *List {
	result := NewList()
	for c := l.Head; c != nil; c = c.Next {
		if fn(c.Data, c, l) {
			result.AppendData(c.Data)
		}
	}
	return result
}

// NextUntil calls fn from start towards the end until fn returns true.
func (l *List) NextUntil(start *ListItem, fn func(data any, item *ListItem, list *List) bool) {
	if start == nil {
		return
	}
	c := l.allocateCursor(nil, start)
	for c.next != nil {
		item := c.next
		c.next = item.Next
		if fn(item.Data, item, l) {
			break
		}
	}
	l.releaseCursor()
}

// PrevUntil calls fn from start towards the beginning until fn returns true.
func (l *List) PrevUntil(start *ListItem, fn func(data any, item *ListItem, list *List) bool) {
	if start == nil {
		return
	}
	c := l.allocateCursor(start, nil)
	for c.prev != nil {
		item := c.prev
		c.prev = item.Prev
		if fn(item.Data, item, l) {
			break
		}
	}
	l.releaseCursor()
}
