package main

import (
	"bytes"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
)

func main() {
	err := filepath.WalkDir("pkg/discord/logging", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		changed := false

		// Replace NewMemberEventService(session, cfg, notifier, store) -> ... store, slog.Default())
		reMember := regexp.MustCompile(`NewMemberEventService\(([^,]+),\s*([^,]+),\s*([^,]+),\s*([^,)]+)\)`)
		if reMember.Match(content) {
			content = reMember.ReplaceAll(content, []byte(`NewMemberEventService($1, $2, $3, $4, slog.Default())`))
			changed = true
		}

		reMessage := regexp.MustCompile(`NewMessageEventService\(([^,]+),\s*([^,]+),\s*([^,]+),\s*([^,)]+)\)`)
		if reMessage.Match(content) {
			content = reMessage.ReplaceAll(content, []byte(`NewMessageEventService($1, $2, $3, $4, slog.Default())`))
			changed = true
		}

		reReaction := regexp.MustCompile(`NewReactionEventService\(([^,]+),\s*([^,]+),\s*([^,)]+)\)`)
		if reReaction.Match(content) {
			content = reReaction.ReplaceAll(content, []byte(`NewReactionEventService($1, $2, $3, slog.Default())`))
			changed = true
		}

		reNotifier := regexp.MustCompile(`NewNotificationSender\(([^,)]+)\)`)
		if reNotifier.Match(content) {
			content = reNotifier.ReplaceAll(content, []byte(`NewNotificationSender($1, slog.Default())`))
			changed = true
		}

		if changed {
			if !bytes.Contains(content, []byte(`"log/slog"`)) {
				// simple inject
				content = bytes.Replace(content, []byte("import ("), []byte("import (\n\t\"log/slog\"\n"), 1)
			}
			os.WriteFile(path, content, 0644)
			log.Printf("Fixed %s", path)
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}
