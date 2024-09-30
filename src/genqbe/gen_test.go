package genqbe

import (
	"bytes"
	"testing"

	"github.com/horriblename/typee/src/assert"
	"github.com/horriblename/typee/src/parse"
	"github.com/horriblename/typee/src/solve"
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
			`type :Str = {l, w, }
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

function l $foo(l %x, l %y) {
@start
	%_tmp_1 =l add %y, 1
	%_tmp_2 =l add %x, %_tmp_1
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

			types, err := solve.Check(program)
			assert.Ok(err)

			var buf bytes.Buffer
			Gen(&buf, types, program)

			assert.Eq(buf.String(), tC.output)
		})
	}
}
