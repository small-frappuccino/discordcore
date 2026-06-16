//go:build integration

package tickets

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/testdb"
	"github.com/small-frappuccino/discordgo"
)

type ticketInteractionRecorder struct {
	mu               sync.Mutex
	responses        []discordgo.InteractionResponse
	channelCreates   []discordgo.GuildChannelCreateData
	channelEdits     []discordgo.ChannelEdit
	messageSends     []*discordgo.MessageSend
	multipartFiles   map[string][]byte
	mockChannelCount int
}

func (r *ticketInteractionRecorder) addResponse(resp discordgo.InteractionResponse) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.responses = append(r.responses, resp)
}

func (r *ticketInteractionRecorder) addChannelCreate(data discordgo.GuildChannelCreateData) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.channelCreates = append(r.channelCreates, data)
}

func (r *ticketInteractionRecorder) addChannelEdit(data discordgo.ChannelEdit) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.channelEdits = append(r.channelEdits, data)
}

func (r *ticketInteractionRecorder) addMessageSend(data *discordgo.MessageSend) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messageSends = append(r.messageSends, data)
}

func (r *ticketInteractionRecorder) saveMultipartFile(filename string, content []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.multipartFiles == nil {
		r.multipartFiles = make(map[string][]byte)
	}
	r.multipartFiles[filename] = content
}

type ticketCommandTestHarness struct {
	session *discordgo.Session
	rec     *ticketInteractionRecorder
	svc     *TicketService
	cm      *files.ConfigManager
	store   *storage.Store
	guildID string
	ownerID string
}

func newTicketCommandTestHarness(t *testing.T, guildID, ownerID string) *ticketCommandTestHarness {
	t.Helper()

	session, rec := newTicketCommandTestSession(t)

	// Create isolated DB
	baseDSN, err := testdb.BaseDatabaseURLFromEnv()
	if err != nil {
		if testdb.IsDatabaseURLNotConfigured(err) {
			t.Skipf("skipping postgres integration test: %v", err)
		}
		t.Fatalf("resolve test database dsn: %v", err)
	}
	db, cleanup, err := testdb.OpenIsolatedDatabase(context.Background(), baseDSN)
	if err != nil {
		t.Fatalf("open isolated test database: %v", err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Fatalf("cleanup isolated test database: %v", err)
		}
	})

	store, err := storage.NewStore(db)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	if err := store.Init(); err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	if err := cm.AddGuildConfig(files.GuildConfig{
		GuildID: guildID,
		Tickets: files.TicketsConfig{
			Enabled:             true,
			TranscriptChannelID: "audit-123",
			Categories: []files.TicketsCategoryConfig{
				{Name: "Contact Staff", RoleID: "role-staff"},
				{Name: "Contact Admins", RoleID: "role-admins"},
			},
		},
	}); err != nil {
		t.Fatalf("failed to add guild config: %v", err)
	}

	if err := session.State.GuildAdd(&discordgo.Guild{ID: guildID, OwnerID: ownerID}); err != nil {
		t.Fatalf("failed to add guild to state: %v", err)
	}

	// Add a dummy ticket channel for close/reopen testing
	if err := session.State.ChannelAdd(&discordgo.Channel{
		ID:      "ticket-123-id",
		GuildID: guildID,
		Name:    "ticket-0001",
		PermissionOverwrites: []*discordgo.PermissionOverwrite{
			{
				ID:    "user-1",
				Type:  discordgo.PermissionOverwriteTypeMember,
				Allow: discordgo.PermissionViewChannel | discordgo.PermissionSendMessages,
			},
		},
	}); err != nil {
		t.Fatalf("failed to add channel to state: %v", err)
	}

	if err := session.State.ChannelAdd(&discordgo.Channel{
		ID:      "closed-123-id",
		GuildID: guildID,
		Name:    "closed-0002",
		PermissionOverwrites: []*discordgo.PermissionOverwrite{
			{
				ID:    "user-1",
				Type:  discordgo.PermissionOverwriteTypeMember,
				Allow: discordgo.PermissionViewChannel,
				Deny:  discordgo.PermissionSendMessages,
			},
		},
	}); err != nil {
		t.Fatalf("failed to add closed channel to state: %v", err)
	}

	svc := NewTicketService(store)
	return &ticketCommandTestHarness{
		session: session,
		rec:     rec,
		svc:     svc,
		cm:      cm,
		store:   store,
		guildID: guildID,
		ownerID: ownerID,
	}
}

func newTicketCommandTestSession(t *testing.T) (*discordgo.Session, *ticketInteractionRecorder) {
	t.Helper()

	rec := &ticketInteractionRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.Path

		if strings.Contains(path, "/callback") || strings.HasSuffix(path, "/messages/@original") {
			var resp discordgo.InteractionResponse
			_ = json.NewDecoder(req.Body).Decode(&resp)
			rec.addResponse(resp)
			w.WriteHeader(http.StatusOK)
			return
		}

		if strings.Contains(path, "/channels") && req.Method == http.MethodGet && strings.Contains(path, "/guilds/") {
			rec.mu.Lock()
			count := rec.mockChannelCount
			rec.mu.Unlock()
			if count == 0 {
				count = 2
			}
			ch := make([]discordgo.Channel, count)
			for i := 0; i < count; i++ {
				ch[i] = discordgo.Channel{ID: fmt.Sprintf("chan-%d", i), Name: "general"}
			}
			json.NewEncoder(w).Encode(ch)
			return
		}

		if strings.Contains(path, "/channels") && req.Method == http.MethodPost && !strings.Contains(path, "/messages") {
			// Channel Create
			var data discordgo.GuildChannelCreateData
			_ = json.NewDecoder(req.Body).Decode(&data)
			rec.addChannelCreate(data)

			// Return a dummy channel
			ch := discordgo.Channel{ID: "new-channel-id", Name: data.Name}
			json.NewEncoder(w).Encode(ch)
			return
		}

		if strings.Contains(path, "/channels") && req.Method == http.MethodPatch {
			// Channel Edit
			var data discordgo.ChannelEdit
			_ = json.NewDecoder(req.Body).Decode(&data)
			rec.addChannelEdit(data)
			ch := discordgo.Channel{ID: "edited-channel-id", Name: data.Name}
			json.NewEncoder(w).Encode(ch)
			return
		}

		if strings.Contains(path, "/permissions/") && req.Method == http.MethodPut {
			// Permission edit
			w.WriteHeader(http.StatusOK)
			return
		}

		if strings.HasSuffix(path, "/messages") && req.Method == http.MethodPost {
			contentType := req.Header.Get("Content-Type")
			if strings.HasPrefix(contentType, "multipart/form-data") {
				// We need to parse multipart
				reader, err := req.MultipartReader()
				if err == nil {
					for {
						part, err := reader.NextPart()
						if err == io.EOF {
							break
						}
						if err != nil {
							continue
						}
						if part.FileName() != "" {
							content, _ := io.ReadAll(part)
							rec.saveMultipartFile(part.FileName(), content)
						}
					}
				}
				rec.addMessageSend(&discordgo.MessageSend{Content: "multipart"})
				json.NewEncoder(w).Encode(discordgo.Message{ID: "msg-1"})
				return
			}

			var data discordgo.MessageSend
			_ = json.NewDecoder(req.Body).Decode(&data)
			rec.addMessageSend(&data)
			json.NewEncoder(w).Encode(discordgo.Message{ID: "msg-1"})
			return
		}

		if strings.HasSuffix(path, "/messages") && req.Method == http.MethodGet {
			// Return some dummy messages for transcript fetching
			msgs := []discordgo.Message{
				{ID: "msg-1", Content: "Hello", Author: &discordgo.User{ID: "u1", Username: "User 1"}},
				{ID: "msg-2", Content: "World", Author: &discordgo.User{ID: "u2", Username: "User 2"}},
			}
			// Only return them once to not loop infinitely (if beforeID is empty)
			if req.URL.Query().Get("before") == "" {
				json.NewEncoder(w).Encode(msgs)
			} else {
				json.NewEncoder(w).Encode([]discordgo.Message{})
			}
			return
		}

		if req.Method == http.MethodDelete {
			json.NewEncoder(w).Encode(discordgo.Channel{ID: "deleted-channel"})
			return
		}

		// Fallback for channels list
		if req.Method == http.MethodGet && strings.HasSuffix(path, "/channels") {
			json.NewEncoder(w).Encode([]discordgo.Channel{})
			return
		}

		// Fetch specific channel
		if req.Method == http.MethodGet && strings.Contains(path, "/channels/") && !strings.Contains(path, "/messages") {
			chID := path[strings.LastIndex(path, "/")+1:]
			name := "ticket-0001"
			if strings.HasPrefix(chID, "closed-") {
				name = "closed-0002"
			} else if !strings.HasPrefix(chID, "ticket-") && !strings.HasPrefix(chID, "closed-") {
				// E.g. chan-1
				name = "general"
			}
			json.NewEncoder(w).Encode(discordgo.Channel{
				ID:   chID,
				Name: name,
				PermissionOverwrites: []*discordgo.PermissionOverwrite{
					{
						ID:    "user-1",
						Type:  discordgo.PermissionOverwriteTypeMember,
						Allow: discordgo.PermissionViewChannel | discordgo.PermissionSendMessages,
					},
				},
			})
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	oldAPI := discordgo.EndpointAPI
	oldChannels := discordgo.EndpointChannels
	oldWebhooks := discordgo.EndpointWebhooks
	oldGuilds := discordgo.EndpointGuilds
	discordgo.EndpointAPI = server.URL + "/"
	discordgo.EndpointChannels = discordgo.EndpointAPI + "channels/"
	discordgo.EndpointWebhooks = discordgo.EndpointAPI + "webhooks/"
	discordgo.EndpointGuilds = discordgo.EndpointAPI + "guilds/"
	t.Cleanup(func() { discordgo.EndpointAPI = oldAPI })
	t.Cleanup(func() { discordgo.EndpointChannels = oldChannels })
	t.Cleanup(func() { discordgo.EndpointWebhooks = oldWebhooks })
	t.Cleanup(func() { discordgo.EndpointGuilds = oldGuilds })

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("failed to create discord session: %v", err)
	}
	return session, rec
}

func newTicketInteraction(
	guildID, userID, channelID string,
	customID string,
	values []string,
) *discordgo.InteractionCreate {
	var compType discordgo.ComponentType = discordgo.ButtonComponent
	if len(values) > 0 {
		compType = discordgo.SelectMenuComponent
	}

	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:        "interaction-" + customID,
			AppID:     "app",
			Token:     "token",
			Type:      discordgo.InteractionMessageComponent,
			GuildID:   guildID,
			ChannelID: channelID,
			Member:    &discordgo.Member{User: &discordgo.User{ID: userID}},
			Data: discordgo.MessageComponentInteractionData{
				CustomID:      customID,
				ComponentType: compType,
				Values:        values,
			},
		},
	}
}
