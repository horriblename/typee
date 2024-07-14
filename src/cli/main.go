package main

import (
	"fmt"
	"io"
	"os"

	"github.com/horriblename/typee/src/parse"
	"github.com/horriblename/typee/src/solve"
)

func errorf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
}

func main() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		errorf("could not read stdin: %s", err)
		os.Exit(1)
	}

	program, err := parse.ParseString(string(data))
	if err != nil {
		errorf("could not parse source: %s", err)
		os.Exit(1)
	}

	typ, err := solve.Infer(program[0])
	if err != nil {
		errorf("during type inference: %s", err)
		os.Exit(1)
	}

	println(typ.String())
}
