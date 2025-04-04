package genqbe

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/can"
	"github.com/horriblename/typee/src/parse"
	"github.com/horriblename/typee/src/simplesub"
)

func TestGen(t *testing.T) {
	entries, err := fs.ReadDir(os.DirFS("."), "tests")
	assert.Ok(err)

	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".hor") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".hor")

		t.Run(name, func(t *testing.T) {
			assert := assert.NewTestAsserts(t)
			src, err := os.ReadFile("tests/" + entry.Name())
			assert.Ok(err)

			if bytes.HasPrefix(src, []byte(";skip\n")) {
				t.Skipf("skipping %s: skip directive found", name)
				return
			}

			expectFile := path.Join("tests", name+".qbe")
			e, err := os.ReadFile(expectFile)
			assert.Ok(err)
			expect := string(e)

			program, err := parse.ParseString(string(src))
			assert.Ok(err)

			mod := can.ModuleName("MainModule")
			typer := simplesub.NewTyper(mod, false)
			_, types, err := typer.TypeProgram(program)
			assert.Ok(err)

			var buf bytes.Buffer
			Gen(&buf, mod, types, program, types[mod].TypesAst)

			got := buf.String()
			if got != expect {
				gotFile := path.Join(t.TempDir(), "got.txt")
				assert.Ok(os.WriteFile(gotFile, []byte(got), 0o644))
				diff := diff(t, expectFile, gotFile)

				if os.Getenv("HOR_FIX_TEST") != "1" {
					t.Errorf("--- diff:\n%s", diff)
				} else {
					t.Errorf("updating test expectations...")
					assert.Ok(os.WriteFile(expectFile, []byte(got), 0o644))
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
