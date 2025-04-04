package qbeil

import "fmt"

// labels you can jump to @IDENT
type Label struct {
	Name string
}

func (self Label) IL() string {
	return "@" + self.Name
}

func (b *Builder) TempLabel(namePrefix string) Label {
	b.tempID++
	if namePrefix == "" {
		namePrefix = "_tmp_"
	}
	return Label{Name: fmt.Sprintf("%s%d", namePrefix, b.tempID)}
}

// jump if not zero
func (b *Builder) Jnz(val Value, then Label, otherwise Label) {
	b.indented([]byte("jnz "))
	b.Buf.WriteString(val.IL())
	b.Buf.WriteString(", ")
	b.Buf.WriteString(then.IL())
	b.Buf.WriteString(", ")
	b.Buf.WriteString(otherwise.IL())
	b.Buf.WriteString("\n")
}

func (b *Builder) InsertLabel(l Label) {
	b.Buf.Write([]byte(l.IL()))
	b.Buf.WriteByte('\n')
}
