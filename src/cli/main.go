package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"

	"github.com/horriblename/typee/src/parse"
	"github.com/horriblename/typee/src/simplesub"
)

const helpMain string = `
usage
	
	program <cmd> <args...>

cmd is one of:

	build    Build a program
	run      Run a program
	check    Type check a program
	repl     Start a repl
`

func main() {
	if len(os.Args) < 2 {
		errorf(helpMain)
		os.Exit(2)
	}

	cmd := os.Args[1]
	shiftArgs()

	var err error
	switch cmd {
	case "build":
		err = cmdBuild()
	case "run":
		err = cmdRun()
	case "check":
		err = cmdCheck()
	case "repl":
		err = cmdRepl()
	default:
		errorf("Unknown command: %s", cmd)
		errorf(helpMain)
		os.Exit(2)
	}

	if err != nil {
		errorf("%s", err)
		os.Exit(1)
	}
}

func shiftArgs() {
	if len(os.Args) > 1 {
		program := os.Args[0]
		os.Args = os.Args[1:]
		os.Args[0] = program
	}
}

func errorf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
	fmt.Fprintln(os.Stderr)
}

const flagOut = "o"
const flagOutLong = "out"
const defaultOut = "a.out"

const flagPrintTypes = "print-types"
const defaultPrintTypes = false

func cmdCheck() error {
	printTypes := flag.Bool(flagPrintTypes, defaultPrintTypes, "Print top-level type info to stdout")
	flag.Parse()

	params := buildParams{
		targetStage: check,
		inFile:      flag.Arg(0),
		outFile:     "",
		printTypes:  *printTypes,
	}

	return buildProgram(params)
}

func cmdBuild() error {
	outPath := flag.String(flagOut, defaultOut, "")
	outPathLong := flag.String(flagOutLong, defaultOut, "")

	if *outPathLong != defaultOut {
		*outPath = *outPathLong
	}

	flag.Parse()
	params := buildParams{
		targetStage: build,
		inFile:      flag.Arg(0),
		outFile:     *outPath,
	}

	return buildProgram(params)
}

func cmdRun() error {
	outPath := flag.String(flagOut, defaultOut, "")
	outPathLong := flag.String(flagOutLong, defaultOut, "")

	if *outPathLong != defaultOut {
		*outPath = *outPathLong
	}

	flag.Parse()
	params := buildParams{
		targetStage: run,
		inFile:      flag.Arg(0),
		outFile:     *outPath,
	}

	return buildProgram(params)
}

const flagRawType = "raw-type"
const helpRawType = "print pre-simplified types"

func cmdRepl() error {
	scanner := bufio.NewScanner(os.Stdin)
	typer := simplesub.NewTyper(true)
	rawType := flag.Bool(flagRawType, false, helpRawType)
	flag.Parse()

	for {
		os.Stderr.WriteString("\n> ")
		if ok := scanner.Scan(); !ok {
			break
		}

		expr, err := parse.ParseString(scanner.Text())
		if err != nil {
			errorf("%s", err.Error())
			continue
		}

		if len(expr) == 0 {
			continue
		}

		ty, err := typer.TypeTerm(expr[0])
		if err != nil {
			errorf("%s", err)
			continue
		}

		if *rawType {
			errorf("pre-simplify: %s", ty.String())
		}

		simplified := simplesub.SimplifyType(ty)

		if *rawType {
			errorf("pre-coalesce: %v", simplified)
		}

		simpleTy := simplesub.CoalesceType(simplified)

		errorf(": %s", simpleTy)
	}

	return scanner.Err()
}
