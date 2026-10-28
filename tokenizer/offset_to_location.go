package tokenizer

import "github.com/csstree/csstree/js"

// OffsetToLocation maps source offsets to lines and columns. Lines and columns are stored as unsigned 32-bit
// integers, so start values outside that range wrap (a start line of -1 reads 4294967295); offsets are plain
// numbers.
type OffsetToLocation struct {
	Source      string
	StartOffset int
	StartLine   int
	StartColumn int
	src         []uint16
	lines       []uint32
	columns     []uint32
	computed    bool
}

// NewOffsetToLocation returns a map over an empty source.
func NewOffsetToLocation() *OffsetToLocation {
	m := &OffsetToLocation{}
	m.SetSource("", 0, 1, 1)
	return m
}

// SetSource sets the source and the location of its first character (the original's defaults are 0, 1 and 1).
func (m *OffsetToLocation) SetSource(source string, startOffset, startLine, startColumn int) {
	m.Source = source
	m.src = js.UTF16(source)
	m.StartOffset = startOffset
	m.StartLine = startLine
	m.StartColumn = startColumn
	m.computed = false
}

func (m *OffsetToLocation) compute() {
	sourceLength := len(m.src)
	startOffset := 0
	if sourceLength > 0 {
		startOffset = IsBOM(int(m.src[0]))
	}
	m.lines = adoptBuffer(m.lines, sourceLength)
	m.columns = adoptBuffer(m.columns, sourceLength)
	line := m.StartLine
	column := m.StartColumn
	for i := startOffset; i < sourceLength; i++ {
		code := m.src[i]
		m.lines[i] = js.ToUint32(float64(line))
		m.columns[i] = js.ToUint32(float64(column))
		column++
		if code == '\n' || code == '\r' || code == '\f' {
			if code == '\r' && i+1 < sourceLength && m.src[i+1] == '\n' {
				i++
				m.lines[i] = js.ToUint32(float64(line))
				m.columns[i] = js.ToUint32(float64(column))
			}
			line++
			column = 1
		}
	}
	m.lines[sourceLength] = js.ToUint32(float64(line))
	m.columns[sourceLength] = js.ToUint32(float64(column))
	m.computed = true
}

func (m *OffsetToLocation) point(offset int) *js.Object {
	if !m.computed {
		m.compute()
	}
	var line, column any = js.Undefined, js.Undefined
	if offset >= 0 && offset < len(m.lines) {
		line, column = int(m.lines[offset]), int(m.columns[offset])
	}
	return js.NewObject("offset", m.StartOffset+offset, "line", line, "column", column)
}

// GetLocation returns {source, offset, line, column} for an offset.
func (m *OffsetToLocation) GetLocation(offset int, filename string) *js.Object {
	p := m.point(offset)
	loc := js.NewObject("source", filename)
	for _, k := range p.Keys() {
		v, _ := p.Lookup(k)
		loc.Set(k, v)
	}
	return loc
}

// GetLocationRange returns {source, start: {offset, line, column}, end: {...}} for two offsets.
func (m *OffsetToLocation) GetLocationRange(start, end int, filename string) *js.Object {
	return js.NewObject("source", filename, "start", m.point(start), "end", m.point(end))
}
