package main

import (
	"fmt"
	"reflect"
	"strings"
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
		{"optional key may be missing", "A=1", "A=\nB= # optional", nil},
		{"optional key may be empty", "A=1\nB=", "A=\nB= # optional url", nil},
		{"optional value is still checked", "B=not-a-url", "B= # optional url", []string{"error invalid B env:1"}},
		{"empty required value is not type checked", "PORT=", "PORT=8080 # int", []string{"error empty PORT env:1"}},
		{
			"values checked against hints",
			"PORT=80a\nLEVEL=verbose\nDEBUG=true\nNOTE=hi",
			"PORT=8080 # int\nLEVEL=info # one of: debug,info\nDEBUG=false # bool\nNOTE= # free text, ask the team",
			[]string{"error invalid PORT env:1", "error invalid LEVEL env:2"},
		},
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

func TestCheckStrict(t *testing.T) {
	env := mustParse(t, "A=1\nZ=9")
	example := mustParse(t, "A=")
	for _, strict := range []bool{false, true} {
		issues := Check(env, example, Options{EnvPath: "env", ExamplePath: "ex", Strict: strict})
		want := SevWarning
		if strict {
			want = SevError
		}
		if len(issues) != 1 || issues[0].Kind != "extra" || issues[0].Severity != want {
			t.Errorf("strict=%v: got %+v, want one extra %s", strict, issues, want)
		}
	}
}

func TestCheckMasksInvalidValues(t *testing.T) {
	env := mustParse(t, "ADMIN_EMAIL=root-at-internal.example.com")
	example := mustParse(t, "ADMIN_EMAIL= # email")
	issues := Check(env, example, Options{EnvPath: "env", ExamplePath: "ex"})
	if len(issues) != 1 {
		t.Fatalf("got %d issues, want 1", len(issues))
	}
	is := issues[0]
	if is.Value != "ro****" {
		t.Errorf("value = %q, want %q", is.Value, "ro****")
	}
	if want := `ADMIN_EMAIL should be an email address (got "ro****")`; is.Message != want {
		t.Errorf("message = %q, want %q", is.Message, want)
	}
}

func TestLooksLikeName(t *testing.T) {
	names := []string{
		"PORT", "Path", "MyVar", "http_proxy", "OLD_FLAG", "DATABASE_URL", "AWS_SECRET_ACCESS_KEY",
		"OAUTH2_CLIENT_SECRET", "S3_BUCKET_NAME_2", "npm_config_registry", "PYTHONDONTWRITEBYTECODE",
	}
	for _, k := range names {
		if !looksLikeName(k) {
			t.Errorf("%s should look like a name", k)
		}
	}
	secrets := []string{
		"dGhpc2lzYXZlcnlzZWNyZXR0b2tlbjEyMzQ1", // base64
		"ijklMNOPqrstUVWX0123456789abcd",       // the second half of a wrapped value
		"Ab3_dEf9_GhIjKlMnOp",                  // URL-safe base64
		"JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP",     // base32
		"deadbeef00112233445566778899aabb",     // hex
	}
	for _, k := range secrets {
		if looksLikeName(k) {
			t.Errorf("%s should not look like a name", k)
		}
	}
}

func TestCheckHidesNamesThatLookLikeSecrets(t *testing.T) {
	const token = "dGhpc2lzYXZlcnlzZWNyZXR0b2tlbjEyMzQ1"
	env := mustParse(t, "A=1\n"+token+"==\n"+token+"==")
	issues := Check(env, mustParse(t, "A="), Options{EnvPath: "env", ExamplePath: "ex"})
	var got []string
	for _, is := range issues {
		got = append(got, short(is))
		if strings.Contains(is.Key, token) || strings.Contains(is.Message, token) {
			t.Errorf("issue leaks the token: %+v", is)
		}
	}
	want := []string{"error duplicate dG**** env:3", "warning extra dG**** env:3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
	if !strings.Contains(issues[1].Message, "pasted secret") {
		t.Errorf("message should say why the name is hidden: %q", issues[1].Message)
	}
}

func TestCheckShowsOddNamesTheExampleLists(t *testing.T) {
	const key = "myApplicationDatabaseUrl2"
	issues := Check(mustParse(t, key+"=x\n"+key+"=y"), mustParse(t, key+"="), Options{EnvPath: "env", ExamplePath: "ex"})
	if len(issues) != 1 || issues[0].Key != key {
		t.Fatalf("got %+v, want one issue that names %s", issues, key)
	}
}

func TestMask(t *testing.T) {
	tests := []struct{ in, want string }{
		{"x", "****"},
		{"8080", "****"},
		{"1234567", "****"},
		{"12345678", "12****"},
		{"sk_live_51HabcdefGHIJ", "sk****"},
		{"пароль-секрет", "па****"},
	}
	for _, tt := range tests {
		if got := Mask(tt.in); got != tt.want {
			t.Errorf("Mask(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
