// envcheck checks a .env file against .env.example: missing keys, keys the
// example does not know about, empty values and duplicate keys.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the whole program apart from os.Exit, so tests can call it.
// It returns 0 when there are no errors, 1 when the check finds errors and
// 2 when the files cannot be read or parsed.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("envcheck", flag.ContinueOnError)
	fs.SetOutput(stderr)
	envPath := fs.String("env", ".env", "path to the .env file to check")
	examplePath := fs.String("example", ".env.example", "path to the example file that lists the expected keys")
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

	issues := Check(env, example, Options{EnvPath: *envPath, ExamplePath: *examplePath})
	for _, is := range issues {
		fmt.Fprintf(stdout, "%s:%d: %s: %s\n", is.File, is.Line, is.Severity, is.Message)
	}

	errs, warns := Count(issues)
	if errs == 0 && warns == 0 {
		fmt.Fprintf(stderr, "ok: %s checked\n", plural(len(lastByKey(example)), "key"))
	} else {
		fmt.Fprintf(stderr, "%s, %s\n", plural(errs, "error"), plural(warns, "warning"))
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
