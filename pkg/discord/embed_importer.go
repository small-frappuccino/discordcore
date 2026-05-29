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
		return nil, err
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
		return "", err
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
