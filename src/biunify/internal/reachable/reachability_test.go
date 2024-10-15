package reachable

import (
	"testing"

	"github.com/horriblename/typee/src/assert"
)

func TestReachability(t *testing.T) {
	testCases := []struct {
		desc    string
		input   []Edge
		asserts []Edge
	}{
		{
			desc:    "basic",
			input:   []Edge{{0, 1}},
			asserts: []Edge{{0, 1}},
		},
		{
			desc:    "three nodes",
			input:   []Edge{{0, 1}, {2, 0}},
			asserts: []Edge{{0, 1}, {2, 0}, {2, 1}},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			_ = assert.NewTestAsserts(t)
			r := Reachability{}
			dummy := []Edge{}
			for _, edge := range tC.input {
				for edge.From >= len(r.upSets) {
					r.AddNode()
				}
				for edge.To >= len(r.upSets) {
					r.AddNode()
				}

				r.AddEdge(edge.From, edge.To, &dummy)
				t.Logf("adding edge %v: got transitive edges: %v", edge, dummy)
				dummy = []Edge{}
			}

			for _, a := range tC.asserts {
				if !r.downSets[a.From].has(a.To) {
					t.Errorf("node %d does not flow to %d", a.From, a.To)
					t.Logf("down sets of %d: %v", a.From, r.downSets[a.From])
				}
			}
		})
	}
}
