package tokenizer

import "github.com/csstree/csstree/internal/js"

const (
	offsetMask      = 0x00FFFFFF
	typeShift       = 24
	blockOpenToken  = 1
	blockCloseToken = 2
	minBufferSize   = 16 * 1024
)

var balancePair = func() [32]int {
	var p [32]int
	p[Function] = RightParenthesis
	p[LeftParenthesis] = RightParenthesis
	p[LeftSquareBracket] = RightSquareBracket
	p[LeftCurlyBracket] = RightCurlyBracket
	return p
}()

var blockTokens = func() [32]int {
	var b [32]int
	b[Function] = blockOpenToken
	b[LeftParenthesis] = blockOpenToken
	b[LeftSquareBracket] = blockOpenToken
	b[LeftCurlyBracket] = blockOpenToken
	b[RightParenthesis] = blockCloseToken
	b[RightSquareBracket] = blockCloseToken
	b[RightCurlyBracket] = blockCloseToken
	return b
}()

// adoptBuffer returns buffer when it holds size entries, else a new zeroed buffer of at least 16384 entries.
func adoptBuffer(buffer []uint32, size int) []uint32 {
	if buffer == nil || len(buffer) < size {
		return make([]uint32, max(size+1024, minBufferSize))
	}
	return buffer
}

func boundIndex(index, lo, hi int) int {
	if index < lo {
		return lo
	}
	if index > hi {
		return hi
	}
	return index
}

// TokenStream is a tokenized source with a cursor, block balance information and lookahead helpers.
type TokenStream struct {
	Source          string
	Src             []uint16
	FirstCharOffset int
	TokenIndex      int
	TokenType       int
	TokenStart      int
	TokenEnd        int
	TokenCount      int
	EOF             bool
	// OffsetAndType holds (type << 24) | end for each token.
	OffsetAndType []uint32
	// Balance holds, for each token, the index of the token that closes or opens its block.
	Balance []uint32
}

// NewTokenStream returns a stream over source split by tokenize.
func NewTokenStream(source string, tokenize func(string, func(int, int, int))) *TokenStream {
	ts := &TokenStream{}
	ts.SetSource(source, tokenize)
	return ts
}

// Reset moves the cursor before the first token.
func (ts *TokenStream) Reset() {
	ts.EOF = false
	ts.TokenIndex = -1
	ts.TokenType = 0
	ts.TokenStart = ts.FirstCharOffset
	ts.TokenEnd = ts.FirstCharOffset
}

// SetSource tokenizes source with tokenize (nil tokenizes nothing) and moves the cursor to the first token.
func (ts *TokenStream) SetSource(source string, tokenize func(string, func(int, int, int))) {
	src := js.UTF16(source)
	sourceLength := len(src)
	offsetAndType := adoptBuffer(ts.OffsetAndType, sourceLength+1)
	balance := adoptBuffer(ts.Balance, sourceLength+1)
	tokenCount := 0
	firstCharOffset := -1
	balanceCloseType := 0
	balanceStart := sourceLength

	ts.OffsetAndType = nil
	ts.Balance = nil
	clear(balance)

	if tokenize != nil {
		tokenize(source, func(typ, start, end int) {
			index := tokenCount
			tokenCount++
			offsetAndType[index] = uint32(typ<<typeShift | end)
			if firstCharOffset == -1 {
				firstCharOffset = start
			}
			balance[index] = uint32(balanceStart)
			if typ == balanceCloseType {
				prevBalanceStart := int(balance[balanceStart])
				balance[balanceStart] = uint32(index)
				balanceStart = prevBalanceStart
				balanceCloseType = balancePair[offsetAndType[prevBalanceStart]>>typeShift]
			} else if ts.IsBlockOpenerTokenType(typ) {
				balanceStart = index
				balanceCloseType = balancePair[typ]
			}
		})
	}

	offsetAndType[tokenCount] = uint32(EOF<<typeShift | sourceLength)
	balance[tokenCount] = uint32(tokenCount)

	for i := 0; i < tokenCount; i++ {
		start := int(balance[i])
		if start <= i {
			end := balance[start]
			if int(end) != i {
				balance[i] = end
			}
		} else if start > tokenCount {
			balance[i] = uint32(tokenCount)
		}
	}

	ts.Source = source
	ts.Src = src
	ts.FirstCharOffset = firstCharOffset
	if firstCharOffset == -1 {
		ts.FirstCharOffset = 0
	}
	ts.TokenCount = tokenCount
	ts.OffsetAndType = offsetAndType
	ts.Balance = balance

	ts.Reset()
	ts.Next()
}

// LookupType returns the type of the token offset positions after the current one, or EOF.
func (ts *TokenStream) LookupType(offset int) int {
	offset += ts.TokenIndex
	if offset < ts.TokenCount {
		return int(ts.item(offset) >> typeShift)
	}
	return EOF
}

// LookupTypeNonSC returns the type of the idx-th token from the cursor that is not white space or a comment.
func (ts *TokenStream) LookupTypeNonSC(idx int) int {
	for offset := ts.TokenIndex; offset < ts.TokenCount; offset++ {
		tokenType := int(ts.item(offset) >> typeShift)
		if tokenType != WhiteSpace && tokenType != Comment {
			if idx == 0 {
				return tokenType
			}
			idx--
		}
	}
	return EOF
}

// LookupOffset returns the start offset of the token offset positions after the current one.
func (ts *TokenStream) LookupOffset(offset int) int {
	offset += ts.TokenIndex
	if offset < ts.TokenCount {
		return int(ts.item(offset-1) & offsetMask)
	}
	return len(ts.Src)
}

// LookupOffsetNonSC returns the distance from the cursor to the idx-th token that is not white space or a
// comment, or EOF (0).
func (ts *TokenStream) LookupOffsetNonSC(idx int) int {
	for offset := ts.TokenIndex; offset < ts.TokenCount; offset++ {
		tokenType := int(ts.item(offset) >> typeShift)
		if tokenType != WhiteSpace && tokenType != Comment {
			if idx == 0 {
				return offset - ts.TokenIndex
			}
			idx--
		}
	}
	return EOF
}

// LookupValue compares the text of the token offset positions after the current one with a lowercase reference.
func (ts *TokenStream) LookupValue(offset int, referenceStr string) bool {
	offset += ts.TokenIndex
	if offset < ts.TokenCount {
		return CmpStr(ts.Src, int(ts.item(offset-1)&offsetMask), int(ts.item(offset)&offsetMask), referenceStr)
	}
	return false
}

// GetTokenStart returns the start offset of a token.
func (ts *TokenStream) GetTokenStart(tokenIndex int) int {
	if tokenIndex == ts.TokenIndex {
		return ts.TokenStart
	}
	if tokenIndex > 0 {
		if tokenIndex < ts.TokenCount {
			return int(ts.OffsetAndType[tokenIndex-1] & offsetMask)
		}
		return int(ts.OffsetAndType[ts.TokenCount] & offsetMask)
	}
	return ts.FirstCharOffset
}

// GetTokenEnd returns the end offset of a token.
func (ts *TokenStream) GetTokenEnd(tokenIndex int) int {
	if tokenIndex == ts.TokenIndex {
		return ts.TokenEnd
	}
	return int(ts.OffsetAndType[boundIndex(tokenIndex, 0, ts.TokenCount)] & offsetMask)
}

// GetTokenType returns the type of a token.
func (ts *TokenStream) GetTokenType(tokenIndex int) int {
	if tokenIndex == ts.TokenIndex {
		return ts.TokenType
	}
	return int(ts.OffsetAndType[boundIndex(tokenIndex, 0, ts.TokenCount)] >> typeShift)
}

// SubstrToCursor returns the source text from start to the current token's start.
func (ts *TokenStream) SubstrToCursor(start int) string {
	return ts.Substring(start, ts.TokenStart)
}

// Substring is source.substring(start, end) on the stream's source.
func (ts *TokenStream) Substring(start, end int) string {
	start, end = boundIndex(start, 0, len(ts.Src)), boundIndex(end, 0, len(ts.Src))
	if start > end {
		start, end = end, start
	}
	return js.FromUTF16(ts.Src[start:end])
}

// CharCodeAt returns the source code unit at offset, or -1 outside the source.
func (ts *TokenStream) CharCodeAt(offset int) int {
	return at(ts.Src, offset)
}

// IsBlockOpenerTokenType reports a function, (, [ or { token type.
func (ts *TokenStream) IsBlockOpenerTokenType(tokenType int) bool {
	return tokenType >= 0 && tokenType < 32 && blockTokens[tokenType] == blockOpenToken
}

// IsBlockCloserTokenType reports a ), ] or } token type.
func (ts *TokenStream) IsBlockCloserTokenType(tokenType int) bool {
	return tokenType >= 0 && tokenType < 32 && blockTokens[tokenType] == blockCloseToken
}
