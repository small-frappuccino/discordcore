package control

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

func newDiscordOAuthSessionStore(path string) (discordOAuthSessionStore, error) {
	storePath := strings.TrimSpace(path)
	if storePath == "" {
		return nil, fmt.Errorf("oauth session store path is required")
	}

	store := &discordOAuthSessionDiskStore{
		path:     storePath,
		sessions: map[string]discordOAuthSession{},
	}
	if err := store.loadFromDisk(time.Now().UTC()); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *discordOAuthSessionDiskStore) Create(
	user discordOAuthUser,
	scopes []string,
	accessToken string,
	refreshToken string,
	tokenType string,
	tokenTTL time.Duration,
	ttl time.Duration,
) (discordOAuthSession, error) {
	sessionID, err := generateRandomToken(32)
	if err != nil {
		return discordOAuthSession{}, fmt.Errorf("generate session id: %w", err)
	}
	csrfToken, err := generateRandomToken(32)
	if err != nil {
		return discordOAuthSession{}, fmt.Errorf("generate csrf token: %w", err)
	}

	now := time.Now().UTC()
	sessionExpiresAt := now.Add(ttl).UTC()
	session := discordOAuthSession{
		ID:                   sessionID,
		User:                 user,
		Scopes:               slices.Clone(scopes),
		CSRFToken:            csrfToken,
		AccessToken:          strings.TrimSpace(accessToken),
		RefreshToken:         strings.TrimSpace(refreshToken),
		AccessTokenExpiresAt: resolveAccessTokenExpiry(now, tokenTTL, sessionExpiresAt),
		TokenType:            strings.TrimSpace(tokenType),
		CreatedAt:            now,
		ExpiresAt:            sessionExpiresAt,
	}
	session = canonicalizeDiscordOAuthSession(session)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneExpiredLocked(now)
	s.sessions[sessionID] = session
	if err := s.persistLocked(); err != nil {
		delete(s.sessions, sessionID)
		return discordOAuthSession{}, err
	}
	return cloneDiscordOAuthSession(session), nil
}

func (s *discordOAuthSessionDiskStore) Get(sessionID string, now time.Time) (discordOAuthSession, bool, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return discordOAuthSession{}, false, nil
	}
	now = now.UTC()

	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return discordOAuthSession{}, false, nil
	}
	if now.After(session.ExpiresAt) {
		delete(s.sessions, sessionID)
		if err := s.persistLocked(); err != nil {
			return discordOAuthSession{}, false, err
		}
		return discordOAuthSession{}, false, nil
	}
	return cloneDiscordOAuthSession(session), true, nil
}

func (s *discordOAuthSessionDiskStore) Save(session discordOAuthSession) error {
	session = canonicalizeDiscordOAuthSession(session)
	if session.ID == "" {
		return fmt.Errorf("oauth session id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[session.ID]; !ok {
		return fmt.Errorf("oauth session not found")
	}

	now := time.Now().UTC()
	s.pruneExpiredLocked(now)
	if !session.ExpiresAt.IsZero() && now.After(session.ExpiresAt) {
		delete(s.sessions, session.ID)
	} else {
		s.sessions[session.ID] = session
	}
	return s.persistLocked()
}

func (s *discordOAuthSessionDiskStore) Delete(sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[sessionID]; !ok {
		return nil
	}
	delete(s.sessions, sessionID)
	return s.persistLocked()
}

func (s *discordOAuthSessionDiskStore) pruneExpiredLocked(now time.Time) bool {
	now = now.UTC()
	pruned := false
	for sessionID, session := range s.sessions {
		if !session.ExpiresAt.IsZero() && now.After(session.ExpiresAt) {
			delete(s.sessions, sessionID)
			pruned = true
		}
	}
	return pruned
}

func (s *discordOAuthSessionDiskStore) loadFromDisk(now time.Time) error {
	now = now.UTC()
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read oauth session store file %q: %w", s.path, err)
	}

	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil
	}

	var payload discordOAuthSessionStoreFilePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("decode oauth session store file %q: %w", s.path, err)
	}

	loaded := make(map[string]discordOAuthSession, len(payload.Sessions))
	for _, item := range payload.Sessions {
		session := canonicalizeDiscordOAuthSession(item)
		if session.ID == "" {
			continue
		}
		if session.ExpiresAt.IsZero() || now.After(session.ExpiresAt) {
			continue
		}
		loaded[session.ID] = session
	}
	s.sessions = loaded
	return nil
}

func (s *discordOAuthSessionDiskStore) persistLocked() error {
	dir := filepath.Dir(s.path)
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create oauth session store directory %q: %w", dir, err)
	}

	payload := discordOAuthSessionStoreFilePayload{
		Sessions: make([]discordOAuthSession, 0, len(s.sessions)),
	}
	for _, session := range s.sessions {
		payload.Sessions = append(payload.Sessions, cloneDiscordOAuthSession(session))
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode oauth session store file %q: %w", s.path, err)
	}

	tmpFile, err := os.CreateTemp(dir, "oauth_sessions_*.tmp")
	if err != nil {
		return fmt.Errorf("create temp oauth session store file in %q: %w", dir, err)
	}
	tmpPath := tmpFile.Name()
	cleanupTmp := true
	defer func() {
		if cleanupTmp {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(raw); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write temp oauth session store file %q: %w", tmpPath, err)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("sync temp oauth session store file %q: %w", tmpPath, err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp oauth session store file %q: %w", tmpPath, err)
	}

	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale oauth session store file %q: %w", s.path, err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("replace oauth session store file %q: %w", s.path, err)
	}
	cleanupTmp = false
	return nil
}

func canonicalizeDiscordOAuthSession(session discordOAuthSession) discordOAuthSession {
	session.ID = strings.TrimSpace(session.ID)
	session.User.ID = strings.TrimSpace(session.User.ID)
	session.User.Username = strings.TrimSpace(session.User.Username)
	session.User.Discriminator = strings.TrimSpace(session.User.Discriminator)
	session.User.GlobalName = strings.TrimSpace(session.User.GlobalName)
	session.User.Avatar = strings.TrimSpace(session.User.Avatar)
	session.Scopes = slices.Clone(session.Scopes)
	session.CSRFToken = strings.TrimSpace(session.CSRFToken)
	session.AccessToken = strings.TrimSpace(session.AccessToken)
	session.RefreshToken = strings.TrimSpace(session.RefreshToken)
	session.TokenType = strings.TrimSpace(session.TokenType)
	if !session.AccessTokenExpiresAt.IsZero() {
		session.AccessTokenExpiresAt = session.AccessTokenExpiresAt.UTC()
	}
	if !session.CreatedAt.IsZero() {
		session.CreatedAt = session.CreatedAt.UTC()
	}
	if !session.ExpiresAt.IsZero() {
		session.ExpiresAt = session.ExpiresAt.UTC()
	}
	return session
}

func cloneDiscordOAuthSession(session discordOAuthSession) discordOAuthSession {
	return canonicalizeDiscordOAuthSession(session)
}

func generateRandomToken(length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("token length must be positive")
	}
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func requiresSessionCSRFToken(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}
