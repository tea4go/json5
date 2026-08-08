package main

import (
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

type parseMode uint8

const (
	modeJSON parseMode = iota
	modeJSON5
)

type valueKind uint8

const (
	kindNull valueKind = iota
	kindBool
	kindNumber
	kindString
	kindArray
	kindObject
)

type position struct {
	offset int
	line   int
	column int
}

type comment struct {
	raw       []byte
	start     position
	end       position
	startLine int
	endLine   int
}

type member struct {
	key              string
	value            value
	keyPos           position
	endLine          int
	leadingComments  []comment
	trailingComments []comment
}

type value struct {
	kind   valueKind
	text   []byte
	pos    position
	end    position
	array  []value
	object []member
}

type parseError struct {
	pos     position
	message string
}

func (e *parseError) Error() string {
	return fmt.Sprintf("line %d, column %d: %s", e.pos.line, e.pos.column, e.message)
}

type parser struct {
	data []byte
	mode parseMode
	off  int
	line int
	col  int
}

func parseDocument(data []byte, mode parseMode) (value, error) {
	p := parser{data: data, mode: mode, line: 1, col: 1}
	if _, err := p.skipSpaceAndComments(); err != nil {
		return value{}, err
	}
	v, err := p.parseValue()
	if err != nil {
		return value{}, err
	}
	if _, err := p.skipSpaceAndComments(); err != nil {
		return value{}, err
	}
	if p.off != len(p.data) {
		return value{}, p.errorf("unexpected trailing content")
	}
	return v, nil
}

func (p *parser) position() position {
	return position{offset: p.off, line: p.line, column: p.col}
}

func (p *parser) errorf(format string, args ...any) error {
	return &parseError{pos: p.position(), message: fmt.Sprintf(format, args...)}
}

func (p *parser) takeByte() byte {
	b := p.data[p.off]
	p.off++
	switch b {
	case '\r':
		p.line++
		p.col = 1
	case '\n':
		if p.off < 2 || p.data[p.off-2] != '\r' {
			p.line++
			p.col = 1
		}
	default:
		p.col++
	}
	return b
}

func (p *parser) skipWhitespace() {
	for p.off < len(p.data) {
		switch p.data[p.off] {
		case ' ', '\t', '\r', '\n':
			p.takeByte()
		case '\f':
			if p.mode != modeJSON5 {
				return
			}
			p.takeByte()
		default:
			return
		}
	}
}

func (p *parser) skipSpaceAndComments() ([]comment, error) {
	var comments []comment
	for {
		p.skipWhitespace()
		if p.mode != modeJSON5 || p.off+1 >= len(p.data) || p.data[p.off] != '/' {
			return comments, nil
		}
		start := p.position()
		switch p.data[p.off+1] {
		case '/':
			begin := p.off
			p.takeByte()
			p.takeByte()
			for p.off < len(p.data) && p.data[p.off] != '\r' && p.data[p.off] != '\n' {
				p.takeByte()
			}
			end := p.position()
			comments = append(comments, comment{raw: append([]byte(nil), p.data[begin:p.off]...), start: start, end: end, startLine: start.line, endLine: end.line})
		case '*':
			begin := p.off
			p.takeByte()
			p.takeByte()
			for p.off+1 < len(p.data) && (p.data[p.off] != '*' || p.data[p.off+1] != '/') {
				p.takeByte()
			}
			if p.off+1 >= len(p.data) {
				for p.off < len(p.data) {
					p.takeByte()
				}
				return nil, p.errorf("unterminated block comment")
			}
			p.takeByte()
			p.takeByte()
			end := p.position()
			comments = append(comments, comment{raw: append([]byte(nil), p.data[begin:p.off]...), start: start, end: end, startLine: start.line, endLine: end.line})
		default:
			return comments, nil
		}
	}
}

func (p *parser) parseValue() (value, error) {
	if p.off >= len(p.data) {
		return value{}, p.errorf("expected value")
	}
	start := p.position()
	switch p.data[p.off] {
	case 'n':
		return p.parseLiteral("null", kindNull, nil, start)
	case 't':
		return p.parseLiteral("true", kindBool, []byte("true"), start)
	case 'f':
		return p.parseLiteral("false", kindBool, []byte("false"), start)
	case '"':
		text, err := p.parseQuotedString('"')
		if err != nil {
			return value{}, err
		}
		return value{kind: kindString, text: []byte(text), pos: start, end: p.position()}, nil
	case '\'', '`':
		if p.mode != modeJSON5 {
			return value{}, p.errorf("expected value")
		}
		var text []byte
		var err error
		if p.data[p.off] == '`' {
			text, err = p.parseRawString()
		} else {
			var decoded string
			decoded, err = p.parseQuotedString('\'')
			text = []byte(decoded)
		}
		if err != nil {
			return value{}, err
		}
		return value{kind: kindString, text: text, pos: start, end: p.position()}, nil
	case '[':
		return p.parseArray(start)
	case '{':
		return p.parseObject(start)
	case '-', '+', '.':
		if p.mode == modeJSON5 {
			return p.parseJSON5Number(start)
		}
		if p.data[p.off] == '-' {
			return p.parseJSONNumber(start)
		}
		return value{}, p.errorf("expected value")
	default:
		if p.data[p.off] >= '0' && p.data[p.off] <= '9' {
			if p.mode == modeJSON5 {
				return p.parseJSON5Number(start)
			}
			return p.parseJSONNumber(start)
		}
		if p.mode == modeJSON5 && (p.hasToken("Infinity") || p.hasToken("NaN")) {
			return p.parseJSON5Number(start)
		}
		return value{}, p.errorf("expected value")
	}
}

func (p *parser) parseLiteral(token string, kind valueKind, text []byte, start position) (value, error) {
	if len(p.data)-p.off < len(token) || string(p.data[p.off:p.off+len(token)]) != token {
		return value{}, p.errorf("invalid literal")
	}
	for range token {
		p.takeByte()
	}
	return value{kind: kind, text: text, pos: start, end: p.position()}, nil
}

func (p *parser) parseObject(start position) (value, error) {
	p.takeByte()
	leading, err := p.skipSpaceAndComments()
	if err != nil {
		return value{}, err
	}
	members := []member{}
	if p.off < len(p.data) && p.data[p.off] == '}' {
		p.takeByte()
		return value{kind: kindObject, pos: start, end: p.position(), object: members}, nil
	}
	for {
		keyPos := p.position()
		var key string
		switch {
		case p.off < len(p.data) && p.data[p.off] == '"':
			key, err = p.parseQuotedString('"')
		case p.mode == modeJSON5 && p.off < len(p.data) && p.data[p.off] == '\'':
			key, err = p.parseQuotedString('\'')
		case p.mode == modeJSON5 && p.off < len(p.data) && isIdentifierStart(p.data[p.off]):
			key = p.parseIdentifier()
		default:
			if p.mode == modeJSON {
				return value{}, p.errorf("expected quoted object key")
			}
			return value{}, p.errorf("expected object key")
		}
		if err != nil {
			return value{}, err
		}
		if _, err = p.skipSpaceAndComments(); err != nil {
			return value{}, err
		}
		if p.off >= len(p.data) || p.data[p.off] != ':' {
			return value{}, p.errorf("expected ':' after object key")
		}
		p.takeByte()
		if _, err = p.skipSpaceAndComments(); err != nil {
			return value{}, err
		}
		item, err := p.parseValue()
		if err != nil {
			return value{}, err
		}
		m := member{key: key, value: item, keyPos: keyPos, endLine: item.end.line, leadingComments: leading}
		comments, err := p.skipSpaceAndComments()
		if err != nil {
			return value{}, err
		}
		if p.off >= len(p.data) {
			return value{}, p.errorf("expected ',' or '}'")
		}
		switch p.data[p.off] {
		case '}':
			m.trailingComments = comments
			members = append(members, m)
			p.takeByte()
			return value{kind: kindObject, pos: start, end: p.position(), object: members}, nil
		case ',':
			commaLine := p.line
			p.takeByte()
			afterComma, err := p.skipSpaceAndComments()
			if err != nil {
				return value{}, err
			}
			if p.off < len(p.data) && p.data[p.off] == '}' {
				if p.mode != modeJSON5 {
					return value{}, p.errorf("trailing comma is not allowed")
				}
				m.trailingComments = append(comments, afterComma...)
				members = append(members, m)
				p.takeByte()
				return value{kind: kindObject, pos: start, end: p.position(), object: members}, nil
			}
			var nextLeading []comment
			for _, c := range comments {
				if c.startLine == m.endLine {
					m.trailingComments = append(m.trailingComments, c)
				} else {
					nextLeading = append(nextLeading, c)
				}
			}
			m.endLine = commaLine
			for _, c := range afterComma {
				if c.startLine == commaLine {
					m.trailingComments = append(m.trailingComments, c)
				} else {
					nextLeading = append(nextLeading, c)
				}
			}
			members = append(members, m)
			leading = nextLeading
		default:
			return value{}, p.errorf("expected ',' or '}'")
		}
	}
}

func isIdentifierStart(b byte) bool {
	return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b == '_' || b == '$'
}

func isIdentifierPart(b byte) bool {
	return isIdentifierStart(b) || b >= '0' && b <= '9'
}

func (p *parser) parseIdentifier() string {
	start := p.off
	for p.off < len(p.data) && isIdentifierPart(p.data[p.off]) {
		p.takeByte()
	}
	return string(p.data[start:p.off])
}

func (p *parser) parseArray(start position) (value, error) {
	p.takeByte()
	if _, err := p.skipSpaceAndComments(); err != nil {
		return value{}, err
	}
	items := []value{}
	if p.off < len(p.data) && p.data[p.off] == ']' {
		p.takeByte()
		return value{kind: kindArray, pos: start, end: p.position(), array: items}, nil
	}
	for {
		item, err := p.parseValue()
		if err != nil {
			return value{}, err
		}
		items = append(items, item)
		if _, err := p.skipSpaceAndComments(); err != nil {
			return value{}, err
		}
		if p.off >= len(p.data) {
			return value{}, p.errorf("expected ',' or ']'")
		}
		switch p.data[p.off] {
		case ']':
			p.takeByte()
			return value{kind: kindArray, pos: start, end: p.position(), array: items}, nil
		case ',':
			p.takeByte()
			if _, err := p.skipSpaceAndComments(); err != nil {
				return value{}, err
			}
			if p.off < len(p.data) && p.data[p.off] == ']' {
				if p.mode != modeJSON5 {
					return value{}, p.errorf("trailing comma is not allowed")
				}
				p.takeByte()
				return value{kind: kindArray, pos: start, end: p.position(), array: items}, nil
			}
		default:
			return value{}, p.errorf("expected ',' or ']'")
		}
	}
}

func (p *parser) parseQuotedString(quote byte) (string, error) {
	p.takeByte()
	var out strings.Builder
	for p.off < len(p.data) {
		b := p.data[p.off]
		if b == quote {
			p.takeByte()
			return out.String(), nil
		}
		if b < 0x20 {
			return "", p.errorf("unescaped control character in string")
		}
		if b == '\\' {
			p.takeByte()
			if p.off >= len(p.data) {
				return "", p.errorf("unterminated escape sequence")
			}
			escape := p.takeByte()
			switch escape {
			case '"', '\'', '\\', '/':
				if escape == '\'' && p.mode != modeJSON5 {
					return "", p.errorf("invalid escape sequence")
				}
				out.WriteByte(escape)
			case 'b':
				out.WriteByte('\b')
			case 'f':
				out.WriteByte('\f')
			case 'n':
				out.WriteByte('\n')
			case 'r':
				out.WriteByte('\r')
			case 't':
				out.WriteByte('\t')
			case 'u':
				r, err := p.parseUnicodeEscape()
				if err != nil {
					return "", err
				}
				out.WriteRune(r)
			case '\n':
				if p.mode != modeJSON5 {
					return "", p.errorf("invalid escape sequence")
				}
			case '\r':
				if p.mode != modeJSON5 {
					return "", p.errorf("invalid escape sequence")
				}
				if p.off < len(p.data) && p.data[p.off] == '\n' {
					p.takeByte()
				}
			default:
				return "", p.errorf("invalid escape sequence")
			}
			continue
		}
		if b < utf8.RuneSelf {
			out.WriteByte(p.takeByte())
			continue
		}
		r, size := utf8.DecodeRune(p.data[p.off:])
		if r == utf8.RuneError && size == 1 {
			return "", p.errorf("invalid UTF-8 in string")
		}
		out.Write(p.data[p.off : p.off+size])
		p.off += size
		p.col++
	}
	return "", p.errorf("unterminated string")
}

func (p *parser) parseRawString() ([]byte, error) {
	p.takeByte()
	start := p.off
	for p.off < len(p.data) {
		if p.data[p.off] == '`' {
			text := append([]byte(nil), p.data[start:p.off]...)
			p.takeByte()
			return text, nil
		}
		p.takeByte()
	}
	return nil, p.errorf("unterminated raw string")
}

func (p *parser) parseUnicodeEscape() (rune, error) {
	first, err := p.parseHex4()
	if err != nil {
		return 0, err
	}
	if first >= 0xdc00 && first <= 0xdfff {
		return 0, p.errorf("unpaired low surrogate")
	}
	if first < 0xd800 || first > 0xdbff {
		return rune(first), nil
	}
	if len(p.data)-p.off < 2 || p.data[p.off] != '\\' || p.data[p.off+1] != 'u' {
		return 0, p.errorf("unpaired high surrogate")
	}
	p.takeByte()
	p.takeByte()
	second, err := p.parseHex4()
	if err != nil {
		return 0, err
	}
	if second < 0xdc00 || second > 0xdfff {
		return 0, p.errorf("invalid surrogate pair")
	}
	return utf16.DecodeRune(rune(first), rune(second)), nil
}

func (p *parser) parseHex4() (uint16, error) {
	if len(p.data)-p.off < 4 {
		return 0, p.errorf("incomplete unicode escape")
	}
	var n uint16
	for i := 0; i < 4; i++ {
		b := p.data[p.off]
		var digit byte
		switch {
		case b >= '0' && b <= '9':
			digit = b - '0'
		case b >= 'a' && b <= 'f':
			digit = b - 'a' + 10
		case b >= 'A' && b <= 'F':
			digit = b - 'A' + 10
		default:
			return 0, p.errorf("invalid unicode escape")
		}
		n = n*16 + uint16(digit)
		p.takeByte()
	}
	return n, nil
}

func (p *parser) hasToken(token string) bool {
	return len(p.data)-p.off >= len(token) && string(p.data[p.off:p.off+len(token)]) == token
}

func (p *parser) parseJSON5Number(start position) (value, error) {
	tokenStart := p.off
	if p.off < len(p.data) && (p.data[p.off] == '+' || p.data[p.off] == '-') {
		p.takeByte()
	}
	if p.hasToken("Infinity") {
		for range "Infinity" {
			p.takeByte()
		}
		return p.finishJSON5Number(tokenStart, start)
	}
	if p.hasToken("NaN") && p.off == tokenStart {
		for range "NaN" {
			p.takeByte()
		}
		return p.finishJSON5Number(tokenStart, start)
	}
	if p.off+1 < len(p.data) && p.data[p.off] == '0' && (p.data[p.off+1] == 'x' || p.data[p.off+1] == 'X') {
		p.takeByte()
		p.takeByte()
		digits := p.off
		for p.off < len(p.data) && isHexDigit(p.data[p.off]) {
			p.takeByte()
		}
		if p.off == digits {
			return value{}, p.errorf("expected hexadecimal digit")
		}
		return p.finishJSON5Number(tokenStart, start)
	}

	digitsBefore := 0
	if p.off < len(p.data) && p.data[p.off] == '0' {
		p.takeByte()
		digitsBefore = 1
		if p.off < len(p.data) && p.data[p.off] >= '0' && p.data[p.off] <= '9' {
			return value{}, p.errorf("leading zero is not allowed")
		}
	} else {
		for p.off < len(p.data) && p.data[p.off] >= '0' && p.data[p.off] <= '9' {
			p.takeByte()
			digitsBefore++
		}
	}
	digitsAfter := 0
	if p.off < len(p.data) && p.data[p.off] == '.' {
		p.takeByte()
		for p.off < len(p.data) && p.data[p.off] >= '0' && p.data[p.off] <= '9' {
			p.takeByte()
			digitsAfter++
		}
	}
	if digitsBefore == 0 && digitsAfter == 0 {
		return value{}, p.errorf("expected number")
	}
	if p.off < len(p.data) && (p.data[p.off] == 'e' || p.data[p.off] == 'E') {
		p.takeByte()
		if p.off < len(p.data) && (p.data[p.off] == '+' || p.data[p.off] == '-') {
			p.takeByte()
		}
		digits := p.off
		for p.off < len(p.data) && p.data[p.off] >= '0' && p.data[p.off] <= '9' {
			p.takeByte()
		}
		if p.off == digits {
			return value{}, p.errorf("expected digit in exponent")
		}
	}
	return p.finishJSON5Number(tokenStart, start)
}

func isHexDigit(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F'
}

func (p *parser) finishJSON5Number(tokenStart int, start position) (value, error) {
	if p.off < len(p.data) && isIdentifierPart(p.data[p.off]) {
		return value{}, p.errorf("invalid number token")
	}
	text := append([]byte(nil), p.data[tokenStart:p.off]...)
	return value{kind: kindNumber, text: text, pos: start, end: p.position()}, nil
}

func (p *parser) parseJSONNumber(start position) (value, error) {
	tokenStart := p.off
	if p.data[p.off] == '-' {
		p.takeByte()
		if p.off >= len(p.data) {
			return value{}, p.errorf("expected digit after '-'")
		}
	}
	if p.data[p.off] == '0' {
		p.takeByte()
		if p.off < len(p.data) && p.data[p.off] >= '0' && p.data[p.off] <= '9' {
			return value{}, p.errorf("leading zero is not allowed")
		}
	} else if p.data[p.off] >= '1' && p.data[p.off] <= '9' {
		for p.off < len(p.data) && p.data[p.off] >= '0' && p.data[p.off] <= '9' {
			p.takeByte()
		}
	} else {
		return value{}, p.errorf("expected digit")
	}
	if p.off < len(p.data) && p.data[p.off] == '.' {
		p.takeByte()
		if p.off >= len(p.data) || p.data[p.off] < '0' || p.data[p.off] > '9' {
			return value{}, p.errorf("expected digit after decimal point")
		}
		for p.off < len(p.data) && p.data[p.off] >= '0' && p.data[p.off] <= '9' {
			p.takeByte()
		}
	}
	if p.off < len(p.data) && (p.data[p.off] == 'e' || p.data[p.off] == 'E') {
		p.takeByte()
		if p.off < len(p.data) && (p.data[p.off] == '+' || p.data[p.off] == '-') {
			p.takeByte()
		}
		if p.off >= len(p.data) || p.data[p.off] < '0' || p.data[p.off] > '9' {
			return value{}, p.errorf("expected digit in exponent")
		}
		for p.off < len(p.data) && p.data[p.off] >= '0' && p.data[p.off] <= '9' {
			p.takeByte()
		}
	}
	text := append([]byte(nil), p.data[tokenStart:p.off]...)
	return value{kind: kindNumber, text: text, pos: start, end: p.position()}, nil
}
