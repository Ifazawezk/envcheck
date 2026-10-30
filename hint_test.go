package main

import (
	"reflect"
	"testing"
)

func TestParseHint(t *testing.T) {
	tests := []struct {
		comment string
		want    Hint
	}{
		{"", Hint{}},
		{"int", Hint{Type: "int"}},
		{" bool ", Hint{Type: "bool"}},
		{"url", Hint{Type: "url"}},
		{"EMAIL", Hint{Type: "email"}},
		{"optional", Hint{Optional: true}},
		{"optional url", Hint{Optional: true, Type: "url"}},
		{"int, optional", Hint{Optional: true, Type: "int"}},
		{"one of: debug,info,warn,error", Hint{OneOf: []string{"debug", "info", "warn", "error"}}},
		{"optional, one of: eu-west-1, us-east-1 ", Hint{Optional: true, OneOf: []string{"eu-west-1", "us-east-1"}}},
		// Ordinary comments are not hints.
		{"ask the ops team for this", Hint{}},
		{"optional, used for metrics", Hint{}},
		{"int bool", Hint{}},
		{"int one of: 1,2", Hint{}},
		{"one of:", Hint{}},
	}
	for _, tt := range tests {
		if got := ParseHint(tt.comment); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ParseHint(%q) = %+v, want %+v", tt.comment, got, tt.want)
		}
	}
}

func TestHintValid(t *testing.T) {
	levels := Hint{OneOf: []string{"debug", "info"}}
	tests := []struct {
		hint  Hint
		value string
		want  bool
	}{
		{Hint{}, "anything", true},
		{Hint{Type: "int"}, "8080", true},
		{Hint{Type: "int"}, "-3", true},
		{Hint{Type: "int"}, "80a", false},
		{Hint{Type: "int"}, "1.5", false},
		{Hint{Type: "bool"}, "true", true},
		{Hint{Type: "bool"}, "FALSE", true},
		{Hint{Type: "bool"}, "0", true},
		{Hint{Type: "bool"}, "yes", true},
		{Hint{Type: "bool"}, "maybe", false},
		{Hint{Type: "url"}, "https://api.example.com/v1", true},
		{Hint{Type: "url"}, "postgres://user:pass@db:5432/app", true},
		{Hint{Type: "url"}, "api.example.com", false},
		{Hint{Type: "url"}, "http://", false},
		{Hint{Type: "email"}, "admin@example.com", true},
		{Hint{Type: "email"}, "Admin <admin@example.com>", false},
		{Hint{Type: "email"}, "admin.example.com", false},
		{levels, "info", true},
		{levels, "INFO", false},
		{levels, "verbose", false},
	}
	for _, tt := range tests {
		if got := tt.hint.Valid(tt.value); got != tt.want {
			t.Errorf("%+v.Valid(%q) = %v, want %v", tt.hint, tt.value, got, tt.want)
		}
	}
}
