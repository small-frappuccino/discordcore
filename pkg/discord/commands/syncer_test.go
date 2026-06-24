package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/httputil"
	"github.com/diamondburned/arikawa/v3/utils/httputil/httpdriver"
	"github.com/stretchr/testify/require"
)

// Mock Types
type mockCommand struct {
	name string
	desc string
}

func (m *mockCommand) Name() string                     { return m.name }
func (m *mockCommand) Description() string              { return m.desc }
func (m *mockCommand) Options() []discord.CommandOption { return nil }
func (m *mockCommand) Handle(ctx *ArikawaContext) error { return nil }
func (m *mockCommand) RequiresGuild() bool              { return false }
func (m *mockCommand) RequiresPermissions() bool        { return false }

type mockCommandWithPerms struct {
	mockCommand
	perms discord.Permissions
}

func (m *mockCommandWithPerms) DefaultMemberPermissions() discord.Permissions {
	return m.perms
}

type mockTransport struct {
	roundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTripFunc(req)
}

func newMockArikawaClient(transport http.RoundTripper) *api.Client {
	c := api.NewClient("Bot mock_token")
	c.Client.Client = httpdriver.WrapClient(http.Client{Transport: transport})
	return c
}

// 1. Testes de Mapeamento e Type Assertions
func TestCommandSyncer_BuildCreateData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cmd      ArikawaCommand
		validate func(t *testing.T, data api.CreateCommandData)
	}{
		{
			name: "Cenário B (Fallback/Omissão)",
			cmd:  &mockCommand{name: "basic", desc: "basic cmd"},
			validate: func(t *testing.T, data api.CreateCommandData) {
				require.Equal(t, "basic", data.Name)
				require.Nil(t, data.DefaultMemberPermissions, "expected nil permissions for basic command")
			},
		},
		{
			name: "Cenário A (Implementação Completa)",
			cmd: &mockCommandWithPerms{
				mockCommand: mockCommand{name: "admin", desc: "admin cmd"},
				perms:       discord.PermissionAdministrator,
			},
			validate: func(t *testing.T, data api.CreateCommandData) {
				require.Equal(t, "admin", data.Name)
				require.NotNil(t, data.DefaultMemberPermissions)
				require.Equal(t, discord.PermissionAdministrator, *data.DefaultMemberPermissions)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			registry := NewCommandRegistry()
			registry.Register(tt.cmd)

			syncer := NewCommandSyncer(nil, 12345)
			data := syncer.BuildCreateData(registry)

			require.Len(t, data, 1)
			tt.validate(t, data[0])
		})
	}
}

func FuzzCommandSyncer_BuildCreateData(f *testing.F) {
	f.Add("normal_name", "normal description")
	f.Add("name_with_spaces ", "desc")
	f.Add("!@#$%^", "invalid chars")
	f.Add(strings.Repeat("a", 100), strings.Repeat("b", 200)) // Max limits

	f.Fuzz(func(t *testing.T, name, desc string) {
		registry := NewCommandRegistry()
		registry.Register(&mockCommand{name: name, desc: desc})

		syncer := NewCommandSyncer(nil, 12345)
		data := syncer.BuildCreateData(registry)

		require.Len(t, data, 1)
		require.Equal(t, name, data[0].Name)
		require.Equal(t, desc, data[0].Description)
	})
}

// 2. Testes de Roteamento de Overwrite
func TestCommandSyncer_SyncBulkOverwrite_Routing(t *testing.T) {
	t.Parallel()

	appID := discord.AppID(111)

	tests := []struct {
		name         string
		guildID      discord.GuildID
		expectedPath string
	}{
		{
			name:         "Global Sync",
			guildID:      discord.NullGuildID,
			expectedPath: "/applications/111/commands",
		},
		{
			name:         "Guild Sync Dinâmico",
			guildID:      discord.GuildID(12345),
			expectedPath: "/applications/111/guilds/12345/commands",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			registry := NewCommandRegistry()
			registry.Register(&mockCommand{name: "test", desc: "test cmd"})

			var requestedPath string
			var capturedBody []byte

			transport := &mockTransport{
				roundTripFunc: func(req *http.Request) (*http.Response, error) {
					requestedPath = req.URL.Path
					if req.Body != nil {
						capturedBody, _ = io.ReadAll(req.Body)
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader("[]")),
						Header:     make(http.Header),
					}, nil
				},
			}

			client := newMockArikawaClient(transport)
			syncer := NewCommandSyncer(client, appID)

			err := syncer.SyncBulkOverwrite(tt.guildID, registry)
			require.NoError(t, err)

			require.True(t, strings.HasSuffix(requestedPath, tt.expectedPath), "Path should end with %s, got %s", tt.expectedPath, requestedPath)

			var payloads []api.CreateCommandData
			err = json.Unmarshal(capturedBody, &payloads)
			require.NoError(t, err)
			require.Len(t, payloads, 1)
			require.Equal(t, "test", payloads[0].Name)
		})
	}
}

// 3. Testes de Resiliência de Erros e Telemetria Integrada
func TestCommandSyncer_SyncBulkOverwrite_TelemetryAndErrors(t *testing.T) {
	t.Parallel()

	appID := discord.AppID(111)

	t.Run("Cenário de Sucesso", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, nil))

		transport := &mockTransport{
			roundTripFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("[]")),
					Header:     make(http.Header),
				}, nil
			},
		}

		client := newMockArikawaClient(transport)
		syncer := NewCommandSyncer(client, appID)
		syncer.SetLogger(logger)
		registry := NewCommandRegistry()
		registry.Register(&mockCommand{name: "telemetry"})

		err := syncer.SyncBulkOverwrite(discord.NullGuildID, registry)
		require.NoError(t, err)

		logOutput := buf.String()
		require.Contains(t, logOutput, "Successfully synchronized commands via BulkOverwrite")
		require.Contains(t, logOutput, `"level":"INFO"`)
	})

	t.Run("Cenário de Falha", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, nil))

		transport := &mockTransport{
			roundTripFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusForbidden,
					Body:       io.NopCloser(strings.NewReader(`{"message": "Missing Access", "code": 50001}`)),
					Header: func() http.Header {
						h := make(http.Header)
						h.Set("Content-Type", "application/json")
						return h
					}(),
				}, nil
			},
		}

		client := newMockArikawaClient(transport)
		syncer := NewCommandSyncer(client, appID)
		syncer.SetLogger(logger)
		registry := NewCommandRegistry()

		err := syncer.SyncBulkOverwrite(discord.NullGuildID, registry)
		require.Error(t, err)
		require.Contains(t, err.Error(), "bulk overwrite failed:")

		var httpErr *httputil.HTTPError
		require.ErrorAs(t, err, &httpErr, "error should wrap the original Arikawa HTTPError")
		require.Equal(t, http.StatusForbidden, httpErr.Status)

		logOutput := buf.String()
		require.Contains(t, logOutput, "Bulk command synchronization failed")
		require.Contains(t, logOutput, `"level":"ERROR"`)
	})
}

// 4. Testes de Avaliação Superficial (Diff)
func TestCommandSyncer_Diff(t *testing.T) {
	t.Parallel()

	appID := discord.AppID(111)

	transport := &mockTransport{
		roundTripFunc: func(req *http.Request) (*http.Response, error) {
			remoteCmds := []discord.Command{
				{Name: "shared"},
				{Name: "remote_only"},
			}
			data, _ := json.Marshal(remoteCmds)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(data)),
				Header: func() http.Header {
					h := make(http.Header)
					h.Set("Content-Type", "application/json")
					return h
				}(),
			}, nil
		},
	}

	client := newMockArikawaClient(transport)
	syncer := NewCommandSyncer(client, appID)

	registry := NewCommandRegistry()
	registry.Register(&mockCommand{name: "shared"})
	registry.Register(&mockCommand{name: "local_only"})

	added, updated, deleted, err := syncer.Diff(context.Background(), discord.NullGuildID, registry)

	require.NoError(t, err)
	require.Equal(t, 1, added, "local_only should be added")
	require.Equal(t, 1, updated, "shared should be updated")
	require.Equal(t, 1, deleted, "remote_only should be deleted")
}
