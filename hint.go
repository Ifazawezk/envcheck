package main

import (
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Hint is what a trailing comment in .env.example says about a key:
//
//	PORT=8080 # int
//	DEBUG=false # bool
//	API_URL= # url
//	ADMIN_EMAIL= # email
//	LOG_LEVEL=info # one of: debug,info,warn,error
//	SENTRY_DSN= # optional url
//
// "optional" can be combined with any type. Put it before "one of:",
// since everything after the colon is read as the list of allowed values.
// "required" is accepted too, but changes nothing: keys are required anyway.
type Hint struct {
	Optional bool
	Type     string   // "", "int", "bool", "url" or "email"
	OneOf    []string // allowed values, compared exactly
}

// ParseHint reads a hint from a comment. A comment that is not made only
// of hint words, such as "# ask the ops team", is just a comment and gives
// the zero Hint.
func ParseHint(comment string) Hint {
	s := strings.TrimSpace(comment)
	var h Hint
	if loc := oneOfWord.FindStringIndex(s); loc != nil {
		for _, v := range strings.Split(s[loc[1]:], ",") {
			if v = strings.TrimSpace(v); v != "" {
				h.OneOf = append(h.OneOf, v)
			}
		}
		if len(h.OneOf) == 0 {
			return Hint{}
		}
		s = s[:loc[0]]
	}
	words := strings.FieldsFunc(s, func(r rune) bool { return r == ' ' || r == '\t' || r == ',' })
	for _, w := range words {
		w = strings.ToLower(w)
		if w == "required" {
			continue
		}
		if w == "optional" {
			h.Optional = true
			continue
		}
		typ, ok := typeWords[w]
		if !ok || h.Type != "" || h.OneOf != nil {
			return Hint{}
		}
		h.Type = typ
	}
	return h
}

// A hint with a word it doesn't know counts as a plain comment, which turns the
// check off without saying so. These cover the other ways people write a hint.
var (
	oneOfWord = regexp.MustCompile(`(?i)one of:`)
	typeWords = map[string]string{
		"int": "int", "integer": "int",
		"bool": "bool", "boolean": "bool",
		"url": "url", "email": "email",
	}
)

// Valid reports whether a non-empty value matches the hint.
func (h Hint) Valid(v string) bool {
	if h.OneOf != nil {
		for _, allowed := range h.OneOf {
			if v == allowed {
				return true
			}
		}
		return false
	}
	switch h.Type {
	case "int":
		_, err := strconv.ParseInt(v, 10, 64)
		return err == nil
	case "bool":
		switch strings.ToLower(v) {
		case "true", "false", "1", "0", "yes", "no", "on", "off":
			return true
		}
		return false
	case "url":
		u, err := url.Parse(v)
		return err == nil && u.Scheme != "" && u.Host != ""
	case "email":
		a, err := mail.ParseAddress(v)
		return err == nil && a.Address == v
	}
	return true
}

// Want describes what the hint expects, for use in messages.
func (h Hint) Want() string {
	if h.OneOf != nil {
		return "one of " + strings.Join(h.OneOf, ", ")
	}
	switch h.Type {
	case "int":
		return "a whole number"
	case "bool":
		return "a boolean like true or false"
	case "url":
		return "a URL with a scheme and host"
	case "email":
		return "an email address"
	}
	return "set"
}
