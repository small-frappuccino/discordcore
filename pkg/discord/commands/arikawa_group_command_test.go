package commands

import (
	"testing"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockArikawaCmd struct {
	mock.Mock
}

func (m *mockArikawaCmd) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *mockArikawaCmd) Description() string {
	args := m.Called()
	return args.String(0)
}

func (m *mockArikawaCmd) Options() []discord.CommandOption {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).([]discord.CommandOption)
}

func (m *mockArikawaCmd) Handle(ctx *ArikawaContext) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockArikawaCmd) RequiresGuild() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *mockArikawaCmd) RequiresPermissions() bool {
	args := m.Called()
	return args.Bool(0)
}

func TestArikawaGroupCommand_Handle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		setupSubcmds  func(*ArikawaGroupCommand, *testing.T) func()
		interaction   discord.InteractionData
		expectedError string
	}{
		{
			name:          "fails on invalid type assertion",
			setupSubcmds:  func(c *ArikawaGroupCommand, t *testing.T) func() { return func() {} },
			interaction:   &discord.PingInteraction{},
			expectedError: "invalid interaction data type",
		},
		{
			name: "delegates to correct subcommand",
			setupSubcmds: func(c *ArikawaGroupCommand, t *testing.T) func() {
				mockCmd := new(mockArikawaCmd)
				mockCmd.On("Name").Return("panel")
				mockCmd.On("Handle", mock.Anything).Return(nil).Once()
				c.AddSubCommand(mockCmd)
				return func() {
					mockCmd.AssertExpectations(t)
				}
			},
			interaction: &discord.CommandInteraction{
				Options: []discord.CommandInteractionOption{
					{Name: "panel"},
				},
			},
			expectedError: "",
		},
		{
			name: "returns error on unknown subcommand",
			setupSubcmds: func(c *ArikawaGroupCommand, t *testing.T) func() {
				return func() {}
			},
			interaction: &discord.CommandInteraction{
				Options: []discord.CommandInteractionOption{
					{Name: "ghost_cmd"},
				},
			},
			expectedError: "subcommand \"ghost_cmd\" not found",
		},
		{
			name:         "fails on empty options",
			setupSubcmds: func(c *ArikawaGroupCommand, t *testing.T) func() { return func() {} },
			interaction: &discord.CommandInteraction{
				Options: nil,
			},
			expectedError: "no subcommand specified",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := NewArikawaGroupCommand("roles", "Gerencia cargos")
			verifyMock := tt.setupSubcmds(cmd, t)

			ctx := &ArikawaContext{
				Interaction: &discord.InteractionEvent{
					Data: tt.interaction,
				},
			}

			err := cmd.Handle(ctx)

			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
			}
			verifyMock()
		})
	}
}

func TestArikawaGroupCommand_Options(t *testing.T) {
	t.Parallel()

	t.Run("empty state", func(t *testing.T) {
		t.Parallel()
		cmd := NewArikawaGroupCommand("empty", "desc")
		opts := cmd.Options()
		require.Empty(t, opts, "expected empty or nil slice for options on fresh group")
	})

	t.Run("flat resolution", func(t *testing.T) {
		t.Parallel()
		cmd := NewArikawaGroupCommand("root", "desc")
		mockSub := new(mockArikawaCmd)
		mockSub.On("Name").Return("sub1")
		mockSub.On("Description").Return("sub desc")
		mockSub.On("Options").Return([]discord.CommandOption{
			&discord.StringOption{OptionName: "arg", Description: "arg desc", Required: true},
		})

		cmd.AddSubCommand(mockSub)

		opts := cmd.Options()
		require.Len(t, opts, 1)

		subOpt, ok := opts[0].(*discord.SubcommandOption)
		require.True(t, ok, "expected SubcommandOption for regular command")
		require.Equal(t, "sub1", subOpt.Name())
		require.Equal(t, "sub desc", subOpt.Description)
		require.Len(t, subOpt.Options, 1)
	})

	t.Run("nested group resolution", func(t *testing.T) {
		t.Parallel()
		root := NewArikawaGroupCommand("root", "desc")
		group := NewArikawaGroupCommand("group", "group desc")

		mockSub := new(mockArikawaCmd)
		mockSub.On("Name").Return("leaf")
		mockSub.On("Description").Return("leaf desc")
		mockSub.On("Options").Return([]discord.CommandOption(nil))

		group.AddSubCommand(mockSub)
		root.AddSubCommand(group)

		opts := root.Options()
		require.Len(t, opts, 1)

		groupOpt, ok := opts[0].(*discord.SubcommandGroupOption)
		require.True(t, ok, "expected SubcommandGroupOption for nested group")
		require.Equal(t, "group", groupOpt.Name())
		require.Equal(t, "group desc", groupOpt.Description)
		require.Len(t, groupOpt.Subcommands, 1)

		leafOpt := groupOpt.Subcommands[0]
		require.Equal(t, "leaf", leafOpt.Name())
		require.Equal(t, "leaf desc", leafOpt.Description)
	})
}

func TestArikawaGroupCommand_Invariants(t *testing.T) {
	t.Parallel()

	t.Run("memory initialization", func(t *testing.T) {
		t.Parallel()
		cmd := NewArikawaGroupCommand("test", "test desc")
		require.NotNil(t, cmd.subcommands, "subcommands map should not be nil")
		require.Equal(t, "test", cmd.Name())
		require.Equal(t, "test desc", cmd.Description())
	})

	t.Run("overwriting protection", func(t *testing.T) {
		t.Parallel()
		cmd := NewArikawaGroupCommand("test", "test desc")

		cmd1 := new(mockArikawaCmd)
		cmd1.On("Name").Return("conflict")

		cmd2 := new(mockArikawaCmd)
		cmd2.On("Name").Return("conflict")

		cmd.AddSubCommand(cmd1)
		cmd.AddSubCommand(cmd2)

		require.Len(t, cmd.subcommands, 1, "map should contain exactly 1 entry")
		require.Same(t, cmd2, cmd.subcommands["conflict"], "last added command should overwrite")
	})

	t.Run("load-bearing invariants", func(t *testing.T) {
		t.Parallel()
		cmd := NewArikawaGroupCommand("test", "test desc")
		require.True(t, cmd.RequiresGuild(), "RequiresGuild must be true")
		require.True(t, cmd.RequiresPermissions(), "RequiresPermissions must be true")
	})
}
