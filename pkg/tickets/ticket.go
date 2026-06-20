package tickets

import (
	"context"
	"fmt"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

// GenerateTicketName creates a canonical text channel name for a new ticket.
func GenerateTicketName(id int64) string {
	return fmt.Sprintf("ticket-%04d", id)
}

// IsOpenTicket checks if the given channel name indicates an active ticket.
func IsOpenTicket(name string) bool {
	return len(name) >= 7 && name[:7] == "ticket-"
}

// IsClosedTicket checks if the given channel name indicates a closed ticket.
func IsClosedTicket(name string) bool {
	return len(name) >= 7 && name[:7] == "closed-"
}

// OpenToClosedName converts an open ticket channel name to a closed one.
func OpenToClosedName(name string) string {
	if IsOpenTicket(name) {
		return "closed-" + name[7:]
	}
	return name
}

// ClosedToOpenName converts a closed ticket channel name back to an open one.
func ClosedToOpenName(name string) string {
	if IsClosedTicket(name) {
		return "ticket-" + name[7:]
	}
	return name
}

// ComputeOpenMemberAllow returns the permission bits allowed for the ticket creator upon opening.
func ComputeOpenMemberAllow() discord.Permissions {
	// Standard viewing, sending, and history permissions.
	return discord.PermissionViewChannel | discord.PermissionSendMessages | discord.PermissionReadMessageHistory
}

// ComputeOpenRoleAllow returns the permission bits allowed for the designated staff role upon opening.
func ComputeOpenRoleAllow() discord.Permissions {
	return discord.PermissionViewChannel | discord.PermissionSendMessages | discord.PermissionReadMessageHistory
}

// ComputeCloseMemberAllow takes the current allow mask and removes the ability to send messages.
func ComputeCloseMemberAllow(current discord.Permissions) discord.Permissions {
	// Bitwise clearing of the SendMessages flag
	return current &^ discord.PermissionSendMessages
}

// ComputeCloseMemberDeny takes the current deny mask and explicitly denies the ability to send messages.
func ComputeCloseMemberDeny(current discord.Permissions) discord.Permissions {
	// Bitwise setting of the SendMessages flag
	return current | discord.PermissionSendMessages
}

// ComputeReopenMemberAllow takes the current allow mask and restores the ability to send messages.
func ComputeReopenMemberAllow(current discord.Permissions) discord.Permissions {
	return current | discord.PermissionSendMessages
}

// ComputeReopenMemberDeny takes the current deny mask and removes explicit denial to send messages.
func ComputeReopenMemberDeny(current discord.Permissions) discord.Permissions {
	return current &^ discord.PermissionSendMessages
}

// Manager orchestrates domain logic for tickets avoiding direct Discord integrations.
type Manager struct {
	store *storage.Store
}

// NewManager constructs a ticket manager.
func NewManager(store *storage.Store) *Manager {
	return &Manager{store: store}
}

// NextID retrieves the next sequential ticket identifier for a given guild in a thread-safe manner.
func (m *Manager) NextID(ctx context.Context, guildID string) (int64, error) {
	return m.store.NextTicketID(ctx, guildID)
}
