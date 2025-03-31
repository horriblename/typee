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

			e, err := os.ReadFile("tests/" + name + ".qbe")
			assert.Ok(err)
			expect := string(e)

			program, err := parse.ParseString(string(src))
			assert.Ok(err)

			mod := simplesub.CanonName("MainModule")
			typer := simplesub.NewTyper(mod, false)
			_, types, err := typer.TypeProgram(program)
			assert.Ok(err)

			var buf bytes.Buffer
			Gen(&buf, mod, types, program)

			got := buf.String()
			if got != expect {
				// t.Log("--- expected:")
				// t.Log(expect)
				// t.Log("--- got:")
				// t.Log(got)
				t.Errorf("--- diff:\n%s", diffStr(t, string(expect), got))
			}
		})
	}
}

func diffStr(t *testing.T, a, b string) string {
	t.Helper()
	dir := t.TempDir()
	pathA := path.Join(dir, "a")
	pathB := path.Join(dir, "b")
	if err := os.WriteFile(pathA, []byte(a), 0o644); err != nil {
		t.Fatalf("diffStr: %s", err.Error())
	}
	if err := os.WriteFile(pathB, []byte(b), 0o644); err != nil {
		t.Fatalf("diffStr: %s", err.Error())
	}

	out, err := exec.Command("git", "diff", "--color=always", pathA, pathB).Output()
	_, exitErr := err.(*exec.ExitError)
	if err != nil && !exitErr {
		t.Fatalf("diffStr: running diff cmd: %s", err.Error())
	}
	return string(out)
}
