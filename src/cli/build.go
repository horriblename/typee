package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/horriblename/typee/src/fun"
	"github.com/horriblename/typee/src/genqbe"
	"github.com/horriblename/typee/src/parse"
	"github.com/horriblename/typee/src/simplesub"

	"modernc.org/libqbe"
)

type stage int

const (
	check stage = iota
	assemble
	build
	run
)

//go:embed horstd/std.c
var stdCSrc string

type buildParams struct {
	targetStage    stage
	inFile         string
	outFile        string
	assemblerFlags []string
	linkerFlags    []string
	printTypes     bool
	printAst       bool
	printTypeTable bool
	traceTyper     bool
	logLevel       slog.Level
}

func buildProgram(params buildParams) error {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: params.logLevel,
	}))
	slog.SetDefault(logger)

	var file io.Reader
	var err error
	var mainModule string
	if params.inFile == "" {
		file = os.Stdin
		mainModule = "MainStdin"
	} else {
		file, err = os.Open(params.inFile)
		if err != nil {
			return fmt.Errorf("build: %w", err)
		}
		mainModule = path.Base(params.inFile)
		mainModule = strings.TrimRight(mainModule, path.Ext(mainModule))
	}

	if params.traceTyper {
		simplesub.EnableTrace = true
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

	if params.printAst {
		errorf(strings.Join(fun.Map(ast, func(e parse.Expr) string {
			return e.String()
		}), "\n"))
	}

	typer := simplesub.NewTyper(mainModule, true)
	t, modules, err := typer.TypeProgram(ast)
	if err != nil {
		errorf("during type inference: %s", err)
		os.Exit(1)
	}

	if params.printTypeTable {
		errorf(simplesub.DebugTypeTable(modules[mainModule].TypeTree))
	}

	if params.printTypes {
		for i, expr := range ast {
			switch e := expr.(type) {
			case *parse.Set:
				errorf("%s: %s", e.Name, t[i].String())
			case *parse.FuncDef:
				errorf("%s: %s", e.Name, t[i].String())
			case *parse.ObjectTypeDef:
				errorf("%s: %s", e.Name, t[i].String())
			}
		}
	}

	if params.targetStage <= check {
		return nil
	}

	objFiles := make([]string, 0, len(modules))
	for name, mod := range modules {
		file, err := compileUnit(name, mod.Ast, mod.TypeTree, params.assemblerFlags)
		if err != nil {
			return fmt.Errorf("compiling module %s: %s", name, err)
		}
		objFiles = append(objFiles, file)
	}

	stdObj := "std.o"
	stdCompiler := exec.Command("gcc", "-c", "-o", stdObj, "-x", "c", "-")
	stdCompiler.Stdin = bytes.NewBufferString(stdCSrc)
	stdCompiler.Stdout = os.Stdout
	stdCompiler.Stderr = os.Stderr
	if err := stdCompiler.Run(); err != nil {
		return fmt.Errorf("compiling stdlib: %s", err)
	}

	if params.targetStage <= assemble {
		return nil
	}

	objFiles = append(objFiles, stdObj)

	linkerFlags := append(objFiles, "-o", params.outFile)
	linkerFlags = append(linkerFlags, params.linkerFlags...)
	slog.Debug("linker", "args", linkerFlags)
	linker := exec.Command("gcc", linkerFlags...)
	linker.Stdout = os.Stdout
	linker.Stderr = os.Stderr
	err = linker.Run()
	if err != nil {
		return fmt.Errorf("link: %s", err)
	}

	if params.targetStage <= build {
		return nil
	}

	executable := params.outFile
	if len(params.outFile) > 0 && params.outFile[0] != '/' && params.outFile[:2] != "./" {
		executable = "./" + params.outFile
	}

	cmd := exec.Command(executable)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func compileUnit(mod string, ast []parse.Expr, typeTree map[int]simplesub.TypeScheme, assemblerFlags []string) (outFile string, _ error) {
	qbeFile, err := os.OpenFile(mod+".qbe", os.O_TRUNC|os.O_CREATE|os.O_RDWR, 0o755)
	if err != nil {
		return "", fmt.Errorf("building qbe IL file: %w", err)
	}
	defer qbeFile.Close()

	genqbe.Gen(qbeFile, mod, typeTree, ast)
	qbeFile.Seek(0, 0)

	asmFName := mod + ".s"
	asmFile, err := os.OpenFile(asmFName, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0o755)
	if err != nil {
		return "", fmt.Errorf("creating asm file: %w", err)
	}
	defer asmFile.Close()

	qbePath := mod + ".qbe"
	err = libqbe.Main("amd64_sysv", qbePath, qbeFile, asmFile, nil)
	if err != nil {
		return "", fmt.Errorf("compiling qbe to asm file: %w", err)
	}

	// maybe I should use `as` and `ld` instead? idk
	objFName := mod + ".o"
	assemblerArgs := append(assemblerFlags, "-c", asmFName, "-o", objFName)
	slog.Debug("assembler", "args", assemblerArgs)
	assembler := exec.Command("gcc", assemblerArgs...)
	assembler.Stdout = os.Stdout
	assembler.Stderr = os.Stderr
	err = assembler.Run()
	if err != nil {
		return "", fmt.Errorf("assemble: %w", err)
	}

	return objFName, nil
}
