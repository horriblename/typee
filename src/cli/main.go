package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"

	"github.com/chzyer/readline"
	"github.com/horriblename/typee/src/gir"
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

	maybeProfileMem()

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

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
var memprofile = flag.String("memprofile", "", "write memory profile to `file`")

type closer struct{ close func() error }

func (self closer) Close() error { return self.close() }

func maybeProfileCpu() io.Closer {
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		return closer{func() error {
			pprof.StopCPUProfile()
			return f.Close()
		}}
	}
	return closer{func() error { return nil }}
}

func maybeProfileMem() {
	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal("could not create memory profile: ", err)
		}
		defer f.Close() // error handling omitted for example
		runtime.GC()    // get up-to-date statistics
		// Lookup("allocs") creates a profile similar to go test -memprofile.
		// Alternatively, use Lookup("heap") for a profile
		// that has inuse_space as the default index.
		if err := pprof.Lookup("allocs").WriteTo(f, 0); err != nil {
			log.Fatal("could not write memory profile: ", err)
		}
	}
}

const flagOut = "o"
const flagOutLong = "out"
const defaultOut = "a.out"
const helpOut = "Output file name"

const flagPrintAst = "print-ast"
const defaultPrintAst = false
const helpPrintAst = "Print the ID-annotated AST"

const flagPrintTypes = "print-types"
const defaultPrintTypes = false
const helpPrintTypes = "Print top-level type info to stdout"

const flagPrintTypedTree = "print-typed-tree"
const helpPrintTypedTree = "Print typed-annotated AST"

func cmdCheck() error {
	traceTyper := flag.Bool(flagTraceTyper, false, helpTraceTyper)
	printAst := flag.Bool(flagPrintAst, defaultPrintAst, helpPrintAst)
	printTypes := flag.Bool(flagPrintTypes, defaultPrintTypes, helpPrintTypes)
	printTypedTree := flag.Bool(flagPrintTypedTree, false, helpPrintTypedTree)
	flag.Parse()
	prof := maybeProfileCpu()
	defer prof.Close()

	if *traceTyper {
		simplesub.EnableTrace = true
	}

	params := buildParams{
		targetStage:    check,
		inFile:         flag.Arg(0),
		outFile:        "",
		printAst:       *printAst,
		printTypes:     *printTypes,
		printTypedTree: *printTypedTree,
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

const flagStdlibPath = "stdlib-path"
const helpStdlibPath = "Path to stdlib implementation C file. Use -embedded-stdlib if you don't have a copy of the stdlib source code"

const flagEmbeddedStdlib = "embedded-stdlib"
const helpEmbeddedStdlib = `Use embedded stdlib: debug symbols will not work. If enabled, -stdlib-path is ignored`

func cmdBuild() error {
	outPath := flag.String(flagOut, defaultOut, helpOut)
	outPathLong := flag.String(flagOutLong, defaultOut, helpOut)
	assemblerFlags := flag.String(flagAssemblerFlags, "", helpAssemblerFlags)
	linkerFlags := flag.String(flagLinkerFlags, "", helpLinkerFlags)
	logLevel := flag.Int(flagLogLevel, int(slog.LevelInfo.Level()), helpLogLevel)
	printAst := flag.Bool(flagPrintAst, defaultPrintAst, helpPrintAst)
	printTypes := flag.Bool(flagPrintTypes, defaultPrintTypes, helpPrintTypes)
	printTypedTree := flag.Bool(flagPrintTypedTree, false, helpPrintTypedTree)
	traceTyper := flag.Bool(flagTraceTyper, false, helpTraceTyper)
	externalQbe := flag.Bool(flagExternalQbe, false, helpExternalQbe)
	stdlibPath := flag.String(flagStdlibPath, defaultStdPath, helpStdlibPath)
	embeddedStdlib := flag.Bool(flagEmbeddedStdlib, false, helpEmbeddedStdlib)

	if *outPathLong != defaultOut {
		*outPath = *outPathLong
	}

	flag.Parse()
	prof := maybeProfileCpu()
	defer prof.Close()

	asmFlags := strings.Fields(*assemblerFlags)
	ldFlags := strings.Fields(*linkerFlags)

	params := buildParams{
		targetStage:    build,
		inFile:         flag.Arg(0),
		outFile:        *outPath,
		printAst:       *printAst,
		printTypes:     *printTypes,
		printTypedTree: *printTypedTree,
		assemblerFlags: asmFlags,
		linkerFlags:    ldFlags,
		traceTyper:     *traceTyper,
		externalQbe:    *externalQbe,
		stdlibPath:     *stdlibPath,
		embeddedStdlib: *embeddedStdlib,
		logLevel:       slog.Level(*logLevel),
	}

	return buildProgram(params)
}

func cmdRun() error {
	outPath := flag.String(flagOut, defaultOut, helpOut)
	outPathLong := flag.String(flagOutLong, defaultOut, helpOut)
	assemblerFlags := flag.String(flagAssemblerFlags, "", helpAssemblerFlags)
	linkerFlags := flag.String(flagLinkerFlags, "", helpLinkerFlags)
	logLevel := flag.Int(flagLogLevel, int(slog.LevelInfo.Level()), helpLogLevel)
	printTypes := flag.Bool(flagPrintTypes, defaultPrintTypes, helpPrintTypes)
	printTypedTree := flag.Bool(flagPrintTypedTree, false, helpPrintTypedTree)
	traceTyper := flag.Bool(flagTraceTyper, false, helpTraceTyper)
	externalQbe := flag.Bool(flagExternalQbe, false, helpExternalQbe)
	stdlibPath := flag.String(flagStdlibPath, defaultStdPath, helpStdlibPath)
	embeddedStdlib := flag.Bool(flagEmbeddedStdlib, false, helpEmbeddedStdlib)

	if *outPathLong != defaultOut {
		*outPath = *outPathLong
	}

	flag.Parse()
	prof := maybeProfileCpu()
	defer prof.Close()

	asmFlags := strings.Fields(*assemblerFlags)
	ldFlags := strings.Fields(*linkerFlags)

	params := buildParams{
		targetStage:    run,
		inFile:         flag.Arg(0),
		outFile:        *outPath,
		printTypes:     *printTypes,
		printTypedTree: *printTypedTree,
		assemblerFlags: asmFlags,
		linkerFlags:    ldFlags,
		traceTyper:     *traceTyper,
		externalQbe:    *externalQbe,
		stdlibPath:     *stdlibPath,
		embeddedStdlib: *embeddedStdlib,
		logLevel:       slog.Level(*logLevel),
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
	prof := maybeProfileCpu()
	defer prof.Close()

	if *traceTyper {
		simplesub.EnableTrace = true
	}
	typer := simplesub.NewTyper("Repl", true)
	state := replState{
		typer:   typer,
		rawType: *rawType,
	}

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

		processInput(state, line)
	}

	return nil
}

const flagGlueConfig = "config"
const helpGlueConfig = "path to config.json"
const flagDbgPrintConfig = "dbg-print-config"
const helpDbgPrintConfig = "for debugging: print config"
const flagAutoOut = "O"
const helpAutoOut = `Infer output file name from input file name, or "out.hor"`
const flagRecursive = "r"
const flagRecursiveLong = "recursive"
const helpRecursive = `Generate recursively to the target directory provided by -o (or current working directory).
-config is treated as a directory where the config file of each module is expected to be at <config>/<Module>/config.json.
`

func cmdGlueGir() error {
	configFile := flag.String(flagGlueConfig, "config.json", helpGlueConfig)
	const helpOut = helpOut + " (default stdout)"
	out := flag.String(flagOut, "", helpOut)
	outLong := flag.String(flagOutLong, "", helpOut)
	autoOut := flag.Bool(flagAutoOut, false, helpAutoOut)
	recursive := flag.Bool(flagRecursive, false, "Alias to -recursive")
	recLong := flag.Bool(flagRecursiveLong, false, helpRecursive)
	dbgConfig := flag.Bool(flagDbgPrintConfig, false, helpDbgPrintConfig)
	flag.Parse()
	prof := maybeProfileCpu()
	defer prof.Close()

	if len(flag.Args()) != 1 {
		errorf("wrong arg count")
		flag.Usage()
		os.Exit(2)
	}

	inputMod := flag.Arg(0)
	if *outLong != "" {
		*out = *outLong
	}
	if *autoOut {
		*out = inputMod + ".hor"
	}
	if *recursive || *recLong {
		if err := gir.GenRecursively(inputMod, "", *configFile, *out); err != nil {
			return err
		}
	} else {
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
		defer file.Close()

		config, err := gir.ParseConfig(file)
		if err != nil {
			return fmt.Errorf("parsing glue config file: %w", err)
		}

		if *dbgConfig {
			errorf("config: %#v", config)
		}

		o, err := gir.Gen(inputMod, "", config)
		if err != nil {
			return err
		}

		_, err = outFile.Write(o)
		if err != nil {
			return err
		}
	}

	return nil
}
