// envcheck checks a .env file against .env.example: missing keys, keys the
// example does not know about, empty values, duplicate keys and values that
// do not match the type hints written as comments in the example.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// report is the -json output.
type report struct {
	Env      string  `json:"env"`
	Example  string  `json:"example"`
	OK       bool    `json:"ok"`
	Errors   int     `json:"errors"`
	Warnings int     `json:"warnings"`
	Issues   []Issue `json:"issues"`
}

// run is the whole program apart from os.Exit, so tests can call it.
// It returns 0 when there are no errors, 1 when the check finds errors and
// 2 when the files cannot be read or parsed.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("envcheck", flag.ContinueOnError)
	fs.SetOutput(stderr)
	envPath := fs.String("env", ".env", "path to the .env file to check")
	examplePath := fs.String("example", ".env.example", "path to the example file that lists the expected keys")
	strict := fs.Bool("strict", false, "treat keys that are not in the example as errors")
	asJSON := fs.Bool("json", false, "print the result as JSON")
	quiet := fs.Bool("quiet", false, "print errors only: no warnings and no summary line")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: envcheck [flags]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err == flag.ErrHelp {
		return 0
	} else if err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fs.Usage()
		return 2
	}

	example, err := ParseFile(*examplePath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	env, err := ParseFile(*envPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	issues := Check(env, example, Options{EnvPath: *envPath, ExamplePath: *examplePath, Strict: *strict})
	errs, warns := Count(issues)
	shown := []Issue{}
	for _, is := range issues {
		if !*quiet || is.Severity == SevError {
			shown = append(shown, is)
		}
	}

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		err := enc.Encode(report{
			Env:      *envPath,
			Example:  *examplePath,
			OK:       errs == 0,
			Errors:   errs,
			Warnings: warns,
			Issues:   shown,
		})
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	} else {
		for _, is := range shown {
			fmt.Fprintf(stdout, "%s:%d: %s: %s\n", is.File, is.Line, is.Severity, is.Message)
		}
		switch {
		case *quiet:
		case errs == 0 && warns == 0:
			fmt.Fprintf(stderr, "ok: %s checked\n", plural(len(lastByKey(example)), "key"))
		default:
			fmt.Fprintf(stderr, "%s, %s\n", plural(errs, "error"), plural(warns, "warning"))
		}
	}
	if errs > 0 {
		return 1
	}
	return 0
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
