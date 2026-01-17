package cleanup

import "github.com/bwmarrin/discordgo"

// DeleteMode controls how messages are removed.
type DeleteMode int

const (
	// DeleteModeBulkPreferred uses bulk deletion when possible.
	DeleteModeBulkPreferred DeleteMode = iota
	// DeleteModeSingleOnly deletes each message individually.
	DeleteModeSingleOnly
)

// DeleteOptions configures deletion behavior.
type DeleteOptions struct {
	Mode          DeleteMode
	OnDeleteError func(messageID string, err error)
}

// DeleteMessages removes messages from a channel, returning deleted and failed counts.
func DeleteMessages(session *discordgo.Session, channelID string, messageIDs []string, opts DeleteOptions) (int, int) {
	if session == nil || channelID == "" || len(messageIDs) == 0 {
		return 0, 0
	}

	if opts.Mode == DeleteModeSingleOnly {
		return deleteSingle(session, channelID, messageIDs, opts.OnDeleteError)
	}
	return deleteBulkPreferred(session, channelID, messageIDs, opts.OnDeleteError)
}

func deleteSingle(session *discordgo.Session, channelID string, messageIDs []string, onError func(string, error)) (int, int) {
	deleted := 0
	failed := 0
	for _, id := range messageIDs {
		if id == "" {
			continue
		}
		if err := session.ChannelMessageDelete(channelID, id); err != nil {
			failed++
			if onError != nil {
				onError(id, err)
			}
			continue
		}
		deleted++
	}
	return deleted, failed
}

func deleteBulkPreferred(session *discordgo.Session, channelID string, messageIDs []string, onError func(string, error)) (int, int) {
	deleted := 0
	failed := 0
	for _, chunk := range chunkStrings(messageIDs, 100) {
		if len(chunk) == 0 {
			continue
		}
		if len(chunk) == 1 {
			if err := session.ChannelMessageDelete(channelID, chunk[0]); err != nil {
				failed++
				if onError != nil {
					onError(chunk[0], err)
				}
				continue
			}
			deleted++
			continue
		}
		if err := session.ChannelMessagesBulkDelete(channelID, chunk); err != nil {
			failed += len(chunk)
			if onError != nil {
				for _, id := range chunk {
					onError(id, err)
				}
			}
			continue
		}
		deleted += len(chunk)
	}
	return deleted, failed
}

func chunkStrings(values []string, size int) [][]string {
	if size <= 0 {
		return nil
	}
	var out [][]string
	for len(values) > 0 {
		if len(values) <= size {
			out = append(out, values)
			break
		}
		out = append(out, values[:size])
		values = values[size:]
	}
	return out
}
