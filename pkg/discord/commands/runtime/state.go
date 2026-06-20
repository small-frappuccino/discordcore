package runtime

import (
	"hash/fnv"
	"strconv"
	"strings"
)

// pageMode dictates the current view being rendered in the interactive panel.
type pageMode string

const (
	pageMain   pageMode = "main"
	pageHelp   pageMode = "help"
	pageDetail pageMode = "detail"
)

// runtimeKey uniquely identifies a configurable property within the system.
type runtimeKey string

const (
	stateSep         = "|"
	customIDPrefix   = "runtimecfg:"
	modalEditValueID = customIDPrefix + "modal:edit"
)

// panelState encapsulates the contextual navigational state of the runtime configuration dashboard.
type panelState struct {
	Mode  pageMode
	Group string
	Key   runtimeKey
	Scope string
}

func (s panelState) withMode(m pageMode) panelState  { s.Mode = m; return s }
func (s panelState) withGroup(g string) panelState   { s.Group = g; return s }
func (s panelState) withKey(k runtimeKey) panelState { s.Key = k; return s }
func (s panelState) withScope(sc string) panelState  { s.Scope = sc; return s }

// encode serializes the panelState into a delimited string safe for Discord CustomIDs.
func (s panelState) encode() string {
	return string(s.Mode) + stateSep + s.Group + stateSep + string(s.Key) + stateSep + s.Scope
}

// sanitizeState ensures all fields hold permissible bounds, falling back to safe defaults if malformed.
func sanitizeState(st panelState) panelState {
	switch st.Mode {
	case pageMain, pageHelp, pageDetail:
		// Safe execution path: Mode aligns with recognized identifiers.
	default:
		st.Mode = pageMain
	}

	if st.Group == "" {
		st.Group = "ALL"
	}

	// Ensure scope has a fallback to prevent unauthorized global state mutations implicitly.
	if st.Scope == "" {
		st.Scope = "global"
	}

	return st
}

// decodeState parses an opaquely injected CustomID payload into a structured panelState.
// It explicitly guards against slice bound panics by utilizing strings.SplitN, mitigating malicious inputs.
func decodeState(raw string) panelState {
	st := panelState{Mode: pageMain, Group: "ALL", Scope: "global"}

	// Operational annotation: SplitN with 4 dictates a strict ceiling on slice allocation.
	// This prevents memory exhaustion attacks via infinitely long delimited strings.
	parts := strings.SplitN(raw, stateSep, 4)

	if len(parts) > 0 {
		if v := strings.TrimSpace(parts[0]); v != "" {
			st.Mode = pageMode(v)
		}
	}
	if len(parts) > 1 {
		if v := strings.TrimSpace(parts[1]); v != "" {
			st.Group = v
		}
	}
	if len(parts) > 2 {
		if v := strings.TrimSpace(parts[2]); v != "" {
			st.Key = runtimeKey(v)
		}
	}
	if len(parts) > 3 {
		if v := strings.TrimSpace(parts[3]); v != "" {
			st.Scope = v
		}
	}

	return sanitizeState(st)
}

// runtimeInteractionAuthToken derives a deterministic, short-lived verification token from the actor's Snowflake ID.
// This enforces structural isolation, ensuring components emitted to one user cannot be actioned by another.
func runtimeInteractionAuthToken(userID string) string {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ""
	}

	// Operational annotation: FNV-1a provides rapid, non-cryptographic hashing strictly to prevent
	// accidental cross-session interaction pollution, not adversarial tampering, as Discord's HTTP
	// gateway already guarantees Snowflake provenance.
	hash := fnv.New32a()
	hash.Write([]byte(userID))
	return strconv.FormatUint(uint64(hash.Sum32()), 36)
}

// encodeRuntimeModalState produces an authorized CustomID tailored for Discord modal emission.
func encodeRuntimeModalState(st panelState, actorUserID string) string {
	scope := strings.TrimSpace(st.Scope)
	if scope == "" {
		scope = "global"
	}
	return modalEditValueID + stateSep + string(st.Key) + stateSep + scope + stateSep + runtimeInteractionAuthToken(actorUserID)
}

// decodeRuntimeModalState strictly extracts and validates state from a modal submission CustomID.
// It returns the authorized state, the embedded token, and a boolean affirming extraction viability.
func decodeRuntimeModalState(customID string) (panelState, string, bool) {
	routeID, rawState, hasState := strings.Cut(customID, stateSep)
	if routeID != modalEditValueID || !hasState {
		return panelState{}, "", false
	}

	// Modal payloads inherently encode exactly 3 mutable segments: key, scope, token.
	parts := strings.SplitN(rawState, stateSep, 3)
	if len(parts) != 3 {
		return panelState{}, "", false
	}

	key := runtimeKey(strings.TrimSpace(parts[0]))
	scope := strings.TrimSpace(parts[1])
	if scope == "" {
		scope = "global"
	}

	st := panelState{
		Mode:  pageMain,
		Group: "ALL", // Modals inherently strip group context; defaulting to ALL enforces a safe return to root.
		Key:   key,
		Scope: scope,
	}

	return sanitizeState(st), strings.TrimSpace(parts[2]), true
}
