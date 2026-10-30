// envcheck checks a .env file against .env.example.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	envPath := flag.String("env", ".env", "path to the .env file")
	flag.Parse()

	entries, err := ParseFile(*envPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Printf("%s: %d keys\n", *envPath, len(entries))
}
