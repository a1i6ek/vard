// Command bundle writes each API description bundled into one JSON document,
// into the directory it is given, for Spectral to lint (make lint-api) and
// openapi-typescript to read (make generate).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/a1i6ek/vard/api/openapi"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: bundle <directory>")
		os.Exit(2)
	}
	if err := os.MkdirAll(os.Args[1], 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, root := range []string{openapi.AccountRoot, openapi.AdminRoot} {
		document, err := openapi.JSON(root)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		name := filepath.Join(os.Args[1], strings.TrimSuffix(root, ".yaml")+".json")
		if err := os.WriteFile(name, document, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
