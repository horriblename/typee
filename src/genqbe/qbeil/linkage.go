package qbeil

import "strings"

type Linkage struct {
	Type     LinkageFlags
	SecName  string
	SecFlags string
}

type LinkageFlags int

const (
	Export = 1<<iota + 1
	Thread
	Section
)

func (l Linkage) String() string {
	var s strings.Builder

	if l.Type&Export != 0 {
		s.WriteString("export")
	}

	if l.Type&Thread != 0 {
		if s.Len() != 0 {
			s.WriteRune(' ')
		}

		s.WriteString("thread")
	}

	if l.Type&Section != 0 {
		panic("unimplemented: section linkage")
	}

	return s.String()
}
