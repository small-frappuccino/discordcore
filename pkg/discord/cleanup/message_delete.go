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
	Mode DeleteMode
	// OnDeleteError fires once per message that single-delete cannot remove.
	// The classified failure class is also passed so callers can branch
	// without re-running ClassifyDeleteError.
	OnDeleteError func(messageID string, err error, class FailureClass)
	// OnChunkError fires once per bulk-delete chunk that Discord rejected
	// at the chunk level (permission gone, channel gone, rate limited, etc.).
	// Bulk-age rejections (FailureClassBulkDeleteAge) do NOT fire this
	// callback — those are silently retried as single deletes so the count
	// of "failed" messages stays accurate.
	OnChunkError func(messageIDs []string, err error, class FailureClass)
}

// DeleteMessages removes messages from a channel, returning deleted and failed counts.
func DeleteMessages(session *discordgo.Session, channelID string, messageIDs []string, opts DeleteOptions) (int, int) {
	if session == nil || channelID == "" || len(messageIDs) == 0 {
		return 0, 0
	}

	if opts.Mode == DeleteModeSingleOnly {
		return deleteSingle(session, channelID, messageIDs, opts.OnDeleteError)
	}
	return deleteBulkPreferred(session, channelID, messageIDs, opts.OnDeleteError, opts.OnChunkError)
}

func deleteSingle(session *discordgo.Session, channelID string, messageIDs []string, onError func(string, error, FailureClass)) (int, int) {
	deleted := 0
	failed := 0
	for _, id := range messageIDs {
		if id == "" {
			continue
		}
		if err := session.ChannelMessageDelete(channelID, id); err != nil {
			class := ClassifyDeleteError(err)
			// A 404 means the message was already gone — the cleanup goal
			// is satisfied, so do not count it as a failure or report it.
			if class == FailureClassMissingMessage {
				deleted++
				continue
			}
			failed++
			if onError != nil {
				onError(id, err, class)
			}
			continue
		}
		deleted++
	}
	return deleted, failed
}

func deleteBulkPreferred(session *discordgo.Session, channelID string, messageIDs []string, onError func(string, error, FailureClass), onChunkError func([]string, error, FailureClass)) (int, int) {
	deleted := 0
	failed := 0
	for _, chunk := range chunkStrings(messageIDs, 100) {
		if len(chunk) == 0 {
			continue
		}
		if len(chunk) == 1 {
			d, f := deleteSingle(session, channelID, chunk, onError)
			deleted += d
			failed += f
			continue
		}
		if err := session.ChannelMessagesBulkDelete(channelID, chunk); err != nil {
			class := ClassifyDeleteError(err)
			if class == FailureClassBulkDeleteAge {
				// Discord refused the chunk because at least one message
				// crossed the 14-day boundary mid-flight. Retry the chunk
				// as per-message single deletes so we get accurate
				// per-message classification (the rest of the chunk is
				// usually still valid) instead of marking 100 messages
				// failed for one borderline message.
				d, f := deleteSingle(session, channelID, chunk, onError)
				deleted += d
				failed += f
				continue
			}
			failed += len(chunk)
			if onChunkError != nil {
				onChunkError(chunk, err, class)
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
