package main

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

func testMain() int {
	mockConfigs = []int{1, 2, 3}
	repo := &mockRepo{}
	// Force inlining? No compiler directive for forcing inline at call site in standard Go.
	// But what if we just use a small enough function?
	return Hydrate[*mockRepo](repo)
}
