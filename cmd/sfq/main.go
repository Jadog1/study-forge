package main

import (
	"fmt"
	"os"
)

func main() {
	root := Root()
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
