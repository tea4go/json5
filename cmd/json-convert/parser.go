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
	raw       string
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
	text   string
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
	return fmt.Sprintf("line %d column %d: %s", e.pos.line, e.pos.column, e.message)
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
	p.skipWhitespace()
	v, err := p.parseValue()
	if err != nil {
		return value{}, err
	}
	p.skipWhitespace()
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
	if b == '\n' {
		p.line++
		p.col = 1
	} else {
		p.col++
	}
	return b
}

func (p *parser) skipWhitespace() {
	for p.off < len(p.data) {
		switch p.data[p.off] {
		case ' ', '\t', '\r', '\n':
			p.takeByte()
		default:
			return
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
		return p.parseLiteral("null", kindNull, "", start)
	case 't':
		return p.parseLiteral("true", kindBool, "true", start)
	case 'f':
		return p.parseLiteral("false", kindBool, "false", start)
	case '"':
		text, err := p.parseJSONString()
		if err != nil {
			return value{}, err
		}
		return value{kind: kindString, text: text, pos: start, end: p.position()}, nil
	case '[':
		return p.parseArray(start)
	case '{':
		return p.parseObject(start)
	case '-':
		return p.parseJSONNumber(start)
	default:
		if p.data[p.off] >= '0' && p.data[p.off] <= '9' {
			return p.parseJSONNumber(start)
		}
		return value{}, p.errorf("expected value")
	}
}

func (p *parser) parseLiteral(token string, kind valueKind, text string, start position) (value, error) {
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
	p.skipWhitespace()
	members := []member{}
	if p.off < len(p.data) && p.data[p.off] == '}' {
		p.takeByte()
		return value{kind: kindObject, pos: start, end: p.position(), object: members}, nil
	}
	for {
		if p.off >= len(p.data) || p.data[p.off] != '"' {
			return value{}, p.errorf("expected quoted object key")
		}
		keyPos := p.position()
		key, err := p.parseJSONString()
		if err != nil {
			return value{}, err
		}
		p.skipWhitespace()
		if p.off >= len(p.data) || p.data[p.off] != ':' {
			return value{}, p.errorf("expected ':' after object key")
		}
		p.takeByte()
		p.skipWhitespace()
		item, err := p.parseValue()
		if err != nil {
			return value{}, err
		}
		members = append(members, member{key: key, value: item, keyPos: keyPos, endLine: item.end.line})
		p.skipWhitespace()
		if p.off >= len(p.data) {
			return value{}, p.errorf("expected ',' or '}'")
		}
		switch p.data[p.off] {
		case '}':
			p.takeByte()
			return value{kind: kindObject, pos: start, end: p.position(), object: members}, nil
		case ',':
			p.takeByte()
			p.skipWhitespace()
			if p.off < len(p.data) && p.data[p.off] == '}' {
				return value{}, p.errorf("trailing comma is not allowed")
			}
		default:
			return value{}, p.errorf("expected ',' or '}'")
		}
	}
}

func (p *parser) parseArray(start position) (value, error) {
	p.takeByte()
	p.skipWhitespace()
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
		p.skipWhitespace()
		if p.off >= len(p.data) {
			return value{}, p.errorf("expected ',' or ']'")
		}
		switch p.data[p.off] {
		case ']':
			p.takeByte()
			return value{kind: kindArray, pos: start, end: p.position(), array: items}, nil
		case ',':
			p.takeByte()
			p.skipWhitespace()
			if p.off < len(p.data) && p.data[p.off] == ']' {
				return value{}, p.errorf("trailing comma is not allowed")
			}
		default:
			return value{}, p.errorf("expected ',' or ']'")
		}
	}
}

func (p *parser) parseJSONString() (string, error) {
	p.takeByte()
	var out strings.Builder
	for p.off < len(p.data) {
		b := p.data[p.off]
		if b == '"' {
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
			case '"', '\\', '/':
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
	text := string(p.data[tokenStart:p.off])
	return value{kind: kindNumber, text: text, pos: start, end: p.position()}, nil
}
