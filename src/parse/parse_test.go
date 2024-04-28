package parse

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	testCases := []struct {
		desc   string
		input  string
		output []Expr
	}{
		{
			desc:  "form",
			input: "(foo bar)",
			output: []Expr{&Form{
				children: []Expr{
					&Symbol{Name: "foo"},
					&Symbol{Name: "bar"},
				},
			}},
		},
		{
			desc:  "def",
			input: "(def foo (x y) (foo x y))",
			output: []Expr{&FuncDef{
				Name: "foo",
				Args: []string{"x", "y"},
				Body: []Expr{&Form{
					children: []Expr{
						&Symbol{"foo"},
						&Symbol{"x"},
						&Symbol{"y"},
					},
				}},
			}},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			got, err := parseString(tC.input)
			if err != nil {
				t.Logf("%#v", got)
				t.Fatal(err)
			}

			if !reflect.DeepEqual(got, tC.output) {
				t.Fatalf("expected output:\n  %#v\n  %#v", tC.output, got)
			}
		})
	}
}
