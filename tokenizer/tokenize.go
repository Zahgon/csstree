package tokenizer

import "github.com/csstree/csstree/js"

// Tokenize splits source into tokens and calls onToken with each token's type and its start and end offsets in
// UTF-16 code units. A leading byte order mark is skipped.
func Tokenize(source string, onToken func(typ, start, end int)) {
	TokenizeUnits(js.UTF16(source), onToken)
}

// TokenizeUnits is Tokenize on source already split into UTF-16 code units.
func TokenizeUnits(src []uint16, onToken func(typ, start, end int)) {
	t := &tokenizerState{src: src}
	t.run(onToken)
}

type tokenizerState struct {
	src    []uint16
	offset int
	typ    int
}

func (t *tokenizerState) code(offset int) int {
	return charCode(t.src, offset)
}

// consumeNumericToken is § 4.3.3. Consume a numeric token.
func (t *tokenizerState) consumeNumericToken() {
	t.offset = ConsumeNumber(t.src, t.offset)
	if IsIdentifierStart(t.code(t.offset), t.code(t.offset+1), t.code(t.offset+2)) {
		t.typ = Dimension
		t.offset = ConsumeName(t.src, t.offset)
		return
	}
	if t.code(t.offset) == 0x0025 {
		t.typ = Percentage
		t.offset++
		return
	}
	t.typ = Number
}

// consumeIdentLikeToken is § 4.3.4. Consume an ident-like token.
func (t *tokenizerState) consumeIdentLikeToken() {
	nameStartOffset := t.offset
	t.offset = ConsumeName(t.src, t.offset)
	if CmpStr(t.src, nameStartOffset, t.offset, "url") && t.code(t.offset) == 0x0028 {
		t.offset = FindWhiteSpaceEnd(t.src, t.offset+1)
		if t.code(t.offset) == 0x0022 || t.code(t.offset) == 0x0027 {
			t.typ = Function
			t.offset = nameStartOffset + 4
			return
		}
		t.consumeURLToken()
		return
	}
	if t.code(t.offset) == 0x0028 {
		t.typ = Function
		t.offset++
		return
	}
	t.typ = Ident
}

// consumeStringToken is § 4.3.5. Consume a string token; the opening quote is at the current offset.
func (t *tokenizerState) consumeStringToken() {
	endingCodePoint := t.code(t.offset)
	t.offset++
	t.typ = String
	for ; t.offset < len(t.src); t.offset++ {
		code := int(t.src[t.offset])
		switch CharCodeCategory(code) {
		case endingCodePoint:
			t.offset++
			return
		case WhiteSpaceCategory:
			if IsNewline(code) {
				t.offset += GetNewlineLength(t.src, t.offset, code)
				t.typ = BadString
				return
			}
		case 0x005C:
			if t.offset == len(t.src)-1 {
				break
			}
			nextCode := t.code(t.offset + 1)
			if IsNewline(nextCode) {
				t.offset += GetNewlineLength(t.src, t.offset+1, nextCode)
			} else if IsValidEscape(code, nextCode) {
				t.offset = ConsumeEscaped(t.src, t.offset) - 1
			}
		}
	}
}

// consumeURLToken is § 4.3.6. Consume a url token.
func (t *tokenizerState) consumeURLToken() {
	t.typ = URL
	t.offset = FindWhiteSpaceEnd(t.src, t.offset)
	for ; t.offset < len(t.src); t.offset++ {
		code := int(t.src[t.offset])
		switch CharCodeCategory(code) {
		case 0x0029:
			t.offset++
			return
		case WhiteSpaceCategory:
			t.offset = FindWhiteSpaceEnd(t.src, t.offset)
			if t.code(t.offset) == 0x0029 || t.offset >= len(t.src) {
				if t.offset < len(t.src) {
					t.offset++
				}
				return
			}
			t.offset = ConsumeBadURLRemnants(t.src, t.offset)
			t.typ = BadURL
			return
		case 0x0022, 0x0027, 0x0028, NonPrintableCategory:
			t.offset = ConsumeBadURLRemnants(t.src, t.offset)
			t.typ = BadURL
			return
		case 0x005C:
			if IsValidEscape(code, t.code(t.offset+1)) {
				t.offset = ConsumeEscaped(t.src, t.offset) - 1
				break
			}
			t.offset = ConsumeBadURLRemnants(t.src, t.offset)
			t.typ = BadURL
			return
		}
	}
}

func (t *tokenizerState) delim() {
	t.typ = Delim
	t.offset++
}

func (t *tokenizerState) run(onToken func(typ, start, end int)) {
	start := IsBOM(t.code(0))
	t.offset = start
	for t.offset < len(t.src) {
		code := int(t.src[t.offset])
		switch CharCodeCategory(code) {
		case WhiteSpaceCategory:
			t.typ = WhiteSpace
			t.offset = FindWhiteSpaceEnd(t.src, t.offset+1)
		case 0x0022, 0x0027:
			t.consumeStringToken()
		case 0x0023:
			if IsName(t.code(t.offset+1)) || IsValidEscape(t.code(t.offset+1), t.code(t.offset+2)) {
				t.typ = Hash
				t.offset = ConsumeName(t.src, t.offset+1)
			} else {
				t.delim()
			}
		case 0x0028:
			t.typ = LeftParenthesis
			t.offset++
		case 0x0029:
			t.typ = RightParenthesis
			t.offset++
		case 0x002B, 0x002E:
			if IsNumberStart(code, t.code(t.offset+1), t.code(t.offset+2)) != 0 {
				t.consumeNumericToken()
			} else {
				t.delim()
			}
		case 0x002C:
			t.typ = Comma
			t.offset++
		case 0x002D:
			switch {
			case IsNumberStart(code, t.code(t.offset+1), t.code(t.offset+2)) != 0:
				t.consumeNumericToken()
			case t.code(t.offset+1) == 0x002D && t.code(t.offset+2) == 0x003E:
				t.typ = CDC
				t.offset += 3
			case IsIdentifierStart(code, t.code(t.offset+1), t.code(t.offset+2)):
				t.consumeIdentLikeToken()
			default:
				t.delim()
			}
		case 0x002F:
			if t.code(t.offset+1) == 0x002A {
				t.typ = Comment
				t.offset = indexOfCommentEnd(t.src, t.offset+2)
			} else {
				t.delim()
			}
		case 0x003A:
			t.typ = Colon
			t.offset++
		case 0x003B:
			t.typ = Semicolon
			t.offset++
		case 0x003C:
			if t.code(t.offset+1) == 0x0021 && t.code(t.offset+2) == 0x002D && t.code(t.offset+3) == 0x002D {
				t.typ = CDO
				t.offset += 4
			} else {
				t.delim()
			}
		case 0x0040:
			if IsIdentifierStart(t.code(t.offset+1), t.code(t.offset+2), t.code(t.offset+3)) {
				t.typ = AtKeyword
				t.offset = ConsumeName(t.src, t.offset+1)
			} else {
				t.delim()
			}
		case 0x005B:
			t.typ = LeftSquareBracket
			t.offset++
		case 0x005C:
			if IsValidEscape(code, t.code(t.offset+1)) {
				t.consumeIdentLikeToken()
			} else {
				t.delim()
			}
		case 0x005D:
			t.typ = RightSquareBracket
			t.offset++
		case 0x007B:
			t.typ = LeftCurlyBracket
			t.offset++
		case 0x007D:
			t.typ = RightCurlyBracket
			t.offset++
		case DigitCategory:
			t.consumeNumericToken()
		case NameStartCategory:
			t.consumeIdentLikeToken()
		default:
			t.delim()
		}
		onToken(t.typ, start, t.offset)
		start = t.offset
	}
}

// indexOfCommentEnd returns the offset after the first "*/" at or after from, or the source length.
func indexOfCommentEnd(src []uint16, from int) int {
	for i := from; i+1 < len(src); i++ {
		if src[i] == '*' && src[i+1] == '/' {
			return i + 2
		}
	}
	return len(src)
}
