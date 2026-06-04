package control

import (
	"github.com/small-frappuccino/discordcore/pkg/files"
)

// ErrorResponse represents a generic error returned by the API.
type ErrorResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

// WorkspaceResponse represents a global feature workspace response.
type WorkspaceResponse struct {
	Status    string           `json:"status"`
	Workspace featureWorkspace `json:"workspace"`
}

// FeatureResponse represents a global feature configuration response.
type FeatureResponse struct {
	Status  string        `json:"status"`
	Feature featureRecord `json:"feature"`
}

// GuildFeatureResponse represents a guild-specific feature configuration response.
type GuildFeatureResponse struct {
	Status  string        `json:"status"`
	GuildID string        `json:"guild_id"`
	Feature featureRecord `json:"feature"`
}

// GuildChannelsResponse represents a list of available guild channels for configuration.
type GuildChannelsResponse struct {
	Status   string               `json:"status"`
	GuildID  string               `json:"guild_id"`
	Channels []guildChannelOption `json:"channels"`
}

// GuildMembersResponse represents a list of guild members matching a search query.
type GuildMembersResponse struct {
	Status  string              `json:"status"`
	GuildID string              `json:"guild_id"`
	Members []guildMemberOption `json:"members"`
}

// GuildRolesResponse represents a list of available guild roles.
type GuildRolesResponse struct {
	Status  string            `json:"status"`
	GuildID string            `json:"guild_id"`
	Roles   []guildRoleOption `json:"roles"`
}

// QOTDSummaryResponse represents the dashboard summary of QOTD state for a guild.
type QOTDSummaryResponse struct {
	Status  string              `json:"status"`
	GuildID string              `json:"guild_id"`
	Summary qotdSummaryResponse `json:"summary"`
}

// QOTDSettingsResponse represents the QOTD guild configuration payload.
type QOTDSettingsResponse struct {
	Status   string           `json:"status"`
	GuildID  string           `json:"guild_id"`
	Settings files.QOTDConfig `json:"settings"`
}

// QOTDQuestionsResponse represents a list of QOTD questions.
type QOTDQuestionsResponse struct {
	Status    string                 `json:"status"`
	GuildID   string                 `json:"guild_id"`
	Questions []qotdQuestionResponse `json:"questions"`
}

// QOTDQuestionsBatchResponse represents a batch create response.
type QOTDQuestionsBatchResponse struct {
	Status    string                 `json:"status"`
	GuildID   string                 `json:"guild_id"`
	Questions []qotdQuestionResponse `json:"questions"`
	Error     string                 `json:"error,omitempty"`
}

// QOTDQuestionResponse represents a single QOTD question mutation response.
type QOTDQuestionResponse struct {
	Status   string               `json:"status"`
	GuildID  string               `json:"guild_id"`
	Question qotdQuestionResponse `json:"question"`
}

// QOTDDeleteQuestionResponse represents a successful QOTD question deletion.
type QOTDDeleteQuestionResponse struct {
	Status    string `json:"status"`
	GuildID   string `json:"guild_id"`
	DeletedID int64  `json:"deleted_id"`
}

// QOTDPublishResult represents the result payload of a manual publish action.
type QOTDPublishResult struct {
	PostURL      string                    `json:"post_url"`
	Question     qotdQuestionResponse      `json:"question"`
	OfficialPost *qotdOfficialPostResponse `json:"official_post"`
}

// QOTDPublishNowResponse represents the full response of a manual publish action.
type QOTDPublishNowResponse struct {
	Status  string            `json:"status"`
	GuildID string            `json:"guild_id"`
	Result  QOTDPublishResult `json:"result"`
}

// RuntimeConfigResponse represents a runtime config update response.
type RuntimeConfigResponse struct {
	Status        string              `json:"status"`
	RuntimeConfig files.RuntimeConfig `json:"runtime_config"`
}
