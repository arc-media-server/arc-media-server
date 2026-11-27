package dbsql

import "strings"

// sqlTokens keeps quoted strings, identifiers and comments intact while splitting SQL.
func sqlTokens(q string) []string {
	var out []string
	for i := 0; i < len(q); {
		start := i
		c := q[i]
		i++
		switch {
		case c == '\'' || c == '"' || c == '`':
			for i < len(q) {
				if q[i] == c {
					i++
					if i < len(q) && q[i] == c {
						i++
						continue
					}
					break
				}
				i++
			}
		case c == '-' && i < len(q) && q[i] == '-':
			for i < len(q) && q[i] != '\n' {
				i++
			}
		case c == '/' && i < len(q) && q[i] == '*':
			i++
			for i+1 < len(q) && q[i:i+2] != "*/" {
				i++
			}
			if i+1 < len(q) {
				i += 2
			}
		case i < len(q) && ((c == '<' && (q[i] == '>' || q[i] == '=')) || ((c == '>' || c == '!') && q[i] == '=') || (c == '|' && q[i] == '|') || (c == ':' && q[i] == ':')):
			i++
		case c == '?':
			for i < len(q) && q[i] >= '0' && q[i] <= '9' {
				i++
			}
		case c == ' ' || c == '\n' || c == '\t' || c == '\r':
			for i < len(q) && strings.ContainsRune(" \n\t\r", rune(q[i])) {
				i++
			}
		case c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || c >= '0' && c <= '9':
			for i < len(q) {
				v := q[i]
				if !(v >= 'a' && v <= 'z' || v >= 'A' && v <= 'Z' || v == '_' || v >= '0' && v <= '9') {
					break
				}
				i++
			}
		}
		out = append(out, q[start:i])
	}
	return out
}

// Significant returns SQL tokens with whitespace and comments removed.
func Significant(q string) []string {
	var out []string
	for _, t := range sqlTokens(q) {
		if strings.TrimSpace(t) == "" || strings.HasPrefix(t, "--") || strings.HasPrefix(t, "/*") {
			continue
		}
		out = append(out, t)
	}
	return out
}

// ClosingParen finds the token matching the opening parenthesis at start.
func ClosingParen(t []string, start int) int {
	depth := 0
	for i := start; i < len(t); i++ {
		if t[i] == "(" {
			depth++
		}
		if t[i] == ")" {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
