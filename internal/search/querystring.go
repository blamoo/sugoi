package search

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Clause is one space-separated part of a Bleve query string, such as
// `-artist_kw:"alice b"`, `rating:>=4` or `landscape~1`.
//
// Bleve's query-string syntax has no grouping or boolean operators, so a
// query is just a list of clauses:
//
//	[+|-] [field:] [>|>=|<|<=] value [^boost|~fuzziness]
//
// where field and value are a bare string or a "quoted phrase", and a bare
// value may be a /regexp/ or contain * and ? wildcards.
type Clause struct {
	Raw    string // the clause exactly as written, so a query can be rebuilt unchanged
	Prefix string // "+", "-" or ""
	Field  string // unescaped; "" when the clause has no field (or an empty `"":`)
	Op     string // ">", ">=", "<", "<=" or ""
	Value  string // unescaped, without the quotes of a phrase (`f:\-1` and `f:-1` both give "-1")
	Phrase bool   // Value was quoted
	Suffix string // raw "^boost" or "~fuzziness", or ""
}

// ParseQueryString splits a Bleve query string into clauses, following the
// tokenization of Bleve's own lexer. It never fails: malformed input (an
// unterminated quote, a dangling "field:") still yields clauses, so callers
// can rewrite whatever the user typed. Rebuild the query with JoinClauses.
//
// Like in Bleve, whitespace only separates tokens: "rating : 5", "rating: 5"
// and "+ rating:5" are single clauses, and Raw keeps their inner spaces.
func ParseQueryString(q string) []Clause {
	// at returns the position of the next token from i on, and whether it
	// starts with one of chars.
	at := func(i int, chars string) (int, bool) {
		j := i + skipQuerySpace(q[i:])
		return j, j < len(q) && strings.IndexByte(chars, q[j]) >= 0
	}

	var ret []Clause
	i := 0
	for {
		i += skipQuerySpace(q[i:])
		if i >= len(q) {
			return ret
		}

		start := i
		var c Clause
		if q[i] == '+' || q[i] == '-' {
			c.Prefix = q[i : i+1]
			i += 1 + skipQuerySpace(q[i+1:])
		}

		val, phrase, n := scanQueryValue(q[i:])
		i += n
		if j, ok := at(i, ":"); ok {
			c.Field = val
			i = j + 1
			if j, ok := at(i, "<>"); ok {
				c.Op = q[j : j+1]
				i = j + 1
				if j, ok := at(i, "="); ok {
					c.Op += "="
					i = j + 1
				}
			}
			sign := ""
			if j, ok := at(i, "-"); ok { // a negative number, "-" being its own token
				sign = "-"
				i = j + 1
			}
			i += skipQuerySpace(q[i:])
			val, phrase, n = scanQueryValue(q[i:])
			val = sign + val
			i += n
		}
		c.Value, c.Phrase = val, phrase

		if j, ok := at(i, "^~"); ok {
			i = j
			s := i
			i++
			for i < len(q) && q[i] != ' ' {
				if q[i] == '\\' && i+1 < len(q) {
					i++
				}
				i++
			}
			c.Suffix = q[s:i]
		}

		c.Raw = q[start:i]
		ret = append(ret, c)
	}
}

// JoinClauses rebuilds a query string from clauses, using each one's Raw text.
func JoinClauses(clauses []Clause) string {
	parts := make([]string, 0, len(clauses))
	for _, c := range clauses {
		if c.Raw != "" {
			parts = append(parts, c.Raw)
		}
	}
	return strings.Join(parts, " ")
}

// String builds the clause from its parsed parts (ignoring Raw), escaping the
// field and value as needed. Use it for clauses created in code.
func (c Clause) String() string {
	var b strings.Builder
	b.WriteString(c.Prefix)
	if c.Field != "" {
		// A bare number can't be a field name, and a bare token loses its
		// leading space; a phrase keeps both.
		if looksLikeQueryNumber(c.Field) || skipQuerySpace(c.Field) > 0 {
			b.WriteString(Quote(c.Field))
		} else {
			b.WriteString(escapeQueryString(c.Field, false))
		}
		b.WriteByte(':')
		b.WriteString(c.Op)
	}
	if c.Phrase {
		b.WriteString(Quote(c.Value))
	} else {
		// After "field:" a leading '-' may start a negative number.
		b.WriteString(escapeQueryString(c.Value, c.Field != ""))
	}
	b.WriteString(c.Suffix)
	return b.String()
}

// scanQueryValue reads a phrase or a bare string at the start of s and
// returns its unescaped value and the number of bytes consumed. A bare string
// ends at an unescaped ' ', ':', '^' or '~' (other whitespace is part of it,
// as in Bleve's lexer); an unterminated phrase runs to the end of s.
func scanQueryValue(s string) (val string, phrase bool, n int) {
	var b strings.Builder
	if strings.HasPrefix(s, `"`) {
		i := 1
		for i < len(s) {
			switch {
			case s[i] == '"':
				return b.String(), true, i + 1
			case s[i] == '\\':
				i += writeUnescaped(&b, s[i+1:])
			default:
				b.WriteByte(s[i])
			}
			i++
		}
		return b.String(), true, i
	}

	i := 0
	for i < len(s) && s[i] != ' ' && s[i] != ':' && s[i] != '^' && s[i] != '~' {
		if s[i] == '\\' {
			i += writeUnescaped(&b, s[i+1:])
		} else {
			b.WriteByte(s[i])
		}
		i++
	}
	return b.String(), false, i
}

// writeUnescaped writes the character escaped by a backslash, s being the
// text right after it. Like Bleve, the backslash is dropped only before a
// reserved character, and a trailing backslash is dropped. Returns the bytes
// of s consumed.
func writeUnescaped(b *strings.Builder, s string) int {
	if s == "" {
		return 0
	}
	if strings.IndexByte(queryReservedChars, s[0]) < 0 {
		b.WriteByte('\\')
	}
	b.WriteByte(s[0])
	return 1
}

// queryReservedChars are the characters a backslash escapes in Bleve's lexer.
const queryReservedChars = "+-=&|><!(){}[]^\"~*?:\\/ "

// escapeQueryString escapes the characters that would end a bare string or
// be read as an operator at its start, except a '-' starting a negative
// number when negNumber is set. Wildcards and regexp slashes are left alone,
// since Bleve gives them meaning even when escaped.
func escapeQueryString(s string, negNumber bool) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\', ' ', ':', '^', '~', '"':
			b.WriteByte('\\')
		case '+', '>', '<', '=':
			if i == 0 {
				b.WriteByte('\\')
			}
		case '-':
			if i == 0 && !(negNumber && looksLikeQueryNumber(s[1:])) {
				b.WriteByte('\\')
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// looksLikeQueryNumber reports whether Bleve's lexer would read s as a
// number token: a digit followed by digits and at most one dot.
func looksLikeQueryNumber(s string) bool {
	if s == "" || s[0] < '0' || s[0] > '9' {
		return false
	}
	dot := false
	for i := 1; i < len(s); i++ {
		switch {
		case s[i] == '.' && !dot:
			dot = true
		case s[i] < '0' || s[i] > '9':
			return false
		}
	}
	return true
}

// skipQuerySpace returns the length of the whitespace at the start of s.
// Between clauses Bleve skips any Unicode space, though only ' ' ends one.
func skipQuerySpace(s string) int {
	i := 0
	for i < len(s) {
		r, n := utf8.DecodeRuneInString(s[i:])
		if !unicode.IsSpace(r) {
			break
		}
		i += n
	}
	return i
}
