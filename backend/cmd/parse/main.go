package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/harsh3dev/multi-resume/backend/internal/checks"
	"github.com/harsh3dev/multi-resume/backend/internal/compile"
	"github.com/harsh3dev/multi-resume/backend/internal/parser"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: parse <file.tex>")
		os.Exit(2)
	}
	stat, err := os.Stat(os.Args[1])
	if err != nil || stat.Size() > compile.MaxTexBytes {
		fmt.Fprintln(os.Stderr, "cannot read source")
		os.Exit(1)
	}
	src, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot read source")
		os.Exit(1)
	}
	tree, err := parser.Parse(string(src), parser.ParseOptions{})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	failures := checks.CheckRoundTrip(string(src), tree)
	result := struct {
		Tree              any      `json:"tree"`
		Report            any      `json:"report"`
		RoundTripFailures []string `json:"roundTripFailures"`
	}{tree, tree.Report, failures}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, "cannot write result")
		os.Exit(1)
	}
}
