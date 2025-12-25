package genqbe

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"testing"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/can"
	"github.com/horriblename/typee/src/opt"
	"github.com/horriblename/typee/src/parse"
	"github.com/horriblename/typee/src/simplesub"
)

func compileStdC(t *testing.T) (objPath string) {
	const STD_SRC = "../cli/horstd/std.c"
	stdCOut := t.TempDir() + "/std.o"
	t.Helper()

	log, err := exec.Command("gcc", "-g", "-c", "-o", stdCOut, STD_SRC).CombinedOutput()
	if err != nil {
		t.Error(string(log))
		t.Fatal(err)
	}
	return stdCOut
}

func compileQbe(stdCPath string, dir string, qbePath string) (string, error) {
	fname := path.Base(qbePath)
	asmFile := path.Join(dir, fname+".s")
	c := exec.Command("qbe", qbePath, "-o", asmFile)
	if output, err := c.CombinedOutput(); err != nil {
		return "", fmt.Errorf("running qbe: %w\noutput:\n%s", err, output)
	}

	binaryFile := path.Join(dir, fname+".out")
	c = exec.Command("gcc", asmFile, stdCPath, "-o", binaryFile)
	if output, err := c.CombinedOutput(); err != nil {
		return "", fmt.Errorf("assemble: %w\noutput:\n%s", err, output)
	}
	return binaryFile, nil
}

func TestGen(t *testing.T) {
	entries, err := fs.ReadDir(os.DirFS("."), "tests")
	assert.Ok(err)

	stdCPath := compileStdC(t)

	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".hor") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".hor")

		t.Run(name, func(t *testing.T) {
			tempDir := t.TempDir()
			assert := assert.NewTestAsserts(t)
			src, err := os.ReadFile("tests/" + entry.Name())
			assert.Ok(err)

			if bytes.HasPrefix(src, []byte(";skip\n")) {
				t.Skipf("skipping %s: skip directive found", name)
				return
			}

			var expectExitCode = opt.None[int]()
			if rest, ok := bytes.CutPrefix(src, []byte(";exit=")); ok {
				if num, _, ok := bytes.Cut(rest, []byte("\n")); ok {
					// linux exit codes are 8-bit numbers
					n, err := strconv.ParseInt(string(num), 10, 8)
					if err != nil {
						panic("bad exit code in " + name + ": " + string(num))
					}

					expectExitCode = opt.Some(int(n))
				}
			}

			expectFile := path.Join("tests", name+".qbe")
			e, err := os.ReadFile(expectFile)
			assert.Ok(err)
			expect := string(e)

			program, err := parse.ParseString(string(src))
			assert.Ok(err, "parse error")

			mod := can.ModuleName("MainModule")
			typer := simplesub.NewTyper(mod, false)
			_, types, err := typer.TypeProgram(program)
			assert.Ok(err, "type error")

			var buf bytes.Buffer
			Gen(&buf, mod, types, program, types[mod].TypesAst)

			got := buf.String()
			if got != expect {
				gotFile := path.Join(tempDir, "got.txt")
				assert.Ok(os.WriteFile(gotFile, []byte(got), 0o644))
				diff := diff(t, expectFile, gotFile)

				if os.Getenv("HOR_FIX_TEST") != "1" {
					t.Errorf("--- diff:\n%s", diff)
				} else {
					t.Errorf("updating test expectations...")
					assert.Ok(os.WriteFile(expectFile, []byte(got), 0o644))
				}
			}

			if want, ok := expectExitCode.Unwrap(); ok {
				binary, err := compileQbe(stdCPath, tempDir, expectFile)
				assert.Ok(err)

				c := exec.Command(binary)

				log, err := c.CombinedOutput()
				lines := bytes.SplitAfter(log, []byte("\n"))
				for _, line := range lines {
					t.Logf("%s >\t%s", name, line)
				}

				if err == nil {
					assert.Eq(0, want, "wrong exit code")
				} else if e, ok := err.(*exec.ExitError); ok {
					assert.Eq(e.ExitCode(), want, "wrong exit code")
				} else {
					t.Errorf("unexpected error running binary: %v", err)
				}
			}
		})
	}
}

func diff(t *testing.T, a, b string) string {
	t.Helper()

	out, err := exec.Command("git", "diff", "--color=always", a, b).Output()
	_, exitErr := err.(*exec.ExitError)
	if err != nil && !exitErr {
		t.Fatalf("diffStr: running diff cmd: %s", err.Error())
	}
	return string(out)
}
