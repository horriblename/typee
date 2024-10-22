package reachable

import (
	"fmt"
	"io"
)

func ExportReachability(r *Reachability, labels []any, w io.Writer) (err error) {
	defer func() {
		er := recover()
		if e, ok := er.(internalError); ok {
			err = e
		} else if er != nil {
			panic(e)
		}
	}()

	checkWrite(w.Write([]byte("digraph Types {\n")))
	checkWrite(io.WriteString(w, "// Node Labels\n"))

	for id, ty := range labels {
		checkWrite(fmt.Fprintf(w, "%d [label=<<i>#%d %#v</i>>]\n", id, id, ty))
	}

	checkWrite(io.WriteString(w, "// Graph\n"))
	for from, downSet := range r.downSets {
		for _, to := range downSet.slice() {
			checkWrite(fmt.Fprintf(w, "%d -> %d\n", from, to))
		}
	}

	checkWrite(w.Write([]byte("}\n")))

	return nil
}

type internalError error

func checkWrite(_ int, err error) {
	if err != nil {
		panic(internalError(err))
	}
}
