package genqbe

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"testing"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/parse"
	"github.com/horriblename/typee/src/simplesub"
)

func TestGen(t *testing.T) {
	testCases := []struct {
		desc   string
		input  string
		output string
	}{
		{
			desc:  "idk",
			input: "(def foo [x y] (+ x (+ y 1)))",
			output: //
			`type :Str = {l, l, }
type :GObject = {l, l, l, }
` + builtinsQbe + `function l $foo(l %x, l %y) {
@start
	%_tmp_1 =l add %y, 1
	%_tmp_2 =l add %x, %_tmp_1
	ret %_tmp_2
}
`,
		},
		{
			desc: "class construction + type annotated method",
			input: `
				(class Foo {pub (def name (Str) [] "foo")})
				(def bar (Foo Str) [foo] (foo#name))
				(def main []
					(print (bar (Foo.new))))
			`,
			output: //
			`type :Str = {l, l, }
type :GObject = {l, l, l, }
type :Foo = {:GObject, l, }
data $_tmp_1 = {b "foo"}
function w $print(:Str %s) {
@start
	%str_data =l loadl %s
	# 64-bit architecture only lul
	%len_loc =l add %s, 8
	%str_len =w loadw %len_loc
	%stdout =l loadl $stdout
	%res =w call $fwrite(l %str_data, w 1, w %str_len, l %stdout)
	ret 0
}

function :Str $Foo_name() {
@start
	%_tmp_2 =l alloc4 24
	storel $_tmp_1, %_tmp_2
	%_tmp_3 =l add %_tmp_2, 8
	storel 3, %_tmp_3
	ret %_tmp_2
}
function :Str $bar(l %foo) {
@start
	%_tmp_4 =:Str call $Foo_name ()
	ret %_tmp_4
}
export function w $main() {
@start
	%_tmp_6 =l call $malloc (l 32)
	%_tmp_5 =:Str call $bar (l %_tmp_6)
	%_tmp_7 =w call $print (:Str %_tmp_5)
	ret %_tmp_7
}
`,
		},
		{
			desc: "class field access",
			input: `(class Foo {pub x Int})
				(def getX (Foo Int) [foo] foo.x)
			`,
			output: `type :Str = {l, l, }
type :GObject = {l, l, l, }
type :Foo = {:GObject, l, l, }
function w $print(:Str %s) {
@start
	%str_data =l loadl %s
	# 64-bit architecture only lul
	%len_loc =l add %s, 8
	%str_len =w loadw %len_loc
	%stdout =l loadl $stdout
	%res =w call $fwrite(l %str_data, w 1, w %str_len, l %stdout)
	ret 0
}

function l $getX(l %foo) {
@start
	%_tmp_1 =l add %foo, 24
	%_tmp_2 =l loadl %_tmp_1
	ret %_tmp_2
}
`,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			assert := assert.NewTestAsserts(t)
			program, err := parse.ParseString(tC.input)
			assert.Ok(err)

			typer := simplesub.NewTyper(false)
			_, types, err := typer.TypeProgram(program)
			assert.Ok(err)

			var buf bytes.Buffer
			Gen(&buf, types, program)

			got := buf.String()
			if got != tC.output {
				t.Log("--- expected:")
				t.Log(tC.output)
				t.Log("--- got:")
				t.Log(got)
				t.Errorf("--- diff:\n%s", diffStr(t, tC.output, got))
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
