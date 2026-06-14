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

	re := regexp.MustCompile(`(BotInstanceTokens:\s*map\[string\]files.EncryptedString\{"main": "[^"]+"\},)`)
	content = re.ReplaceAll(content, []byte("$1\n\t\t\tFeatureRouting: map[string]string{\"qotd\": \"main\"},"))

	ioutil.WriteFile("pkg/discord/qotd/runtime_service_test.go", content, 0644)
}
