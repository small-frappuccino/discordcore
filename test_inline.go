package discordcore

type RouterInline struct{}

//go:noinline
func (r *RouterInline) resolve() (string, error) {
	return "ok", nil
}

//go:noinline
func runInline(p []byte, s string, yield func(int) bool) {
	yield(len(p) + len(s))
}

type InlineIterator struct {
	p []byte
	s string
}

func (it InlineIterator) Seq(yield func(int) bool) {
	runInline(it.p, it.s, yield)
}

func (r *RouterInline) ParseInteraction(p []byte) (InlineIterator, error) {
	s, err := r.resolve()
	if err != nil {
		return InlineIterator{}, err
	}
	return InlineIterator{p: p, s: s}, nil
}

func FInline() {
	r := &RouterInline{}
	seq, _ := r.ParseInteraction([]byte("hello"))
	for x := range seq.Seq {
		_ = x
	}
}

func RunInline() {
	FInline()
}
