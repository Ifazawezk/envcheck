package main

import "fmt"

// Severity is how serious an issue is. Only errors make envcheck fail.
type Severity string

const (
	SevError   Severity = "error"
	SevWarning Severity = "warning"
)

// Issue is one problem found when comparing a .env file to its example.
type Issue struct {
	Severity Severity `json:"severity"`
	Kind     string   `json:"kind"` // missing, extra, empty, duplicate or invalid
	Key      string   `json:"key"`
	File     string   `json:"file"` // the file the issue points at
	Line     int      `json:"line"`
	Message  string   `json:"message"`
	Value    string   `json:"value,omitempty"` // masked, only for invalid values
}

// Options names the two files, so issues can point at the right one. With
// Strict set, keys that the example does not list are errors, not warnings.
type Options struct {
	EnvPath     string
	ExamplePath string
	Strict      bool
}

// Check compares the entries of a .env file with those of .env.example.
// When a key is set more than once the last assignment is the one checked,
// since that is the value most loaders end up using. Values never appear in
// issues unmasked.
// Issues in the .env file come first, in line order, then issues that point
// at the example (duplicates and keys missing from .env), also in line order.
func Check(env, example []Entry, opt Options) []Issue {
	want := lastByKey(example)
	have := lastByKey(env)
	var issues []Issue
	add := func(sev Severity, kind string, e Entry, file, msg string) {
		issues = append(issues, Issue{Severity: sev, Kind: kind, Key: e.Key, File: file, Line: e.Line, Message: msg})
	}

	firstLine := map[string]int{}
	for _, e := range env {
		if line, ok := firstLine[e.Key]; ok {
			add(SevError, "duplicate", e, opt.EnvPath, fmt.Sprintf("%s is set more than once (first on line %d)", e.Key, line))
		} else {
			firstLine[e.Key] = e.Line
		}
		if have[e.Key] != e {
			continue // a later line sets this key again
		}
		spec, ok := want[e.Key]
		if !ok {
			sev := SevWarning
			if opt.Strict {
				sev = SevError
			}
			add(sev, "extra", e, opt.EnvPath, fmt.Sprintf("%s is not in %s", e.Key, opt.ExamplePath))
			continue
		}
		h := ParseHint(spec.Comment)
		switch {
		case e.Value == "":
			if !h.Optional {
				add(SevError, "empty", e, opt.EnvPath, e.Key+" is empty")
			}
		case !h.Valid(e.Value):
			masked := Mask(e.Value)
			add(SevError, "invalid", e, opt.EnvPath, fmt.Sprintf("%s should be %s (got %q)", e.Key, h.Want(), masked))
			issues[len(issues)-1].Value = masked
		}
	}

	firstLine = map[string]int{}
	for _, s := range example {
		if line, ok := firstLine[s.Key]; ok {
			add(SevError, "duplicate", s, opt.ExamplePath, fmt.Sprintf("%s is declared more than once (first on line %d)", s.Key, line))
		} else {
			firstLine[s.Key] = s.Line
		}
		if want[s.Key] != s {
			continue
		}
		if _, ok := have[s.Key]; !ok && !ParseHint(s.Comment).Optional {
			add(SevError, "missing", s, opt.ExamplePath, fmt.Sprintf("%s is missing from %s", s.Key, opt.EnvPath))
		}
	}
	return issues
}

// Count returns the number of errors and warnings in issues.
func Count(issues []Issue) (errs, warns int) {
	for _, is := range issues {
		if is.Severity == SevError {
			errs++
		} else {
			warns++
		}
	}
	return errs, warns
}

// Mask hides a value so it can be shown in output. Values shorter than 8
// characters are hidden completely; longer ones keep their first two
// characters, which is usually enough to tell which value is meant. The
// number of stars is always the same, so the length is not given away.
func Mask(v string) string {
	r := []rune(v)
	if len(r) < 8 {
		return "****"
	}
	return string(r[:2]) + "****"
}

func lastByKey(entries []Entry) map[string]Entry {
	m := make(map[string]Entry, len(entries))
	for _, e := range entries {
		m[e.Key] = e
	}
	return m
}
