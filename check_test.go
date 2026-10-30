package main

import (
	"fmt"
	"reflect"
	"testing"
)

// short renders an issue as "severity kind KEY file:line" for easy comparison.
func short(is Issue) string {
	return fmt.Sprintf("%s %s %s %s:%d", is.Severity, is.Kind, is.Key, is.File, is.Line)
}

func mustParse(t *testing.T, src string) []Entry {
	t.Helper()
	entries, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func TestCheck(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		example string
		want    []string
	}{
		{"all present", "A=1\nB=2", "A=\nB=x", nil},
		{"order does not matter", "B=2\nA=1", "A=\nB=", nil},
		{"missing", "A=1", "A=\nB=\nC=", []string{"error missing B ex:2", "error missing C ex:3"}},
		{"extra", "A=1\nZ=9", "A=", []string{"warning extra Z env:2"}},
		{"empty", "A=\nB=''\nC=\" \"", "A=\nB=\nC=", []string{"error empty A env:1", "error empty B env:2"}},
		{"duplicate in env", "A=1\nB=2\nA=3", "A=\nB=", []string{"error duplicate A env:3"}},
		{"last duplicate is checked", "A=1\nA=", "A=", []string{"error duplicate A env:2", "error empty A env:2"}},
		{"duplicate in example", "A=1", "A=\nA=", []string{"error duplicate A ex:2"}},
		{"duplicate extra reported once", "Z=1\nZ=2", "", []string{"error duplicate Z env:2", "warning extra Z env:2"}},
		{"empty files", "", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := Check(mustParse(t, tt.env), mustParse(t, tt.example), Options{EnvPath: "env", ExamplePath: "ex"})
			var got []string
			for _, is := range issues {
				got = append(got, short(is))
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got  %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestCount(t *testing.T) {
	issues := []Issue{{Severity: SevError}, {Severity: SevWarning}, {Severity: SevError}}
	if errs, warns := Count(issues); errs != 2 || warns != 1 {
		t.Fatalf("got %d errors, %d warnings, want 2 and 1", errs, warns)
	}
}
