package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/chzyer/readline"
	"github.com/horriblename/typee/src/gir"
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
	glue-gir Generate bindings to GIR libraries
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
	case "glue-gir":
		err = cmdGlueGir()
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
const helpOut = "Output file name"

const flagPrintTypes = "print-types"
const defaultPrintTypes = false
const helpPrintTypes = "Print top-level type info to stdout"

const flagPrintTypeTable = "print-type-table"
const flagPrintAst = "print-ast"

func cmdCheck() error {
	traceTyper := flag.Bool(flagTraceTyper, false, helpTraceTyper)
	printTypes := flag.Bool(flagPrintTypes, defaultPrintTypes, helpPrintTypes)
	flag.Parse()

	if *traceTyper {
		simplesub.EnableTrace = true
	}

	params := buildParams{
		targetStage: check,
		inFile:      flag.Arg(0),
		outFile:     "",
		printTypes:  *printTypes,
	}

	return buildProgram(params)
}

const flagAssemblerFlags = "assembler-flags"
const helpAssemblerFlags = "Flags to pass to the assembler"

const flagLinkerFlags = "linker-flags"
const helpLinkerFlags = "Flags to pass to the linker"

const flagLogLevel = "log"
const helpLogLevel = "Log level. Lower means more verbose. -4 for debug logs, 8 for errors only"

const flagExternalQbe = "external-qbe"
const helpExternalQbe = "Use qbe executable from PATH"

func cmdBuild() error {
	outPath := flag.String(flagOut, defaultOut, helpOut)
	outPathLong := flag.String(flagOutLong, defaultOut, helpOut)
	assemblerFlags := flag.String(flagAssemblerFlags, "", helpAssemblerFlags)
	linkerFlags := flag.String(flagLinkerFlags, "", helpLinkerFlags)
	logLevel := flag.Int(flagLogLevel, int(slog.LevelInfo.Level()), helpLogLevel)
	printTypes := flag.Bool(flagPrintTypes, defaultPrintTypes, helpPrintTypes)
	printTypeTable := flag.Bool(flagPrintTypeTable, false, "Print a table of expr ID to type.")
	printAst := flag.Bool(flagPrintAst, false, "Print the parse ast")
	traceTyper := flag.Bool(flagTraceTyper, false, helpTraceTyper)
	externalQbe := flag.Bool(flagExternalQbe, false, helpExternalQbe)

	if *outPathLong != defaultOut {
		*outPath = *outPathLong
	}

	flag.Parse()

	asmFlags := strings.Fields(*assemblerFlags)
	ldFlags := strings.Fields(*linkerFlags)

	params := buildParams{
		targetStage:    build,
		inFile:         flag.Arg(0),
		outFile:        *outPath,
		printTypes:     *printTypes,
		printAst:       *printAst,
		printTypeTable: *printTypeTable,
		assemblerFlags: asmFlags,
		linkerFlags:    ldFlags,
		traceTyper:     *traceTyper,
		externalQbe:    *externalQbe,
		logLevel:       slog.Level(*logLevel),
	}

	return buildProgram(params)
}

func cmdRun() error {
	outPath := flag.String(flagOut, defaultOut, helpOut)
	outPathLong := flag.String(flagOutLong, defaultOut, helpOut)

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
const helpRawType = "Print pre-simplified types"

const flagTraceTyper = "trace-typer"
const helpTraceTyper = "Print the type inference debug trace"

func cmdRepl() error {
	rawType := flag.Bool(flagRawType, false, helpRawType)
	traceTyper := flag.Bool(flagTraceTyper, false, helpTraceTyper)
	flag.Parse()

	if *traceTyper {
		simplesub.EnableTrace = true
	}
	typer := simplesub.NewTyper("Repl", true)

	rl, err := readline.NewEx(&readline.Config{
		Prompt:       "> ",
		HistoryFile:  "/tmp/typee.hist",
		HistoryLimit: 100,
	})
	if err != nil {
		return err
	}

	for {
		line, err := rl.Readline()
		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				break
			} else {
				continue
			}
		} else if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		line = strings.TrimSpace(line)

		expr, err := parse.ParseString(line)
		if err != nil {
			errorf("%s", err.Error())
			continue
		}

		if len(expr) == 0 {
			continue
		}

		ty, _, err := typer.TypeProgram(expr)
		if err != nil {
			errorf("%s", err)
			continue
		}

		if *rawType {
			errorf("pre-simplify: (polymorphic) %s", ty[0].String())
		}

		st, ok := ty[0].(simplesub.SimpleType)
		if !ok {
			st = ty[0].(simplesub.PolymorphicType).Body
		}
		simplified := simplesub.SimplifyType(st)

		if *rawType {
			errorf("pre-coalesce: (polymorphic) %v", simplified)
		}

		simpleTy := simplesub.CoalesceType(simplified)

		errorf(": %s", simpleTy)
	}

	return nil
}

const flagGlueConfig = "config"
const helpGlueConfig = "path to config.json"
const flagDbgPrintConfig = "dbg-print-config"
const helpDbgPrintConfig = "for debugging: print config"

func cmdGlueGir() error {
	configFile := flag.String(flagGlueConfig, "config.json", helpGlueConfig)
	out := flag.String(flagOut, "", helpOut)
	outLong := flag.String(flagOutLong, "", helpOut)
	dbgConfig := flag.Bool(flagDbgPrintConfig, false, helpDbgPrintConfig)
	flag.Parse()

	if len(flag.Args()) != 1 {
		errorf("wrong arg count")
		flag.Usage()
		os.Exit(2)
	}

	if *outLong != "" {
		*out = *outLong
	}
	var outFile *os.File = os.Stdout
	if *out != "" {
		var err error
		outFile, err = os.Create(*out)
		if err != nil {
			return err
		}
	}

	file, err := os.Open(*configFile)
	if err != nil {
		return fmt.Errorf("opening glue config file: %w", err)
	}

	config, err := gir.ParseConfig(file)
	if err != nil {
		return fmt.Errorf("parsing glue config file: %w", err)
	}

	if *dbgConfig {
		errorf("config: %#v", config)
	}

	o, err := gir.New(flag.Arg(0), "", config)
	if err != nil {
		return err
	}

	_, err = outFile.Write(o)
	if err != nil {
		return err
	}

	return nil
}
