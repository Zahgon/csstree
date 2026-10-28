package ast

// Clear removes every item.
func (l *List) Clear() {
	l.Head = nil
	l.Tail = nil
}

// Copy returns a new list holding the same data.
func (l *List) Copy() *List {
	result := NewList()
	for c := l.Head; c != nil; c = c.Next {
		result.AppendData(c.Data)
	}
	return result
}

// Prepend inserts item at the beginning.
func (l *List) Prepend(item *ListItem) *List {
	l.updateCursors(nil, item, l.Head, item, true)
	if l.Head != nil {
		l.Head.Prev = item
		item.Next = l.Head
	} else {
		l.Tail = item
	}
	l.Head = item
	return l
}

// PrependData inserts data at the beginning.
func (l *List) PrependData(data any) *List {
	return l.Prepend(CreateItem(data))
}

// Append inserts item at the end.
func (l *List) Append(item *ListItem) *List {
	l.appendItem(item)
	return l
}

// AppendData inserts data at the end.
func (l *List) AppendData(data any) *List {
	l.appendItem(CreateItem(data))
	return l
}

func (l *List) appendItem(item *ListItem) {
	l.updateCursors(l.Tail, item, nil, item, true)
	if l.Tail != nil {
		l.Tail.Next = item
		item.Prev = l.Tail
	} else {
		l.Head = item
	}
	l.Tail = item
}

// Insert inserts item before another item of the list, or at the end when before is nil.
func (l *List) Insert(item, before *ListItem) error {
	if before == nil {
		l.appendItem(item)
		return nil
	}
	l.updateCursors(before.Prev, item, before, item, true)
	if before.Prev == nil {
		if l.Head != before {
			return ErrBeforeNotInList
		}
		l.Head = item
		before.Prev = item
		item.Next = before
		l.updateCursors(nil, item, nil, nil, false)
		return nil
	}
	before.Prev.Next = item
	item.Prev = before.Prev
	before.Prev = item
	item.Next = before
	return nil
}

// InsertData inserts data before another item of the list, or at the end when before is nil.
func (l *List) InsertData(data any, before *ListItem) error {
	return l.Insert(CreateItem(data), before)
}

// Remove unlinks an item of the list and returns it.
func (l *List) Remove(item *ListItem) (*ListItem, error) {
	l.updateCursors(item, item.Prev, item, item.Next, true)
	if item.Prev != nil {
		item.Prev.Next = item.Next
	} else {
		if l.Head != item {
			return nil, ErrItemNotInList
		}
		l.Head = item.Next
	}
	if item.Next != nil {
		item.Next.Prev = item.Prev
	} else {
		if l.Tail != item {
			return nil, ErrItemNotInList
		}
		l.Tail = item.Prev
	}
	item.Prev = nil
	item.Next = nil
	return item, nil
}

// Push appends data.
func (l *List) Push(data any) {
	l.appendItem(CreateItem(data))
}

// Pop removes and returns the last item, or nil.
func (l *List) Pop() *ListItem {
	if l.Tail == nil {
		return nil
	}
	item, _ := l.Remove(l.Tail)
	return item
}

// Unshift prepends data.
func (l *List) Unshift(data any) {
	l.Prepend(CreateItem(data))
}

// Shift removes and returns the first item, or nil.
func (l *List) Shift() *ListItem {
	if l.Head == nil {
		return nil
	}
	item, _ := l.Remove(l.Head)
	return item
}

// PrependList moves every item of list to the beginning.
func (l *List) PrependList(list *List) *List {
	return l.InsertList(list, l.Head)
}

// AppendList moves every item of list to the end.
func (l *List) AppendList(list *List) *List {
	return l.InsertList(list, nil)
}

// InsertList moves every item of list before an item of this list, or to the end when before is nil; list is
// left empty.
func (l *List) InsertList(list *List, before *ListItem) *List {
	if list.Head == nil {
		return l
	}
	if before != nil {
		l.updateCursors(before.Prev, list.Tail, before, list.Head, true)
		if before.Prev != nil {
			before.Prev.Next = list.Head
			list.Head.Prev = before.Prev
		} else {
			l.Head = list.Head
		}
		before.Prev = list.Tail
		list.Tail.Next = before
	} else {
		l.updateCursors(l.Tail, list.Tail, nil, list.Head, true)
		if l.Tail != nil {
			l.Tail.Next = list.Head
			list.Head.Prev = l.Tail
		} else {
			l.Head = list.Head
		}
		l.Tail = list.Tail
	}
	list.Head = nil
	list.Tail = nil
	return l
}

// Replace puts an item or every item of a list in place of oldItem.
func (l *List) Replace(oldItem *ListItem, newItemOrList any) error {
	switch v := newItemOrList.(type) {
	case *List:
		l.InsertList(v, oldItem)
	case *ListItem:
		if err := l.Insert(v, oldItem); err != nil {
			return err
		}
	}
	_, err := l.Remove(oldItem)
	return err
}
