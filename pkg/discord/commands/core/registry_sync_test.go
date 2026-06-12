package core

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
func mockJSONResponse(statusCode int, body interface{}) *http.Response {
	b, _ := json.Marshal(body)
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(string(b))),
		Header:     make(http.Header),
	}
}
func TestCommandManager_GuildScopedSync(t *testing.T) {
	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	session.State.User = &discordgo.User{ID: "bot-user-id"}
	session.Client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Path, "/commands") {
				if req.Method == http.MethodGet {
					return mockJSONResponse(http.StatusOK, []*discordgo.ApplicationCommand{
						{ID: "old-id", Name: "obsolete", Description: "old command to be deleted"},
						{ID: "keep-id", Name: "unchanged", Description: "unchanged"},
					}), nil
				} else if req.Method == http.MethodPut {
					var commands []discordgo.ApplicationCommand
					_ = json.NewDecoder(req.Body).Decode(&commands)
					for i := range commands {
						if commands[i].ID == "" {
							commands[i].ID = "new-id-" + commands[i].Name
						}
					}
					return mockJSONResponse(http.StatusOK, commands), nil

				}
			}
			return mockJSONResponse(http.StatusOK, nil), nil
		}),
	}
	config := files.NewConfigManagerWithStore(&files.MemoryConfigStore{})
	_ = config.AddGuildConfig(files.GuildConfig{
		GuildID:           "guild-1",
		BotInstanceTokens: map[string]files.EncryptedString{"bot": "token"},
	})
	manager := NewCommandManager(session, config)
	checker := NewPermissionChecker(session, config)
	manager.GetRouter().RegisterSlashCommand(testCommand{name: "newcmd"})
	manager.GetRouter().RegisterSlashCommand(testCommand{name: "unchanged"})
	group := NewGroupCommand("group", "desc", checker)
	group.AddSubCommand(testCommand{name: "subcmd"})

	subgroup := NewGroupCommand("subgroup", "desc", checker)
	subgroup.AddSubCommand(testCommand{name: "subsubcmd"})
	group.AddSubCommand(subgroup)
	manager.GetRouter().RegisterSlashCommand(group)
	if !manager.usesGuildScopedSync() {
		t.Fatalf("expected usesGuildScopedSync to be true")
	}
	err = manager.SetupCommands()
	if err != nil {
		t.Fatalf("SetupCommands failed: %v", err)
	}
}
func TestCommandManager_GlobalSync(t *testing.T) {
	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	session.State.User = &discordgo.User{ID: "bot-user-id"}
	session.Client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Path, "/commands") {
				if req.Method == http.MethodGet {
					return mockJSONResponse(http.StatusOK, []*discordgo.ApplicationCommand{}), nil
				} else if req.Method == http.MethodPut {
					var commands []discordgo.ApplicationCommand
					_ = json.NewDecoder(req.Body).Decode(&commands)
					for i := range commands {
						commands[i].ID = "new-id-" + commands[i].Name
					}
					return mockJSONResponse(http.StatusOK, commands), nil
				}
			}
			return mockJSONResponse(http.StatusOK, nil), nil
		}),
	}
	config := files.NewConfigManagerWithStore(&files.MemoryConfigStore{})
	manager := NewCommandManager(session, config)
	manager.GetRouter().RegisterSlashCommand(testCommand{name: "globalcmd"})
	if manager.usesGuildScopedSync() {
		t.Fatalf("expected usesGuildScopedSync to be false")
	}
	err = manager.SetupCommands()
	if err != nil {
		t.Fatalf("SetupCommands failed: %v", err)
	}
}
func TestCommandManager_SyncErrors(t *testing.T) {
	session, _ := discordgo.New("Bot test-token")
	session.State.User = &discordgo.User{ID: "bot-user-id"}
	var getShouldFail bool
	var postShouldFail bool
	var deleteShouldFail bool
	session.Client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Path, "/commands") {
				if req.Method == http.MethodGet {
					if getShouldFail {
						return mockJSONResponse(http.StatusInternalServerError, nil), nil
					}
					return mockJSONResponse(http.StatusOK, []*discordgo.ApplicationCommand{
						{Name: "updatecmd", ID: "id1"},
						{Name: "orphancmd", ID: "id2"},
					}), nil
				} else if req.Method == http.MethodPut {
					if postShouldFail || deleteShouldFail {
						return mockJSONResponse(http.StatusInternalServerError, nil), nil
					}
					var commands []discordgo.ApplicationCommand
					_ = json.NewDecoder(req.Body).Decode(&commands)
					for i := range commands {
						if commands[i].ID == "" {
							commands[i].ID = "new-id-" + commands[i].Name
						}
					}
					return mockJSONResponse(http.StatusOK, commands), nil
				}
			}
			return mockJSONResponse(http.StatusOK, nil), nil
		}),
	}
	config := files.NewConfigManagerWithStore(&files.MemoryConfigStore{})
	manager := NewCommandManager(session, config)

	// register 2 commands: one matches existing ("updatecmd") -> PATCH, one doesn't ("createcmd") -> POST
	manager.GetRouter().RegisterSlashCommand(testCommand{name: "updatecmd"})
	manager.GetRouter().RegisterSlashCommand(testCommand{name: "createcmd"})
	// test GET error
	getShouldFail = true
	_, err := manager.syncCommandScope("g1", map[string]*discordgo.ApplicationCommand{})
	if err == nil {
		t.Fatal("expected GET error")
	}
	getShouldFail = false
	// test POST/PATCH error
	postShouldFail = true

	// Create map for syncing
	desired := map[string]*discordgo.ApplicationCommand{
		"updatecmd": {Name: "updatecmd", Description: "desc changed"}, // triggers update
	}
	_, err = manager.syncCommandScope("g1", desired)
	if err == nil {
		t.Fatal("expected update error")
	}
	desiredCreate := map[string]*discordgo.ApplicationCommand{
		"createcmd": {Name: "createcmd", Description: "desc"}, // triggers create
	}
	_, err = manager.syncCommandScope("g1", desiredCreate)
	if err == nil {
		t.Fatal("expected create error")
	}
	postShouldFail = false
	// 3. Test DELETE error
	deleteShouldFail = true
	// desired is empty, so it will try to delete all by bulk overwriting empty array
	_, err = manager.syncCommandScope("g1", map[string]*discordgo.ApplicationCommand{})
	if err == nil {
		t.Fatalf("delete error should fail bulk overwrite")
	}
}
