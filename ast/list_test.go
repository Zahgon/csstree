package ast

import (
	"fmt"
	"testing"

	"github.com/csstree/csstree/js"
)

func values(l *List) string {
	return js.Stringify(l.ToJSON())
}

func TestListBasics(t *testing.T) {
	l := NewList()
	if !l.IsEmpty() || l.Size() != 0 || l.First() != nil || l.Last() != nil {
		t.Fatal("a new list must be empty")
	}
	l.FromArray([]any{1, 2, 3})
	if l.IsEmpty() || l.Size() != 3 || l.First() != 1 || l.Last() != 3 || values(l) != "[1,2,3]" {
		t.Fatalf("fromArray: %s size %d", values(l), l.Size())
	}
	if got := values(l.Copy()); got != "[1,2,3]" {
		t.Fatalf("copy: %s", got)
	}
	l.Clear()
	if !l.IsEmpty() {
		t.Fatal("clear must empty the list")
	}
}

func TestListMutations(t *testing.T) {
	l := NewList().FromArray([]any{2})
	l.PrependData(1).AppendData(4)
	if err := l.InsertData(3, l.Tail); err != nil {
		t.Fatal(err)
	}
	l.Push(5)
	l.Unshift(0)
	if got := values(l); got != "[0,1,2,3,4,5]" {
		t.Fatalf("got %s", got)
	}
	if item := l.Pop(); item.Data != 5 || l.Shift().Data != 0 || values(l) != "[1,2,3,4]" {
		t.Fatalf("pop and shift: %s", values(l))
	}
	if _, err := l.Remove(l.Head.Next); err != nil || values(l) != "[1,3,4]" {
		t.Fatalf("remove: %s %v", values(l), err)
	}
	other := NewList().FromArray([]any{"a", "b"})
	l.InsertList(other, l.Head.Next)
	if values(l) != `[1,"a","b",3,4]` || !other.IsEmpty() {
		t.Fatalf("insertList: %s", values(l))
	}
	l.AppendList(NewList().FromArray([]any{9})).PrependList(NewList().FromArray([]any{0}))
	if values(l) != `[0,1,"a","b",3,4,9]` {
		t.Fatalf("append and prepend lists: %s", values(l))
	}
	if err := l.Replace(l.Head, CreateItem("x")); err != nil || values(l) != `["x",1,"a","b",3,4,9]` {
		t.Fatalf("replace with an item: %s %v", values(l), err)
	}
	if err := l.Replace(l.Tail, NewList().FromArray([]any{7, 8})); err != nil || values(l) != `["x",1,"a","b",3,4,7,8]` {
		t.Fatalf("replace with a list: %s %v", values(l), err)
	}
}

func TestListErrors(t *testing.T) {
	l := NewList().FromArray([]any{1, 2})
	stranger := CreateItem(3)
	if err := l.Insert(CreateItem(0), stranger); err != ErrBeforeNotInList {
		t.Fatalf("insert before a foreign item: %v", err)
	}
	if _, err := l.Remove(CreateItem(4)); err != ErrItemNotInList {
		t.Fatalf("remove a foreign item: %v", err)
	}
}

func TestListIteration(t *testing.T) {
	l := NewList().FromArray([]any{1, 2, 3, 4})
	var forward, backward []any
	l.ForEach(func(data any, item *ListItem, _ *List) {
		forward = append(forward, data)
		if data == 2 {
			if _, err := l.Remove(item.Next); err != nil {
				t.Fatal(err)
			}
		}
	})
	l.ForEachRight(func(data any, _ *ListItem, _ *List) { backward = append(backward, data) })
	if fmt.Sprint(forward) != "[1 2 4]" || fmt.Sprint(backward) != "[4 2 1]" {
		t.Fatalf("forEach %v forEachRight %v", forward, backward)
	}
	sum := l.Reduce(func(acc, data any, _ *ListItem, _ *List) any { return acc.(int) + data.(int) }, 0)
	order := l.ReduceRight(func(acc, data any, _ *ListItem, _ *List) any { return fmt.Sprint(acc, data) }, "")
	if sum != 7 || order != "421" {
		t.Fatalf("reduce %v reduceRight %v", sum, order)
	}
	doubled := l.Map(func(data any, _ *ListItem, _ *List) any { return data.(int) * 2 })
	even := l.Filter(func(data any, _ *ListItem, _ *List) bool { return data.(int)%2 == 0 })
	some := l.Some(func(data any, _ *ListItem, _ *List) bool { return data == 4 })
	if values(doubled) != "[2,4,8]" || values(even) != "[2,4]" || !some {
		t.Fatalf("map %s filter %s some %v", values(doubled), values(even), some)
	}
	var next, prev []any
	l.NextUntil(l.Head.Next, func(data any, _ *ListItem, _ *List) bool { next = append(next, data); return false })
	l.PrevUntil(l.Tail, func(data any, _ *ListItem, _ *List) bool { prev = append(prev, data); return data == 2 })
	if fmt.Sprint(next) != "[2 4]" || fmt.Sprint(prev) != "[4 2]" {
		t.Fatalf("nextUntil %v prevUntil %v", next, prev)
	}
}

func TestClone(t *testing.T) {
	node := js.NewObject("type", "Value", "loc", js.NewObject("source", "x"), "children",
		NewList().FromArray([]any{js.NewObject("type", "Identifier", "name", "a")}), "extra", []any{1, "ab"})
	c, err := Clone(node)
	if err != nil {
		t.Fatal(err)
	}
	if got := js.Stringify(c); got != `{"type":"Value","loc":{"source":"x"},"children":[{"type":"Identifier","name":"a"}],"extra":[{},{"0":"a","1":"b"}]}` {
		t.Fatalf("clone: %s", got)
	}
	if c.Obj("loc") == node.Obj("loc") || c.Get("children") == node.Get("children") {
		t.Fatal("clone must copy nested objects and lists")
	}
}
