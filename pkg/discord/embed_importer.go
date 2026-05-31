package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

// DefaultPasteProviderURL is the default provider used for uploading JSON embeds.
// Hastebin creates unlisted pastes, which satisfies the restricted access requirement.
const DefaultPasteProviderURL = "https://hastebin.com"

// FetchPastebinContent downloads the text content from a given URL.
func FetchPastebinContent(ctx context.Context, pasteURL string) ([]byte, error) {
	parsed, err := url.Parse(pasteURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme: %s", parsed.Scheme)
	}

	// Auto-correct common provider URLs to their raw endpoints if possible.
	host := strings.ToLower(parsed.Hostname())
	path := parsed.Path
	if strings.Contains(host, "hastebin.com") {
		if !strings.HasPrefix(path, "/raw/") && path != "/" {
			parsed.Path = "/raw" + path
			pasteURL = parsed.String()
		}
	} else if strings.Contains(host, "pastebin.com") {
		if !strings.HasPrefix(path, "/raw/") && path != "/" {
			parsed.Path = "/raw" + path
			pasteURL = parsed.String()
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pasteURL, nil)
	if err != nil {
		return nil, fmt.Errorf("FetchPastebinContent: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch paste: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("provider returned status %d", resp.StatusCode)
	}

	// Limit read to 64KB to avoid memory exhaustion (Discord embeds are small).
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return data, nil
}

// UploadHastebinContent uploads data to the paste provider and returns the unlisted URL.
func UploadHastebinContent(ctx context.Context, data []byte) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, DefaultPasteProviderURL+"/documents", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("UploadHastebinContent: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to upload paste: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("provider returned status %d", resp.StatusCode)
	}

	var result struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode provider response: %w", err)
	}

	if result.Key == "" {
		return "", fmt.Errorf("provider returned empty key")
	}

	return fmt.Sprintf("%s/%s", DefaultPasteProviderURL, result.Key), nil
}

// UploadPastebinContent uploads data to pastebin.com using credentials from global configuration.
func UploadPastebinContent(ctx context.Context, data []byte, devKey, username, password string) (string, error) {
	// First get the user key (session key) from pastebin.com.
	loginVals := url.Values{}
	loginVals.Set("api_dev_key", devKey)
	loginVals.Set("api_user_name", username)
	loginVals.Set("api_user_password", password)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://pastebin.com/api/api_login.php", strings.NewReader(loginVals.Encode()))
	if err != nil {
		return "", fmt.Errorf("UploadPastebinContent: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to authenticate with Pastebin: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read Pastebin auth response: %w", err)
	}
	bodyStr := string(bodyBytes)
	if resp.StatusCode != http.StatusOK || strings.HasPrefix(bodyStr, "Bad API request") {
		return "", fmt.Errorf("Pastebin authentication failed: %s", bodyStr)
	}

	userKey := strings.TrimSpace(bodyStr)

	// Now upload the paste.
	postVals := url.Values{}
	postVals.Set("api_dev_key", devKey)
	postVals.Set("api_user_key", userKey)
	postVals.Set("api_option", "paste")
	postVals.Set("api_paste_code", string(data))
	postVals.Set("api_paste_private", "1") // Unlisted
	postVals.Set("api_paste_format", "json")

	postReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://pastebin.com/api/api_post.php", strings.NewReader(postVals.Encode()))
	if err != nil {
		return "", fmt.Errorf("UploadPastebinContent: %w", err)
	}
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	postResp, err := client.Do(postReq)
	if err != nil {
		return "", fmt.Errorf("failed to upload to Pastebin: %w", err)
	}
	defer postResp.Body.Close()

	postBodyBytes, err := io.ReadAll(postResp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read Pastebin upload response: %w", err)
	}
	postBodyStr := strings.TrimSpace(string(postBodyBytes))
	if postResp.StatusCode != http.StatusOK || strings.HasPrefix(postBodyStr, "Bad API request") {
		return "", fmt.Errorf("Pastebin upload failed: %s", postBodyStr)
	}

	return postBodyStr, nil
}

// UploadExportedContent uploads the data to Pastebin (if configured and user is admin) or Hastebin.
func UploadExportedContent(ctx context.Context, member *discordgo.Member, ownerID string, configManager *files.ConfigManager, data []byte) (string, error) {
	rc := configManager.Config().RuntimeConfig
	if rc.PastebinDevKey != "" && rc.PastebinUserName != "" && rc.PastebinUserPassword != "" {
		// Check if user is administrator
		isAdmin := false
		if member != nil {
			if member.User != nil && member.User.ID == ownerID {
				isAdmin = true
			} else {
				perms := member.Permissions
				if (perms&discordgo.PermissionAdministrator) != 0 || (perms&discordgo.PermissionManageGuild) != 0 {
					isAdmin = true
				}
			}
		}
		if !isAdmin {
			return "", fmt.Errorf("global Pastebin credentials are configured, but this feature is restricted to server administrators")
		}
		return UploadPastebinContent(ctx, data, string(rc.PastebinDevKey), string(rc.PastebinUserName), string(rc.PastebinUserPassword))
	}
	return UploadHastebinContent(ctx, data)
}
