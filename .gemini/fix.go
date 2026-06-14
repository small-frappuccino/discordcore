package main

import (
	"io/ioutil"
	"regexp"
)

func main() {
	content, err := ioutil.ReadFile("pkg/discord/qotd/runtime_service_test.go")
	if err != nil {
		panic(err)
	}

	re := regexp.MustCompile(`(NewRuntimeServiceForBot\([^,]+, [^,]+, [^,]+, "[^"]*"), "[^"]*"\)`)
	content = re.ReplaceAll(content, []byte("$1)"))

	re2 := regexp.MustCompile(`(NewRuntimeServiceForBot\([^,]+, [^,]+, [^,]+, [^,]+), "[^"]*"\)`)
	content = re2.ReplaceAll(content, []byte("$1)"))

	ioutil.WriteFile("pkg/discord/qotd/runtime_service_test.go", content, 0644)
}
