package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		code     int
		stdout   string
		inStderr string
	}{
		{
			name:     "good file passes",
			args:     []string{"-env", "testdata/good.env", "-example", "testdata/app.env.example"},
			code:     0,
			inStderr: "ok: 8 keys checked",
		},
		{
			name: "bad file fails",
			args: []string{"-env", "testdata/bad.env", "-example", "testdata/app.env.example"},
			code: 1,
			stdout: `testdata/bad.env:3: error: DEBUG should be a boolean like true or false
testdata/bad.env:4: error: SECRET_KEY is empty
testdata/bad.env:5: error: ADMIN_EMAIL should be an email address
testdata/bad.env:6: error: LOG_LEVEL should be one of debug, info, warn, error
testdata/bad.env:7: warning: OLD_FLAG is not in testdata/app.env.example
testdata/bad.env:8: error: PORT is set more than once (first on line 2)
testdata/bad.env:8: error: PORT should be a whole number
testdata/app.env.example:5: error: DATABASE_URL is missing from testdata/bad.env
`,
			inStderr: "7 errors, 1 warning",
		},
		{
			name:     "syntax error",
			args:     []string{"-env", "testdata/syntax.env", "-example", "testdata/app.env.example"},
			code:     2,
			inStderr: "testdata/syntax.env:2: missing closing double quote",
		},
		{
			name: "missing file",
			args: []string{"-env", "testdata/does-not-exist.env", "-example", "testdata/app.env.example"},
			code: 2,
		},
		{
			name:     "unexpected argument",
			args:     []string{"testdata/good.env"},
			code:     2,
			inStderr: "usage: envcheck",
		},
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
			if !strings.Contains(stderr.String(), tt.inStderr) {
				t.Errorf("stderr %q does not contain %q", stderr.String(), tt.inStderr)
			}
		})
	}
}
