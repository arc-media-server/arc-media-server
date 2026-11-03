package store

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"
)

func significantSQL(q string) []string {
	var out []string
	for _, t := range sqlTokens(q) {
		if strings.TrimSpace(t) == "" || strings.HasPrefix(t, "--") || strings.HasPrefix(t, "/*") {
			continue
		}
		out = append(out, t)
	}
	return out
}
func postgresSQL(q string, args []driver.NamedValue) (string, []driver.NamedValue) {
	// Baseline functions/operators contain dollar-quoted bodies and already use native SQL.
	if strings.Contains(q, "$arc$") {
		return q, args
	}
	for i := range args {
		if b, ok := args[i].Value.(bool); ok {
			if b {
				args[i].Value = int64(1)
			} else {
				args[i].Value = int64(0)
			}
		}
	}
	t := significantSQL(q)
	// SQLite's indexed placeholders may repeat or leave unused input positions. Rebind used
	// arguments in occurrence order; never send an untyped unused $1 to PostgreSQL.
	var out []driver.NamedValue
	bound := map[int]int{}
	next := 0
	for i, x := range t {
		if !strings.HasPrefix(x, "?") {
			continue
		}
		index := 0
		if len(x) > 1 {
			index, _ = strconv.Atoi(x[1:])
			if index > next {
				next = index
			}
		} else {
			next++
			index = next
		}
		n, ok := bound[index]
		if !ok {
			n = len(out) + 1
			bound[index] = n
			if index > 0 && index <= len(args) {
				a := args[index-1]
				a.Ordinal = n
				a.Name = ""
				out = append(out, a)
			} else {
				out = append(out, driver.NamedValue{Ordinal: n})
			}
		}
		t[i] = fmt.Sprintf("$%d", n)
	}
	for i := 0; i+9 < len(t); i++ {
		if strings.EqualFold(strings.Join(t[i:i+10], ""), "lower(hex(randomblob(16)))") {
			t[i] = "replace(gen_random_uuid()::text,'-','')"
			for j := i + 1; j < i+10; j++ {
				t[j] = ""
			}
		}
	}
	for i := 0; i+1 < len(t); i++ {
		if (t[i] == "|" && t[i+1] == "|") || (t[i] == "<" && strings.Contains("=>", t[i+1])) || (t[i] == ">" && t[i+1] == "=") || (t[i] == ":" && t[i+1] == ":") {
			t[i] += t[i+1]
			t = append(t[:i+1], t[i+2:]...)
		}
	}
	for i := 0; i < len(t); i++ {
		switch strings.ToUpper(t[i]) {
		case "LIKE":
			t[i] = "~@~"
			if i+2 < len(t) && t[i+2] == "||" {
				end := i + 1
				for end+2 < len(t) && t[end+1] == "||" {
					end += 2
				}
				t[i+1] = "(" + t[i+1]
				t[end] += ")"
			}
			if i > 0 && strings.EqualFold(t[i-1], "NOT") {
				t[i-1] = ""
				t[i] = "!~@~"
			}
		case "LOWER":
			if i+1 < len(t) && t[i+1] == "(" {
				t[i] = "arc_lower"
			}
		case "GROUP_CONCAT":
			t[i] = "string_agg"
			end := closingSQL(t, i+1)
			if end > i {
				t[end] = ", ',' )"
			}
		case "MAX", "MIN":
			if i+1 < len(t) && t[i+1] == "(" {
				end := closingSQL(t, i+1)
				depth := 0
				for j := i + 2; j < end; j++ {
					if t[j] == "(" {
						depth++
					}
					if t[j] == ")" {
						depth--
					}
					if t[j] == "," && depth == 0 {
						if strings.EqualFold(t[i], "MAX") {
							t[i] = "GREATEST"
						} else {
							t[i] = "LEAST"
						}
						break
					}
				}
			}
		case "ROWID":
			t[i] = "arc_rowid"
		case "STRFTIME":
			end := closingSQL(t, i+1)
			if end > i && strings.Join(t[i:end+1], "") == "strftime('%Y',started_at,'unixepoch','localtime')" {
				t[i] = "EXTRACT(YEAR FROM to_timestamp(started_at))"
				for j := i + 1; j <= end; j++ {
					t[j] = ""
				}
			}
		case "COLLATE":
			if i+1 < len(t) && strings.EqualFold(t[i+1], "NOCASE") {
				t[i], t[i+1] = "", ""
				if i+2 < len(t) && t[i+2] == "=" {
					t[i+2] = "~=="
				} else if i >= 2 && t[i-2] == "=" {
					t[i-2] = "~=="
				} else {
					start := i - 1
					if start >= 2 && t[start-1] == "." {
						start -= 2
					}
					t[start] = "arc_lower(" + t[start]
					t[i-1] += ")"
				}
			}
		}
	}
	// INSERT OR IGNORE ignores only unique conflicts here; PostgreSQL still rejects bad FKs,
	// nulls and CHECKs rather than silently dropping corrupt state.
	ignore := len(t) > 2 && strings.EqualFold(t[0], "INSERT") && strings.EqualFold(t[1], "OR") && strings.EqualFold(t[2], "IGNORE")
	if ignore {
		t[1], t[2] = "", ""
		at := len(t)
		for i, x := range t {
			if strings.EqualFold(x, "RETURNING") {
				at = i
				break
			}
		}
		t = append(t[:at], append([]string{"ON CONFLICT DO NOTHING"}, t[at:]...)...)
	}
	qualifyPGUpsert(t)

	return strings.Join(t, " "), out
}
func closingSQL(t []string, start int) int {
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
func qualifyPGUpsert(t []string) {
	if len(t) < 4 || !strings.EqualFold(t[0], "INSERT") {
		return
	}
	table := ""
	for i, x := range t {
		if strings.EqualFold(x, "INTO") && i+1 < len(t) {
			table = t[i+1]
			break
		}
	}
	cols := pgColumns[table]
	if len(cols) == 0 {
		return
	}
	start := -1
	for i := 0; i+2 < len(t); i++ {
		if strings.EqualFold(t[i], "DO") && strings.EqualFold(t[i+1], "UPDATE") && strings.EqualFold(t[i+2], "SET") {
			start = i + 3
			break
		}
	}
	if start < 0 {
		return
	}
	depth := 0
	lhs := true
	for i := start; i < len(t); i++ {
		x := t[i]
		if x == "(" {
			depth++
		}
		if x == ")" {
			depth--
		}
		if depth == 0 && x == "," {
			lhs = true
			continue
		}
		if lhs {
			if x == "=" {
				lhs = false
			}
			continue
		}
		for _, col := range cols {
			if x == col && (i == 0 || t[i-1] != ".") && (i+1 == len(t) || t[i+1] != "." && t[i+1] != "(") {
				t[i] = table + "." + x
				break
			}
		}
	}
}
