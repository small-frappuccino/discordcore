package discordcore

import "iter"

var mockConfigs []int

type mockRepo struct{}

func (m *mockRepo) FetchAll() iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := 0; i < len(mockConfigs); i++ {
			if !yield(mockConfigs[i]) {
				return
			}
		}
	}
}

type Repo interface {
	FetchAll() iter.Seq[int]
}

func Hydrate[R Repo](repo R) int {
	seq := repo.FetchAll()
	sum := 0
	for v := range seq {
		sum += v
	}
	return sum
}

func RunIter() int {
	mockConfigs = []int{1, 2, 3}
	repo := &mockRepo{}
	return Hydrate[*mockRepo](repo)
}
