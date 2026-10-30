package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

const badOutput = `testdata/bad.env:3: error: DEBUG should be a boolean like true or false (got "****")
testdata/bad.env:4: error: SECRET_KEY is empty
testdata/bad.env:5: error: ADMIN_EMAIL should be an email address (got "op****")
testdata/bad.env:6: error: LOG_LEVEL should be one of debug, info, warn, error (got "****")
testdata/bad.env:7: warning: OLD_FLAG is not in testdata/app.env.example
testdata/bad.env:8: error: PORT is set more than once (first on line 2)
testdata/bad.env:8: error: PORT should be a whole number (got "****")
testdata/app.env.example:5: error: DATABASE_URL is missing from testdata/bad.env
`

func TestRun(t *testing.T) {
	good := []string{"-env", "testdata/good.env", "-example", "testdata/app.env.example"}
	bad := []string{"-env", "testdata/bad.env", "-example", "testdata/app.env.example"}
	tests := []struct {
		name     string
		args     []string
		code     int
		stdout   string
		inStderr string // "" means stderr must be empty
	}{
		{name: "good file passes", args: good, code: 0, inStderr: "ok: 8 keys checked"},
		{name: "bad file fails", args: bad, code: 1, stdout: badOutput, inStderr: "7 errors, 1 warning"},
		{
			name:     "strict makes extra keys errors",
			args:     append([]string{"-strict"}, bad...),
			code:     1,
			stdout:   strings.Replace(badOutput, "warning: OLD_FLAG", "error: OLD_FLAG", 1),
			inStderr: "8 errors, 0 warnings",
		},
		{
			name:   "quiet prints errors only",
			args:   append([]string{"-quiet"}, bad...),
			code:   1,
			stdout: strings.Replace(badOutput, "testdata/bad.env:7: warning: OLD_FLAG is not in testdata/app.env.example\n", "", 1),
		},
		{name: "quiet and clean prints nothing", args: append([]string{"-quiet"}, good...), code: 0},
		{
			name:     "syntax error",
			args:     []string{"-env", "testdata/syntax.env", "-example", "testdata/app.env.example"},
			code:     2,
			inStderr: "testdata/syntax.env:2: missing closing double quote",
		},
		{
			name:     "missing file",
			args:     []string{"-env", "testdata/does-not-exist.env", "-example", "testdata/app.env.example"},
			code:     2,
			inStderr: "does-not-exist.env",
		},
		{name: "unexpected argument", args: []string{"testdata/good.env"}, code: 2, inStderr: "usage: envcheck"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
			if code != tt.code {
				t.Errorf("exit code %d, want %d (stderr: %s)", code, tt.code, stderr.String())
			}
			if stdout.String() != tt.stdout {
				t.Errorf("stdout:\n%s\nwant:\n%s", stdout.String(), tt.stdout)
			}
			if tt.inStderr == "" && stderr.Len() > 0 {
				t.Errorf("stderr should be empty, got %q", stderr.String())
			}
			if !strings.Contains(stderr.String(), tt.inStderr) {
				t.Errorf("stderr %q does not contain %q", stderr.String(), tt.inStderr)
			}
		})
	}
}

func TestRunJSON(t *testing.T) {
	tests := []struct {
		args                []string
		code                int
		ok                  bool
		errs, warns, issues int
	}{
		{[]string{"-json", "-env", "testdata/good.env", "-example", "testdata/app.env.example"}, 0, true, 0, 0, 0},
		{[]string{"-json", "-env", "testdata/bad.env", "-example", "testdata/app.env.example"}, 1, false, 7, 1, 8},
		{[]string{"-json", "-quiet", "-env", "testdata/bad.env", "-example", "testdata/app.env.example"}, 1, false, 7, 1, 7},
	}
	for _, tt := range tests {
		var stdout, stderr bytes.Buffer
		code := run(tt.args, &stdout, &stderr)
		if code != tt.code {
			t.Errorf("%v: exit code %d, want %d", tt.args, code, tt.code)
		}
		var r report
		if err := json.Unmarshal(stdout.Bytes(), &r); err != nil {
			t.Fatalf("%v: output is not JSON: %v\n%s", tt.args, err, stdout.String())
		}
		if r.OK != tt.ok || r.Errors != tt.errs || r.Warnings != tt.warns || len(r.Issues) != tt.issues {
			t.Errorf("%v: got ok=%v errors=%d warnings=%d issues=%d, want %v %d %d %d",
				tt.args, r.OK, r.Errors, r.Warnings, len(r.Issues), tt.ok, tt.errs, tt.warns, tt.issues)
		}
		if r.Issues == nil {
			t.Errorf("%v: issues should be an empty list, not null", tt.args)
		}
		if stderr.Len() > 0 {
			t.Errorf("%v: unexpected stderr %q", tt.args, stderr.String())
		}
	}
}

// Values from the .env file must never show up in the output, in any mode.
func TestRunNeverPrintsValues(t *testing.T) {
	for _, file := range []string{"testdata/good.env", "testdata/bad.env"} {
		entries, err := ParseFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, mode := range [][]string{nil, {"-json"}, {"-strict"}} {
			var out bytes.Buffer
			run(append(mode, "-env", file, "-example", "testdata/app.env.example"), &out, &out)
			for _, e := range entries {
				// Short values like "1" or "shop" can match by chance.
				if len(e.Value) >= 6 && strings.Contains(out.String(), e.Value) {
					t.Errorf("%s %v: output contains the value of %s", file, mode, e.Key)
				}
			}
		}
	}
}
