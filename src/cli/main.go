package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/horriblename/typee/src/genqbe"
	"github.com/horriblename/typee/src/parse"
	"github.com/horriblename/typee/src/solve"

	"modernc.org/libqbe"
)

const helpMain string = `
usage
	
	program <cmd> <args...>

cmd is one of:

	build    Build a program
	run      Run a program
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

func cmdBuild() error {
	outPath := flag.String(flagOut, defaultOut, "")
	outPathLong := flag.String(flagOutLong, defaultOut, "")

	flag.Parse()
	fname := flag.Arg(0)

	var file io.Reader
	var err error
	if fname == "" {
		file = os.Stdin
	} else {
		file, err = os.Open(fname)
		if err != nil {
			return fmt.Errorf("build: %w", err)
		}
	}

	data, err := io.ReadAll(file)
	if err != nil {
		errorf("could not read stdin: %s", err)
		os.Exit(1)
	}

	ast, err := parse.ParseString(string(data))
	if err != nil {
		errorf("could not parse source: %s", err)
		os.Exit(1)
	}

	typ, err := solve.Check(ast)
	if err != nil {
		errorf("during type inference: %s", err)
		os.Exit(1)
	}

	if *outPathLong != defaultOut {
		*outPath = *outPathLong
	}

	qbeFile, err := os.OpenFile(fname+".qbe", os.O_TRUNC|os.O_CREATE|os.O_RDWR, 0o755)
	if err != nil {
		return fmt.Errorf("build: %w", err)
	}
	defer qbeFile.Close()

	genqbe.Gen(qbeFile, typ, ast)
	qbeFile.Seek(0, 0)

	asmFName := fname + ".s"
	asmFile, err := os.OpenFile(asmFName, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0o755)
	if err != nil {
		return fmt.Errorf("build: %w", err)
	}
	defer asmFile.Close()

	qbePath := fname + ".qbe"
	err = libqbe.Main("amd64_sysv", qbePath, qbeFile, asmFile, nil)
	if err != nil {
		return fmt.Errorf("build: %w", err)
	}

	// maybe I should use `as` and `ld` instead? idk
	assembler := exec.Command("gcc", asmFName, "-o", *outPath)
	assembler.Stdout = os.Stdout
	assembler.Stderr = os.Stderr
	err = assembler.Run()
	if err != nil {
		return fmt.Errorf("assemble: %w", err)
	}

	return os.Chmod(*outPath, 0o755)
}

func cmdRun() error {
	err := cmdBuild()
	if err != nil {
		return err
	}

	outPath := flag.Lookup(flagOut).Value.String()

	if len(outPath) > 0 && outPath[0] != '/' && outPath[:2] != "./" {
		outPath = "./" + outPath
	}

	return exec.Command(outPath).Run()
}
