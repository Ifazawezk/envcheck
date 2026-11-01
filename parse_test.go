package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []Entry
	}{
		{"plain", "A=1\nB=two", []Entry{{Key: "A", Value: "1", Line: 1}, {Key: "B", Value: "two", Line: 2}}},
		{"comments and blank lines", "# header\n\n  # indented\nA=1\n\n", []Entry{{Key: "A", Value: "1", Line: 4}}},
		{"export prefix", "export A=1\nexport\tB=2", []Entry{{Key: "A", Value: "1", Line: 1}, {Key: "B", Value: "2", Line: 2}}},
		{"key named export", "export=yes", []Entry{{Key: "export", Value: "yes", Line: 1}}},
		{"spaces around equals", "A = hello world  ", []Entry{{Key: "A", Value: "hello world", Line: 1}}},
		{"empty value", "A=\nB=''\nC=\"\"", []Entry{{Key: "A", Line: 1}, {Key: "B", Line: 2}, {Key: "C", Line: 3}}},
		{"inline comment", "PORT=8080 # int", []Entry{{Key: "PORT", Value: "8080", Comment: "int", Line: 1}}},
		{"comment after empty value", "API_URL= # url", []Entry{{Key: "API_URL", Comment: "url", Line: 1}}},
		{"hash without space is part of value", "COLOR=#fff\nPASS=abc#123", []Entry{{Key: "COLOR", Value: "#fff", Line: 1}, {Key: "PASS", Value: "abc#123", Line: 2}}},
		{"single quotes are literal", `A='x # y \n "z"'`, []Entry{{Key: "A", Value: `x # y \n "z"`, Line: 1}}},
		{"double quote escapes", `A="a\nb\tc \"q\" \\ \$HOME \d"`, []Entry{{Key: "A", Value: "a\nb\tc \"q\" \\ $HOME \\d", Line: 1}}},
		{"hash inside double quotes", `A="x # y" # note`, []Entry{{Key: "A", Value: "x # y", Comment: "note", Line: 1}}},
		{"comment after single quotes", `A='x'   #  note  `, []Entry{{Key: "A", Value: "x", Comment: "note", Line: 1}}},
		{
			"multi-line double quotes",
			"KEY=\"-----BEGIN-----\nabc  \n-----END-----\"\nNEXT=1",
			[]Entry{{Key: "KEY", Value: "-----BEGIN-----\nabc  \n-----END-----", Line: 1}, {Key: "NEXT", Value: "1", Line: 4}},
		},
		{
			"multi-line single quotes",
			"KEY='-----BEGIN-----\nabc  \n-----END-----'\nNEXT=1",
			[]Entry{{Key: "KEY", Value: "-----BEGIN-----\nabc  \n-----END-----", Line: 1}, {Key: "NEXT", Value: "1", Line: 4}},
		},
		{"multi-line single quotes are literal", "A='x \\n\n\"y\" # z'", []Entry{{Key: "A", Value: "x \\n\n\"y\" # z", Line: 1}}},
		{"comment after multi-line single quotes", "A='x\ny' # note\nB=2", []Entry{{Key: "A", Value: "x\ny", Comment: "note", Line: 1}, {Key: "B", Value: "2", Line: 3}}},
		{"single quote with blank lines inside", "A='x\n\n\ny'", []Entry{{Key: "A", Value: "x\n\n\ny", Line: 1}}},
		{"windows line endings", "A=1\r\nB=\"x\r\ny\"\r\nC='p\r\nq'\r\n", []Entry{{Key: "A", Value: "1", Line: 1}, {Key: "B", Value: "x\ny", Line: 2}, {Key: "C", Value: "p\nq", Line: 4}}},
		{"byte order mark", "\xef\xbb\xbfA=1", []Entry{{Key: "A", Value: "1", Line: 1}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.src)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got  %+v\nwant %+v", got, tt.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		src  string
		line int
		msg  string
	}{
		{"A=1\nsecret-on-its-own", 2, "expected KEY=value"},
		{"1A=x", 1, "invalid key name"},
		{"MY-KEY=x", 1, "invalid key name"},
		{"=x", 1, "invalid key name"},
		{"A=1\nB=\"never closed\nC=3", 2, "missing closing double quote"},
		{"A='open", 1, "missing closing single quote"},
		{"A=1\nB='never closed\nC=3", 2, "missing closing single quote"},
		{"A='one\ntwo' extra", 2, "unexpected text after closing quote"},
		{"\xff\xfeA\x00=\x001\x00\r\x00\n\x00", 1, "UTF-16"},
		{"\xfe\xff\x00A\x00=\x001", 1, "UTF-16"},
		{"\xff\xfe\x45\x06\x47\x06", 1, "UTF-16"}, // Arabic text has no NUL bytes
		{"A=1\nB=2\nC\x00=\x003\x00", 3, "UTF-16"},
		{"A=\"x\"y", 1, "unexpected text after closing quote"},
		{"A=\"one\ntwo\" extra", 2, "unexpected text after closing quote"},
	}
	for _, tt := range tests {
		_, err := Parse(tt.src)
		pe, ok := err.(*ParseError)
		if !ok {
			t.Errorf("%q: got error %v, want a ParseError", tt.src, err)
			continue
		}
		if pe.Line != tt.line || !strings.Contains(pe.Msg, tt.msg) {
			t.Errorf("%q: got %v, want line %d: %s", tt.src, err, tt.line, tt.msg)
		}
	}
}

func TestParseErrorsHideValues(t *testing.T) {
	for _, src := range []string{"sk_live_51Habc", "abc+/def==", "A='sk_live_51Habc"} {
		_, err := Parse(src)
		if err == nil {
			t.Fatalf("%q: expected an error", src)
		}
		if strings.Contains(err.Error(), "abc") {
			t.Errorf("error for %q leaks the value: %v", src, err)
		}
	}
}
