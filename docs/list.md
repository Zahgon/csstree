# List

`ast.List` is the doubly linked list that holds the children of a node. Its iterations stay valid when the
callback removes or inserts items. It serialises to a JSON array.

| method | |
|---|---|
| `NewList()`, `CreateItem(data)` | an empty list, a detached item |
| `Size()`, `IsEmpty()`, `First()`, `Last()` | |
| `FromArray(values)`, `ToArray()`, `ToJSON()`, `Copy()`, `Clear()` | |
| `ForEach`, `ForEachRight`, `Reduce`, `ReduceRight`, `Some`, `Map`, `Filter`, `NextUntil`, `PrevUntil` | iteration |
| `Prepend`, `PrependData`, `Append`, `AppendData`, `Push`, `Unshift` | adding |
| `Insert(item, before)`, `InsertData(data, before)` | inserting; an error when `before` is not in the list |
| `Remove(item)`, `Pop()`, `Shift()` | removing; `Remove` fails for an item that is not in the list |
| `PrependList`, `AppendList`, `InsertList(list, before)`, `Replace(old, itemOrList)` | moving the items of another list |
