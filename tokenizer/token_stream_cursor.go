package tokenizer

import "github.com/csstree/csstree/js"

// GetBlockTokenPairIndex returns the index of the token that closes (or opens) the block of a block token, or -1.
func (ts *TokenStream) GetBlockTokenPairIndex(tokenIndex int) int {
	typ := ts.GetTokenType(tokenIndex)
	switch blockTokens[typ] {
	case blockOpenToken:
		pairIndex := int(ts.Balance[tokenIndex])
		if balancePair[typ] == ts.GetTokenType(pairIndex) {
			return pairIndex
		}
	case blockCloseToken:
		pairIndex := int(ts.Balance[tokenIndex])
		if balancePair[ts.GetTokenType(pairIndex)] == typ {
			return pairIndex
		}
	}
	return -1
}

// IsBalanceEdge reports whether the current token's block ends before tokenIndex.
func (ts *TokenStream) IsBalanceEdge(tokenIndex int) bool {
	return int(ts.Balance[ts.TokenIndex]) < tokenIndex
}

// IsDelim reports whether a token is the delim code: the current token when offset is 0, else the token offset
// positions ahead.
func (ts *TokenStream) IsDelim(code, offset int) bool {
	if offset != 0 {
		return ts.LookupType(offset) == Delim && at(ts.Src, ts.LookupOffset(offset)) == code
	}
	return ts.TokenType == Delim && at(ts.Src, ts.TokenStart) == code
}

// item reads OffsetAndType like a JavaScript typed array: an index outside the buffer reads as 0.
func (ts *TokenStream) item(i int) uint32 {
	if i >= 0 && i < len(ts.OffsetAndType) {
		return ts.OffsetAndType[i]
	}
	return 0
}

// Skip moves the cursor tokenCount tokens forward.
func (ts *TokenStream) Skip(tokenCount int) {
	next := ts.TokenIndex + tokenCount
	if next < ts.TokenCount {
		ts.TokenIndex = next
		ts.TokenStart = int(ts.item(next-1) & offsetMask)
		item := ts.item(next)
		ts.TokenType = int(item >> typeShift)
		ts.TokenEnd = int(item & offsetMask)
	} else {
		ts.TokenIndex = ts.TokenCount
		ts.Next()
	}
}

// Next moves the cursor to the next token, or to EOF.
func (ts *TokenStream) Next() {
	next := ts.TokenIndex + 1
	if next < ts.TokenCount {
		ts.TokenIndex = next
		ts.TokenStart = ts.TokenEnd
		item := ts.OffsetAndType[next]
		ts.TokenType = int(item >> typeShift)
		ts.TokenEnd = int(item & offsetMask)
	} else {
		ts.EOF = true
		ts.TokenIndex = ts.TokenCount
		ts.TokenType = EOF
		ts.TokenStart = len(ts.Src)
		ts.TokenEnd = len(ts.Src)
	}
}

// SkipSC skips white space and comment tokens.
func (ts *TokenStream) SkipSC() {
	for ts.TokenType == WhiteSpace || ts.TokenType == Comment {
		ts.Next()
	}
}

// SkipUntilBalanced moves the cursor from startToken to the first token where stopConsume, given the code unit
// at the token's start, returns 1 (stop before it) or 2 (stop after it), skipping nested blocks, without leaving
// the enclosing block.
func (ts *TokenStream) SkipUntilBalanced(startToken int, stopConsume func(code int) int) {
	cursor := startToken
loop:
	for ; cursor < ts.TokenCount; cursor++ {
		balanceEnd := int(ts.Balance[cursor])
		if balanceEnd < startToken {
			break loop
		}
		offset := ts.FirstCharOffset
		if cursor > 0 {
			offset = int(ts.OffsetAndType[cursor-1] & offsetMask)
		}
		switch stopConsume(at(ts.Src, offset)) {
		case 1:
			break loop
		case 2:
			cursor++
			break loop
		default:
			if ts.IsBlockOpenerTokenType(int(ts.OffsetAndType[cursor] >> typeShift)) {
				cursor = balanceEnd
			}
		}
	}
	ts.Skip(cursor - ts.TokenIndex)
}

// ForEachToken calls fn with the type, start, end and index of every token.
func (ts *TokenStream) ForEachToken(fn func(typ, start, end, index int)) {
	offset := ts.FirstCharOffset
	for i := 0; i < ts.TokenCount; i++ {
		start := offset
		item := ts.OffsetAndType[i]
		end := int(item & offsetMask)
		offset = end
		fn(int(item>>typeShift), start, end, i)
	}
}

// Dump returns every token as {idx, type, chunk, balance}.
func (ts *TokenStream) Dump() []any {
	tokens := make([]any, ts.TokenCount)
	ts.ForEachToken(func(typ, start, end, index int) {
		tokens[index] = js.NewObject(
			"idx", index,
			"type", TokenNames[typ],
			"chunk", ts.Substring(start, end),
			"balance", int(ts.Balance[index]),
		)
	})
	return tokens
}
