package main

import (
	"os"

	"github.com/parallelworks/openapi-client-generator/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
