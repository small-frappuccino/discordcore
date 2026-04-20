package logging

import (
	"strings"
	"time"
)

type statsCounterBucket struct {
	all    int
	humans int
	bots   int
}

func (b *statsCounterBucket) addMember(isBot bool, delta int) {
	b.all += delta
	if isBot {
		b.bots += delta
	} else {
		b.humans += delta
	}
	if b.all < 0 {
		b.all = 0
	}
	if b.humans < 0 {
		b.humans = 0
	}
	if b.bots < 0 {
		b.bots = 0
	}
}

func (b statsCounterBucket) total(memberType string) int {
	switch normalizeMemberType(memberType) {
	case "bots":
		return b.bots
	case "humans":
		return b.humans
	default:
		return b.all
	}
}

type statsMemberSnapshot struct {
	isBot        bool
	trackedRoles []string
}

type statsPublishedChannel struct {
	count int
	name  string
	label string
}

type statsGuildState struct {
	initialized     bool
	dirty           bool
	trackedRolesKey string
	lastReconciled  time.Time
	members         map[string]statsMemberSnapshot
	totals          statsCounterBucket
	roleTotals      map[string]statsCounterBucket
	published       map[string]statsPublishedChannel
}

type statsGuildSnapshot struct {
	totals     statsCounterBucket
	roleTotals map[string]statsCounterBucket
}

func newStatsGuildState(trackedRolesKey string, published map[string]statsPublishedChannel) *statsGuildState {
	return &statsGuildState{
		trackedRolesKey: trackedRolesKey,
		members:         make(map[string]statsMemberSnapshot),
		roleTotals:      make(map[string]statsCounterBucket),
		published:       cloneStatsPublishedChannels(published),
	}
}

func cloneStatsPublishedChannels(in map[string]statsPublishedChannel) map[string]statsPublishedChannel {
	if len(in) == 0 {
		return make(map[string]statsPublishedChannel)
	}
	out := make(map[string]statsPublishedChannel, len(in))
	for channelID, published := range in {
		out[channelID] = published
	}
	return out
}

func (state *statsGuildState) applyAdd(userID string, snapshot statsMemberSnapshot) bool {
	if state == nil || strings.TrimSpace(userID) == "" {
		return false
	}
	if state.members == nil {
		state.members = make(map[string]statsMemberSnapshot)
	}
	if _, exists := state.members[userID]; exists {
		return false
	}
	state.members[userID] = snapshot
	state.addContribution(snapshot, 1)
	return true
}

func (state *statsGuildState) applyRemove(userID string) bool {
	if state == nil || strings.TrimSpace(userID) == "" {
		return false
	}
	prev, ok := state.members[userID]
	if !ok {
		return false
	}
	delete(state.members, userID)
	state.addContribution(prev, -1)
	return true
}

func (state *statsGuildState) applyUpdate(userID string, snapshot statsMemberSnapshot) bool {
	if state == nil || strings.TrimSpace(userID) == "" {
		return false
	}
	prev, ok := state.members[userID]
	if !ok {
		return false
	}
	state.addContribution(prev, -1)
	state.members[userID] = snapshot
	state.addContribution(snapshot, 1)
	return true
}

func (state *statsGuildState) addContribution(snapshot statsMemberSnapshot, delta int) {
	state.totals.addMember(snapshot.isBot, delta)
	if state.roleTotals == nil {
		state.roleTotals = make(map[string]statsCounterBucket)
	}
	for _, roleID := range snapshot.trackedRoles {
		bucket := state.roleTotals[roleID]
		bucket.addMember(snapshot.isBot, delta)
		if bucket.all == 0 && bucket.humans == 0 && bucket.bots == 0 {
			delete(state.roleTotals, roleID)
			continue
		}
		state.roleTotals[roleID] = bucket
	}
}
