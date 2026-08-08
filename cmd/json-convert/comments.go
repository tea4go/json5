package main

import "strings"

func cleanComment(raw []byte) string {
	text := string(raw)
	switch {
	case strings.HasPrefix(text, "//"):
		return strings.TrimSpace(text[2:])
	case strings.HasPrefix(text, "/*") && strings.HasSuffix(text, "*/"):
		text = text[2 : len(text)-2]
	}

	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return ""
	}

	indent := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		n := len(line) - len(strings.TrimLeft(line, " \t"))
		if indent < 0 || n < indent {
			indent = n
		}
	}
	if indent > 0 {
		for i := range lines {
			if len(lines[i]) >= indent {
				lines[i] = lines[i][indent:]
			}
		}
	}
	for i := range lines {
		line := strings.TrimLeft(lines[i], " \t")
		if strings.HasPrefix(line, "*") {
			line = line[1:]
			if strings.HasPrefix(line, " ") {
				line = line[1:]
			}
			lines[i] = line
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func addHintMembers(v value) value {
	switch v.kind {
	case kindArray:
		for i := range v.array {
			v.array[i] = addHintMembers(v.array[i])
		}
	case kindObject:
		members := make([]member, 0, len(v.object)*2)
		for _, m := range v.object {
			m.value = addHintMembers(m.value)
			var hints []string
			for _, c := range append(append([]comment(nil), m.leadingComments...), m.trailingComments...) {
				if hint := cleanComment(c.raw); hint != "" {
					hints = append(hints, hint)
				}
			}
			if len(hints) > 0 {
				text := []byte(strings.Join(hints, "\n"))
				members = append(members, member{
					key:    m.key + "_hint",
					keyPos: m.keyPos,
					value:  value{kind: kindString, text: text, pos: m.keyPos},
				})
			}
			members = append(members, m)
		}
		v.object = members
	}
	return v
}
