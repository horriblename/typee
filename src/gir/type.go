package gir

type typeFlags int

const (
	typeNone    typeFlags = 0
	typePointer typeFlags = 1 << iota
	typeReturn
	typeListMember
	typeReceiver
	typeExact
)
