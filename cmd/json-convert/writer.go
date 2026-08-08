package main

import (
	"bytes"
	"fmt"
	"unicode/utf8"
)

type outputMode uint8

const (
	outputJSON outputMode = iota
	outputJSON5
)

type writer struct {
	bytes.Buffer
	mode   outputMode
	indent int
	depth  int
}

func writeDocument(value value, mode outputMode, indent int) ([]byte, error) {
	w := writer{mode: mode, indent: indent}
	if err := w.writeValue(value); err != nil {
		return nil, err
	}
	w.WriteByte('\n')
	return append([]byte(nil), w.Bytes()...), nil
}

func (w *writer) writeValue(value value) error {
	switch value.kind {
	case kindNull:
		w.WriteString("null")
	case kindBool, kindNumber:
		w.Write(value.text)
	case kindString:
		return w.writeString(value)
	case kindArray:
		return w.writeArray(value.array)
	case kindObject:
		return w.writeObject(value.object)
	default:
		return fmt.Errorf("line %d, column %d: invalid value kind", value.pos.line, value.pos.column)
	}
	return nil
}

func (w *writer) writeString(value value) error {
	if w.mode == outputJSON5 && !bytes.ContainsRune(value.text, '`') {
		w.WriteByte('`')
		w.Write(value.text)
		w.WriteByte('`')
		return nil
	}
	if !utf8.Valid(value.text) {
		return fmt.Errorf("line %d, column %d: invalid UTF-8 in string", value.pos.line, value.pos.column)
	}
	w.writeJSONString(string(value.text))
	return nil
}

func (w *writer) writeJSONString(text string) {
	w.WriteByte('"')
	for _, r := range text {
		switch r {
		case '"', '\\':
			w.WriteByte('\\')
			w.WriteRune(r)
		case '\b':
			w.WriteString(`\b`)
		case '\f':
			w.WriteString(`\f`)
		case '\n':
			w.WriteString(`\n`)
		case '\r':
			w.WriteString(`\r`)
		case '\t':
			w.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&w.Buffer, `\u%04x`, r)
			} else {
				w.WriteRune(r)
			}
		}
	}
	w.WriteByte('"')
}

func (w *writer) writeObject(members []member) error {
	w.WriteByte('{')
	if len(members) == 0 {
		w.WriteByte('}')
		return nil
	}
	w.depth++
	for i, member := range members {
		w.writeSeparator(i)
		w.writeJSONString(member.key)
		w.WriteByte(':')
		if w.indent > 0 {
			w.WriteByte(' ')
		}
		if err := w.writeValue(member.value); err != nil {
			return err
		}
	}
	w.depth--
	w.writeClosingIndent()
	w.WriteByte('}')
	return nil
}

func (w *writer) writeArray(values []value) error {
	w.WriteByte('[')
	if len(values) == 0 {
		w.WriteByte(']')
		return nil
	}
	w.depth++
	for i := range values {
		w.writeSeparator(i)
		if err := w.writeValue(values[i]); err != nil {
			return err
		}
	}
	w.depth--
	w.writeClosingIndent()
	w.WriteByte(']')
	return nil
}

func (w *writer) writeSeparator(index int) {
	if index > 0 {
		w.WriteByte(',')
	}
	if w.indent > 0 {
		w.WriteByte('\n')
		w.writeIndent()
	}
}

func (w *writer) writeClosingIndent() {
	if w.indent > 0 {
		w.WriteByte('\n')
		w.writeIndent()
	}
}

func (w *writer) writeIndent() {
	for i := 0; i < w.depth*w.indent; i++ {
		w.WriteByte(' ')
	}
}
