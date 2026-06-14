package main

import (
	"io/ioutil"
	"regexp"
)

func main() {
	content, err := ioutil.ReadFile("pkg/automod/automod_fallback_dedup_test.go")
	if err != nil {
		panic(err)
	}

	re := regexp.MustCompile(`(NewAutomodService\(nil, nil, "[^"]*"), "[^"]*"\)`)
	content = re.ReplaceAll(content, []byte("$1)"))

	ioutil.WriteFile("pkg/automod/automod_fallback_dedup_test.go", content, 0644)
}
