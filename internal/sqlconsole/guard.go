// Package sqlconsole is the Super Admin SQL console: one statement per request,
// run in a rolled-back-on-error transaction, with server-enforced confirmation
// for destructive statements.
package sqlconsole

import (
	"errors"
	"strings"
	"unicode"
)

var (
	ErrEmpty         = errors.New("Enter a SQL statement")
	ErrMultiple      = errors.New("Run one SQL statement at a time")
	ErrBlockedVerb   = errors.New("This statement type is not allowed in the console (transaction control, SET/RESET and COPY)")
	blockedFirstWord = map[string]bool{"begin": true, "start": true, "commit": true, "end": true, "rollback": true, "abort": true, "savepoint": true, "release": true, "set": true, "reset": true, "copy": true}
)

type Info struct {
	Verb        string
	Destructive bool
	Reason      string
}

// strip removes comments and replaces string/identifier/dollar-quoted bodies so
// keyword checks cannot be fooled by text inside them.
func strip(sql string) string {
	var b strings.Builder
	r := []rune(sql)
	for i := 0; i < len(r); {
		c := r[i]
		switch {
		case c == '-' && i+1 < len(r) && r[i+1] == '-':
			for i < len(r) && r[i] != '\n' {
				i++
			}
			b.WriteRune(' ')
		case c == '/' && i+1 < len(r) && r[i+1] == '*':
			depth := 0
			for i < len(r) {
				if r[i] == '/' && i+1 < len(r) && r[i+1] == '*' {
					depth++
					i += 2
				} else if r[i] == '*' && i+1 < len(r) && r[i+1] == '/' {
					depth--
					i += 2
					if depth == 0 {
						break
					}
				} else {
					i++
				}
			}
			b.WriteRune(' ')
		case c == '\'' || c == '"':
			escape := c == '\'' && i > 0 && (r[i-1] == 'E' || r[i-1] == 'e')
			i++
			for i < len(r) {
				if escape && r[i] == '\\' {
					i += 2
					continue
				}
				if r[i] == c {
					if i+1 < len(r) && r[i+1] == c {
						i += 2
						continue
					}
					break
				}
				i++
			}
			i++
			b.WriteString(" _q ")
		case c == '$':
			j := i + 1
			for j < len(r) && (unicode.IsLetter(r[j]) || unicode.IsDigit(r[j]) || r[j] == '_') {
				j++
			}
			if j < len(r) && r[j] == '$' && (j == i+1 || !unicode.IsDigit(r[i+1])) {
				tag := r[i : j+1]
				k := j + 1
				for k+len(tag) <= len(r) && string(r[k:k+len(tag)]) != string(tag) {
					k++
				}
				i = min(k+len(tag), len(r))
				b.WriteString(" _q ")
				continue
			}
			b.WriteRune(c)
			i++
		default:
			b.WriteRune(c)
			i++
		}
	}
	return b.String()
}

// Analyze classifies one statement. Destructive means DROP, TRUNCATE, or
// UPDATE/DELETE with no WHERE keyword anywhere (conservative: WHERE true still passes).
func Analyze(sql string) (Info, error) {
	clean := strings.TrimSpace(strip(sql))
	clean = strings.TrimSpace(strings.TrimSuffix(clean, ";"))
	if clean == "" {
		return Info{}, ErrEmpty
	}
	if strings.Contains(clean, ";") {
		return Info{}, ErrMultiple
	}
	words := strings.FieldsFunc(strings.ToLower(clean), func(c rune) bool { return !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '_' })
	if len(words) == 0 {
		return Info{}, ErrEmpty
	}
	info := Info{Verb: strings.ToUpper(words[0])}
	if blockedFirstWord[words[0]] {
		return info, ErrBlockedVerb
	}
	set := map[string]bool{}
	for _, w := range words {
		set[w] = true
	}
	switch {
	case set["drop"]:
		info.Destructive, info.Reason = true, "DROP permanently removes tables, columns or constraints"
	case set["truncate"]:
		info.Destructive, info.Reason = true, "TRUNCATE removes every row from the table"
	case (set["delete"] || set["update"]) && !set["where"]:
		info.Destructive, info.Reason = true, "UPDATE/DELETE without WHERE changes every row"
	}
	return info, nil
}
