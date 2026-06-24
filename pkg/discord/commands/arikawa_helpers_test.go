package commands

import (
	"testing"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/stretchr/testify/assert"
)

func TestGetArikawaSubCommandOptions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		data     discord.InteractionData
		expected int // Número de opções extraídas esperadas
	}{
		{
			name:     "Invalid Type Assertion (Ping Interaction)",
			data:     &discord.PingInteraction{},
			expected: 0,
		},
		{
			name: "Flat Command (No Subcommands)",
			data: &discord.CommandInteraction{
				Options: []discord.CommandInteractionOption{
					{Name: "arg1", Type: discord.StringOptionType},
				},
			},
			expected: 1,
		},
		{
			name: "Level 1 Subcommand",
			data: &discord.CommandInteraction{
				Options: []discord.CommandInteractionOption{
					{
						Name: "sub",
						Type: discord.SubcommandOptionType,
						Options: []discord.CommandInteractionOption{
							{Name: "arg1", Type: discord.StringOptionType},
							{Name: "arg2", Type: discord.IntegerOptionType},
						},
					},
				},
			},
			expected: 2,
		},
		{
			name: "Level 2 Subcommand Group",
			data: &discord.CommandInteraction{
				Options: []discord.CommandInteractionOption{
					{
						Name: "group",
						Type: discord.SubcommandGroupOptionType,
						Options: []discord.CommandInteractionOption{
							{
								Name: "sub",
								Type: discord.SubcommandOptionType,
								Options: []discord.CommandInteractionOption{
									{Name: "arg1", Type: discord.StringOptionType},
								},
							},
						},
					},
				},
			},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interaction := &discord.InteractionEvent{
				Data: tt.data,
			}
			result := GetArikawaSubCommandOptions(interaction)

			assert.Len(t, result, tt.expected, "Should extract the correct number of options")
		})
	}

	t.Run("Nil Interaction", func(t *testing.T) {
		result := GetArikawaSubCommandOptions(nil)
		assert.Nil(t, result)
	})
}

func TestArikawaOptionList_String(t *testing.T) {
	t.Parallel()
	opts := ArikawaOptionList{
		{Name: "key", Type: discord.StringOptionType, Value: []byte(`"value"`)},
		{Name: "invalid_type", Type: discord.IntegerOptionType, Value: []byte(`123`)},
		{Name: "nil_value", Type: discord.StringOptionType, Value: nil},
	}

	tests := []struct {
		name      string
		searchKey string
		expected  string
	}{
		{"Happy Path", "key", "value"},
		{"Missing Key", "missing", ""},
		{"Nil Value", "nil_value", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "Type Mismatch" {
				// Special check if we added type mismatch scenario
			}
			assert.Equal(t, tt.expected, opts.String(tt.searchKey))
		})
	}
}

func TestArikawaOptionList_Int(t *testing.T) {
	t.Parallel()
	opts := ArikawaOptionList{
		{Name: "key", Type: discord.IntegerOptionType, Value: []byte(`42`)},
		{Name: "invalid_type", Type: discord.StringOptionType, Value: []byte(`"foo"`)},
		{Name: "nil_value", Type: discord.IntegerOptionType, Value: nil},
	}

	tests := []struct {
		name      string
		searchKey string
		expected  int64
	}{
		{"Happy Path", "key", 42},
		{"Missing Key", "missing", 0},
		{"Type Mismatch", "invalid_type", 0},
		{"Nil Value", "nil_value", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, opts.Int(tt.searchKey))
		})
	}
}

func TestArikawaOptionList_Bool(t *testing.T) {
	t.Parallel()
	opts := ArikawaOptionList{
		{Name: "key", Type: discord.BooleanOptionType, Value: []byte(`true`)},
		{Name: "invalid_type", Type: discord.StringOptionType, Value: []byte(`"foo"`)},
		{Name: "nil_value", Type: discord.BooleanOptionType, Value: nil},
	}

	tests := []struct {
		name      string
		searchKey string
		expected  bool
	}{
		{"Happy Path", "key", true},
		{"Missing Key", "missing", false},
		{"Type Mismatch", "invalid_type", false},
		{"Nil Value", "nil_value", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, opts.Bool(tt.searchKey))
		})
	}
}

func TestArikawaOptionList_Float(t *testing.T) {
	t.Parallel()
	opts := ArikawaOptionList{
		{Name: "key", Type: discord.NumberOptionType, Value: []byte(`42.5`)},
		{Name: "invalid_type", Type: discord.StringOptionType, Value: []byte(`"foo"`)},
		{Name: "nil_value", Type: discord.NumberOptionType, Value: nil},
	}

	tests := []struct {
		name      string
		searchKey string
		expected  float64
	}{
		{"Happy Path", "key", 42.5},
		{"Missing Key", "missing", 0},
		{"Type Mismatch", "invalid_type", 0},
		{"Nil Value", "nil_value", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, opts.Float(tt.searchKey))
		})
	}
}

func TestArikawaOptionList_ChannelID(t *testing.T) {
	t.Parallel()
	opts := ArikawaOptionList{
		{Name: "key", Type: discord.ChannelOptionType, Value: []byte(`"123456789"`)},
		{Name: "invalid_type", Type: discord.StringOptionType, Value: []byte(`"foo"`)},
		{Name: "nil_value", Type: discord.ChannelOptionType, Value: nil},
	}

	tests := []struct {
		name      string
		searchKey string
		expected  string
	}{
		{"Happy Path", "key", "123456789"},
		{"Missing Key", "missing", ""},
		{"Type Mismatch", "invalid_type", ""},
		{"Nil Value", "nil_value", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, opts.ChannelID(tt.searchKey))
		})
	}
}

func TestArikawaOptionList_RoleID(t *testing.T) {
	t.Parallel()
	opts := ArikawaOptionList{
		{Name: "key", Type: discord.RoleOptionType, Value: []byte(`"987654321"`)},
		{Name: "invalid_type", Type: discord.StringOptionType, Value: []byte(`"foo"`)},
		{Name: "nil_value", Type: discord.RoleOptionType, Value: nil},
	}

	tests := []struct {
		name      string
		searchKey string
		expected  string
	}{
		{"Happy Path", "key", "987654321"},
		{"Missing Key", "missing", ""},
		{"Type Mismatch", "invalid_type", ""},
		{"Nil Value", "nil_value", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, opts.RoleID(tt.searchKey))
		})
	}
}

func TestArikawaOptionList_HasOption(t *testing.T) {
	t.Parallel()
	opts := ArikawaOptionList{
		{Name: "key", Type: discord.StringOptionType, Value: []byte(`"value"`)},
		{Name: "nil_value", Type: discord.StringOptionType, Value: nil},
	}

	tests := []struct {
		name      string
		searchKey string
		expected  bool
	}{
		{"Existing Key", "key", true},
		{"Missing Key", "missing", false},
		{"Nil Value", "nil_value", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, opts.HasOption(tt.searchKey))
		})
	}
}

func FuzzArikawaOptionList_String(f *testing.F) {
	f.Add("username")
	f.Add("")
	f.Add("invalid_key!@#")

	opts := ArikawaOptionList{
		{Name: "username", Type: discord.StringOptionType, Value: []byte(`"alice"`)},
	}

	f.Fuzz(func(t *testing.T, searchKey string) {
		_ = opts.String(searchKey)
	})
}

func FuzzArikawaOptionList_AllTypes(f *testing.F) {
	f.Add("username")
	f.Add("")
	f.Add("invalid_key!@#")

	opts := ArikawaOptionList{
		{Name: "username", Type: discord.StringOptionType, Value: []byte(`"alice"`)},
		{Name: "age", Type: discord.IntegerOptionType, Value: []byte(`25`)},
		{Name: "is_admin", Type: discord.BooleanOptionType, Value: []byte(`true`)},
		{Name: "score", Type: discord.NumberOptionType, Value: []byte(`99.9`)},
		{Name: "channel", Type: discord.ChannelOptionType, Value: []byte(`"123456789"`)},
		{Name: "role", Type: discord.RoleOptionType, Value: []byte(`"987654321"`)},
	}

	f.Fuzz(func(t *testing.T, searchKey string) {
		_ = opts.String(searchKey)
		_ = opts.Int(searchKey)
		_ = opts.Bool(searchKey)
		_ = opts.Float(searchKey)
		_ = opts.ChannelID(searchKey)
		_ = opts.RoleID(searchKey)
		_ = opts.HasOption(searchKey)
	})
}
