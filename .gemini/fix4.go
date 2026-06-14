package main

import (
	"io/ioutil"
	"regexp"
	"strings"
)

func main() {
	content, err := ioutil.ReadFile("pkg/discord/qotd/runtime_service_test.go")
	if err != nil {
		panic(err)
	}
	text := string(content)

	// Remove defaultBotInstanceID argument
	re := regexp.MustCompile(`(NewRuntimeServiceForBot\([^,]+, [^,]+, [^,]+, "[^"]*"), "[^"]*"\)`)
	text = re.ReplaceAllString(text, "$1)")

	re2 := regexp.MustCompile(`(NewRuntimeServiceForBot\([^,]+, [^,]+, [^,]+, [^,]+), "[^"]*"\)`)
	text = re2.ReplaceAllString(text, "$1)")

	// Remove fallback test entirely
	testStart := strings.Index(text, "func TestRuntimeServiceCyclesUseQOTDDomainScopedGuildsFallback")
	if testStart != -1 {
		testEnd := strings.Index(text[testStart:], "\n}\n")
		if testEnd != -1 {
			text = text[:testStart] + text[testStart+testEnd+3:]
		}
	}

	// Add FeatureRouting
	reTokens := regexp.MustCompile(`(BotInstanceTokens:\s*map\[string\]files.EncryptedString\{"main": "[^"]+"\},)`)
	text = reTokens.ReplaceAllString(text, "$1\n\t\t\tFeatureRouting: map[string]string{\"qotd\": \"main\"},")

	// Same for "companion" test just in case it needs it (but it already has it)
	// Actually, wait, let's fix any "g-fallback-companion" if they remain... but it was removed.

	ioutil.WriteFile("pkg/discord/qotd/runtime_service_test.go", []byte(text), 0644)
}
