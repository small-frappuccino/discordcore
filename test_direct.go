package discordcore

type IDirect struct{ p []byte }

func (i IDirect) All(y func(int) bool) { y(len(i.p)) }
func FDirect() IDirect                 { return IDirect{p: []byte("hello")} }

func RunDirect() {
	FDirect().All(func(x int) bool { return true })
}
