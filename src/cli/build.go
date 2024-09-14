package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/horriblename/typee/src/genqbe"
	"github.com/horriblename/typee/src/parse"
	"github.com/horriblename/typee/src/solve"

	"modernc.org/libqbe"
)

type stage int

const (
	check stage = iota
	build
	run
)

type buildParams struct {
	targetStage stage
	inFile      string
	outFile     string
}

func buildProgram(params buildParams) error {
	var file io.Reader
	var err error
	if params.inFile == "" {
		file = os.Stdin
	} else {
		file, err = os.Open(params.inFile)
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

	if params.targetStage <= check {
		return nil
	}

	qbeFile, err := os.OpenFile(params.inFile+".qbe", os.O_TRUNC|os.O_CREATE|os.O_RDWR, 0o755)
	if err != nil {
		return fmt.Errorf("build: %w", err)
	}
	defer qbeFile.Close()

	genqbe.Gen(qbeFile, typ, ast)
	qbeFile.Seek(0, 0)

	asmFName := params.inFile + ".s"
	asmFile, err := os.OpenFile(asmFName, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0o755)
	if err != nil {
		return fmt.Errorf("build: %w", err)
	}
	defer asmFile.Close()

	qbePath := params.inFile + ".qbe"
	err = libqbe.Main("amd64_sysv", qbePath, qbeFile, asmFile, nil)
	if err != nil {
		return fmt.Errorf("build: %w", err)
	}

	// maybe I should use `as` and `ld` instead? idk
	assembler := exec.Command("gcc", asmFName, "-o", params.outFile)
	assembler.Stdout = os.Stdout
	assembler.Stderr = os.Stderr
	err = assembler.Run()
	if err != nil {
		return fmt.Errorf("assemble: %w", err)
	}

	if params.targetStage <= build {
		return nil
	}

	executable := params.outFile
	if len(params.outFile) > 0 && params.outFile[0] != '/' && params.outFile[:2] != "./" {
		executable = "./" + params.outFile
	}

	return exec.Command(executable).Run()
}
