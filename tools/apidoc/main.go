// Command apidoc regenerates docs/API.md and docs/openapi.json from internal/api. Run it from the repository root:
//
//	go run ./tools/apidoc
package main

import (
	"fmt"
	"os"

	"ams/internal/apidoc"
)

func main() {
	if err := apidoc.Write("."); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := apidoc.WriteOpenAPI("."); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
