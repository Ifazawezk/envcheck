package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// Entry is one KEY=value assignment from a .env file.
type Entry struct {
	Key     string
	Value   string
	Comment string // text of a trailing "# ..." comment, if any
	Line    int    // line the assignment starts on
}

// ParseError reports a syntax error and the line it was found on.
type ParseError struct {
	Line int
	Msg  string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
}

// ParseFile reads and parses a .env file. Syntax errors are reported as
// "path:line: message".
func ParseFile(path string) ([]Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	entries, err := Parse(string(data))
	var pe *ParseError
	if errors.As(err, &pe) {
		return nil, fmt.Errorf("%s:%d: %s", path, pe.Line, pe.Msg)
	}
	return entries, err
}

// Parse reads .env syntax: KEY=value lines, optional "export " prefixes,
// # comments, single quotes (taken literally), double quotes (with escapes),
// both of which may span several lines, and inline comments after unquoted
// values.
// Error messages never include values, since they are often secrets.
func Parse(src string) ([]Entry, error) {
	src = strings.TrimPrefix(src, "\xef\xbb\xbf") // UTF-8 byte order mark
	// Windows PowerShell's > writes UTF-16, and its >> can add UTF-16 to the end
	// of a UTF-8 file, so the NUL bytes may start on any line.
	nul := strings.IndexByte(src, 0)
	if nul >= 0 || strings.HasPrefix(src, "\xff\xfe") || strings.HasPrefix(src, "\xfe\xff") {
		line := strings.Count(src[:max(nul, 0)], "\n") + 1
		return nil, &ParseError{line, "this looks like UTF-16 text, save the file as UTF-8"}
	}
	src = strings.ReplaceAll(src, "\r\n", "\n")
	lines := strings.Split(src, "\n")

	var entries []Entry
	for i := 0; i < len(lines); i++ {
		line := strings.TrimLeft(lines[i], " \t")
		if strings.TrimSpace(line) == "" || line[0] == '#' {
			continue
		}
		start := i + 1
		if rest, ok := strings.CutPrefix(line, "export"); ok && rest != "" && (rest[0] == ' ' || rest[0] == '\t') {
			line = strings.TrimLeft(rest, " \t")
		}
		key, raw, ok := strings.Cut(line, "=")
		if !ok {
			return nil, &ParseError{start, "expected KEY=value"}
		}
		key = strings.TrimSpace(key)
		if !validKey(key) {
			return nil, &ParseError{start, "invalid key name (use letters, digits and underscores, not starting with a digit)"}
		}

		e := Entry{Key: key, Line: start}
		trimmed := strings.TrimLeft(raw, " \t")
		var rest string
		var end int
		var err error
		switch {
		case strings.HasPrefix(trimmed, `"`):
			e.Value, rest, end, err = doubleQuoted(lines, i, trimmed[1:])
			if err != nil {
				return nil, err
			}
			i = end
		case strings.HasPrefix(trimmed, "'"):
			e.Value, rest, end, err = singleQuoted(lines, i, trimmed[1:])
			if err != nil {
				return nil, err
			}
			i = end
		default:
			e.Value, e.Comment = splitComment(raw)
		}

		if rest = strings.TrimSpace(rest); rest != "" {
			if rest[0] != '#' {
				return nil, &ParseError{i + 1, "unexpected text after closing quote"}
			}
			e.Comment = strings.TrimSpace(rest[1:])
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// singleQuoted reads a single-quoted value that starts on lines[i] with s
// (the text after the opening quote). The text is taken literally, including
// any line breaks. It returns the same values as doubleQuoted.
func singleQuoted(lines []string, i int, s string) (value, rest string, end int, err error) {
	start := i + 1
	var b strings.Builder
	for {
		if j := strings.IndexByte(s, '\''); j >= 0 {
			b.WriteString(s[:j])
			return b.String(), s[j+1:], i, nil
		}
		b.WriteString(s)
		i++
		if i >= len(lines) {
			return "", "", 0, &ParseError{start, "missing closing single quote"}
		}
		b.WriteByte('\n')
		s = lines[i]
	}
}

// doubleQuoted reads a double-quoted value that starts on lines[i] with s
// (the text after the opening quote). It returns the value, whatever follows
// the closing quote, and the index of the line where the value ends.
func doubleQuoted(lines []string, i int, s string) (value, rest string, end int, err error) {
	start := i + 1
	var b strings.Builder
	for {
		for j := 0; j < len(s); j++ {
			c := s[j]
			switch {
			case c == '"':
				return b.String(), s[j+1:], i, nil
			case c == '\\' && j+1 < len(s):
				j++
				switch s[j] {
				case 'n':
					b.WriteByte('\n')
				case 'r':
					b.WriteByte('\r')
				case 't':
					b.WriteByte('\t')
				case '"', '\\', '$':
					b.WriteByte(s[j])
				default:
					// Unknown escapes are kept as written.
					b.WriteByte('\\')
					b.WriteByte(s[j])
				}
			default:
				b.WriteByte(c)
			}
		}
		i++
		if i >= len(lines) {
			return "", "", 0, &ParseError{start, "missing closing double quote"}
		}
		b.WriteByte('\n')
		s = lines[i]
	}
}

// splitComment separates an unquoted value from an inline comment. A # only
// starts a comment when whitespace comes before it, so KEY=#fff and
// KEY=abc#123 keep the # in the value.
func splitComment(raw string) (value, comment string) {
	for j := 1; j < len(raw); j++ {
		if raw[j] == '#' && (raw[j-1] == ' ' || raw[j-1] == '\t') {
			return strings.TrimSpace(raw[:j]), strings.TrimSpace(raw[j+1:])
		}
	}
	return strings.TrimSpace(raw), ""
}

func validKey(k string) bool {
	if k == "" || (k[0] >= '0' && k[0] <= '9') {
		return false
	}
	for _, c := range k {
		if c != '_' && (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}
