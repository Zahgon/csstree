package generator

import (
	"errors"
	"slices"
	"strings"

	"github.com/csstree/csstree/js"
)

// mapping is one Source Map v3 mapping; a generated-only mapping has hasSource false.
type mapping struct {
	genLine, genColumn   float64
	origLine, origColumn float64
	source               string
	hasSource            bool
}

// sourceMap accumulates mappings and writes the Source Map v3 JSON (version, sources, names, mappings).
type sourceMap struct {
	sources  []string
	mappings []mapping
	sorted   bool
	last     mapping
}

func newSourceMap() *sourceMap {
	return &sourceMap{sorted: true, last: mapping{genLine: -1}}
}

func compareStrings(a, b string, aSet, bSet bool) int {
	switch {
	case aSet == bSet && a == b:
		return 0
	case !aSet:
		return 1
	case !bSet:
		return -1
	}
	return slices.Compare(js.UTF16(a), js.UTF16(b))
}

// compareMappings orders mappings by generated position, then source, original position (and name).
func compareMappings(a, b mapping) float64 {
	if d := a.genLine - b.genLine; d != 0 {
		return d
	}
	if d := a.genColumn - b.genColumn; d != 0 {
		return d
	}
	if d := compareStrings(a.source, b.source, a.hasSource, b.hasSource); d != 0 {
		return float64(d)
	}
	if d := a.origLine - b.origLine; d != 0 {
		return d
	}
	return a.origColumn - b.origColumn
}

func (m *sourceMap) add(mp mapping) {
	if mp.hasSource && !slices.Contains(m.sources, mp.source) {
		m.sources = append(m.sources, mp.source)
	}
	last := m.last
	if mp.genLine > last.genLine || (mp.genLine == last.genLine && mp.genColumn >= last.genColumn) ||
		compareMappings(last, mp) <= 0 {
		m.last = mp
	} else {
		m.sorted = false
	}
	m.mappings = append(m.mappings, mp)
}

// addTracked adds a mapping from a generated position to an original one, validated like the original
// generator: lines start at 1, columns at 0 and the source is not empty.
func (m *sourceMap) addTracked(source string, origLine, origColumn, genLine, genColumn float64) error {
	if genLine > 0 && genColumn >= 0 && origLine > 0 && origColumn >= 0 && source != "" {
		m.add(mapping{genLine: genLine, genColumn: genColumn, origLine: origLine, origColumn: origColumn,
			source: source, hasSource: true})
		return nil
	}
	return errors.New("Invalid mapping: " + js.Stringify(js.NewObject(
		"generated", js.NewObject("line", genLine, "column", genColumn),
		"source", source,
		"original", js.NewObject("line", origLine, "column", origColumn),
		"name", nil,
	)))
}

const base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

// encodeVLQ writes a base64 VLQ with the 32-bit integer arithmetic of JavaScript bitwise operators.
func encodeVLQ(b *strings.Builder, value float64) {
	var signed float64
	if value < 0 {
		signed = float64(js.ToInt32(-value)<<1) + 1
	} else {
		signed = float64(js.ToInt32(value) << 1)
	}
	digit := uint32(js.ToInt32(signed)) & 31
	vlq := js.ToUint32(signed) >> 5
	for {
		if vlq > 0 {
			digit |= 32
		}
		b.WriteByte(base64Chars[digit])
		if vlq == 0 {
			return
		}
		digit = vlq & 31
		vlq >>= 5
	}
}

func (m *sourceMap) serialize() string {
	if !m.sorted {
		slices.SortStableFunc(m.mappings, func(a, b mapping) int {
			switch d := compareMappings(a, b); {
			case d < 0:
				return -1
			case d > 0:
				return 1
			}
			return 0
		})
		m.sorted = true
	}
	var b strings.Builder
	prevGenColumn, prevGenLine := 0.0, 1.0
	prevOrigColumn, prevOrigLine, prevSource := 0.0, 0.0, 0.0
	for i, mp := range m.mappings {
		if mp.genLine != prevGenLine {
			prevGenColumn = 0
			for mp.genLine != prevGenLine && prevGenLine <= mp.genLine {
				b.WriteByte(';')
				prevGenLine++
			}
		} else if i > 0 {
			if compareMappings(mp, m.mappings[i-1]) == 0 {
				continue
			}
			b.WriteByte(',')
		}
		encodeVLQ(&b, mp.genColumn-prevGenColumn)
		prevGenColumn = mp.genColumn
		if mp.hasSource {
			sourceIdx := float64(slices.Index(m.sources, mp.source))
			encodeVLQ(&b, sourceIdx-prevSource)
			prevSource = sourceIdx
			encodeVLQ(&b, mp.origLine-1-prevOrigLine)
			prevOrigLine = mp.origLine - 1
			encodeVLQ(&b, mp.origColumn-prevOrigColumn)
			prevOrigColumn = mp.origColumn
		}
	}
	return b.String()
}

func (m *sourceMap) toJSON() *js.Object {
	sources := make([]any, len(m.sources))
	for i, s := range m.sources {
		sources[i] = s
	}
	return js.NewObject("version", 3, "sources", sources, "names", []any{}, "mappings", m.serialize())
}

var trackNodes = map[string]bool{"Atrule": true, "Selector": true, "Declaration": true}

// GenerateWithSourceMap returns the CSS of a node and a Source Map v3 that maps the generated position of every
// at-rule, selector and declaration with a location back to that location.
func GenerateWithSourceMap(node any, opts Options) (string, *js.Object, error) {
	h := newHandlers()
	m := newSourceMap()
	line, column := 1.0, 0.0
	origLine, origColumn := 0.0, 0.0
	activatedLine, activatedColumn := 1.0, 0.0
	active := false
	origNode, origEmit := h.Node, h.Emit
	h.Node = func(n any) {
		obj, _ := n.(*js.Object)
		start := obj.Obj("loc").Obj("start")
		typ := obj.Str("type")
		if start != nil && trackNodes[typ] {
			nodeLine, _ := start.Num("line")
			nodeColumn, _ := start.Num("column")
			nodeColumn--
			if origLine != nodeLine || origColumn != nodeColumn {
				origLine, origColumn = nodeLine, nodeColumn
				if active {
					active = false
					if line != activatedLine || column != activatedColumn {
						m.add(mapping{genLine: activatedLine, genColumn: activatedColumn})
					}
				}
				active = true
				if err := m.addTracked(js.ToString(obj.Obj("loc").Get("source")), origLine, origColumn, line, column); err != nil {
					panic(&generateError{err.Error()})
				}
			}
		}
		origNode(n)
		if active && trackNodes[typ] {
			activatedLine, activatedColumn = line, column
		}
	}
	h.Emit = func(value string, typ int, auto bool) {
		for _, u := range js.UTF16(value) {
			if u == '\n' {
				line++
				column = 0
			} else {
				column++
			}
		}
		origEmit(value, typ, auto)
	}
	origResult := h.Result
	h.Result = func() any {
		if active {
			m.add(mapping{genLine: activatedLine, genColumn: activatedColumn})
		}
		return origResult()
	}
	result, err := run(node, h, opts.Mode)
	if err != nil {
		return "", nil, err
	}
	return result.(string), m.toJSON(), nil
}
