package discordcore

type IEscape struct {
	p []byte
}

func (i IEscape) Seq(y func(int) bool) {
}

func FEscape() IEscape {
	return IEscape{p: []byte("hello")}
}

func RunEscape() {
	for x := range FEscape().Seq {
		_ = x
	}
}
