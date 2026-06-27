package moderation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/core"
)

const discordAPIBase = "https://discord.com/api/v10"

var (
	ErrForbidden    = errors.New("403 forbidden: bot sem permissão (falta de intent ou hierarquia de cargos insuficiente)")
	ErrUnauthorized = errors.New("401 unauthorized: token inválido ou revogado")
)

type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("429 too many requests: retry after %v", e.RetryAfter)
}

// RESTGateway implementa a interface DiscordGateway definida em ports.go.
type RESTGateway struct {
	client *http.Client
}

// NewRESTGateway constrói um novo cliente HTTP isolado sem limitador global.
func NewRESTGateway() *RESTGateway {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 1000
	t.MaxConnsPerHost = 100
	t.MaxIdleConnsPerHost = 100
	t.IdleConnTimeout = 90 * time.Second

	return &RESTGateway{
		client: &http.Client{
			Transport: t,
			Timeout:   10 * time.Second,
		},
	}
}

// checkRateLimitHeaders inspeciona a resposta para backpressure preemptivo.
func checkRateLimitHeaders(resp *http.Response) error {
	if resp.StatusCode == http.StatusTooManyRequests {
		resetAfterStr := resp.Header.Get("X-RateLimit-Reset-After")
		if resetAfterStr != "" {
			if resetSecs, err := strconv.ParseFloat(resetAfterStr, 64); err == nil {
				return &RateLimitError{RetryAfter: time.Duration(resetSecs * float64(time.Second))}
			}
		}
		// Default fallback
		return &RateLimitError{RetryAfter: 5 * time.Second}
	}

	// Preemptive backpressure se restantes == 0
	remStr := resp.Header.Get("X-RateLimit-Remaining")
	if remStr == "0" {
		resetAfterStr := resp.Header.Get("X-RateLimit-Reset-After")
		if resetAfterStr != "" {
			if resetSecs, err := strconv.ParseFloat(resetAfterStr, 64); err == nil {
				return &RateLimitError{RetryAfter: time.Duration(resetSecs * float64(time.Second))}
			}
		}
	}
	return nil
}

func (g *RESTGateway) ExecuteBan(ctx context.Context, bot *core.BotInstance, targetUserID uint64, reason string, deleteSeconds int) error {
	payloadStr := `{"delete_message_seconds":` + strconv.Itoa(deleteSeconds) + `}`

	endpoint := discordAPIBase + "/guilds/" + bot.GuildID + "/bans/" + strconv.FormatUint(targetUserID, 10)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader([]byte(payloadStr)))
	if err != nil {
		return fmt.Errorf("falha ao criar requisição HTTP: %w", err)
	}

	req.Header.Set("Authorization", "Bot "+bot.Token)
	req.Header.Set("Content-Type", "application/json")

	if reason != "" {
		req.Header.Set("X-Audit-Log-Reason", url.PathEscape(reason))
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("erro de I/O na API do discord: %w", err)
	}
	defer resp.Body.Close()

	if rlErr := checkRateLimitHeaders(resp); rlErr != nil && resp.StatusCode == http.StatusTooManyRequests {
		return rlErr // Abort with 429
	}

	switch resp.StatusCode {
	case http.StatusNoContent, http.StatusOK:
		return checkRateLimitHeaders(resp) // Pass preemptive backpressure if any
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusUnauthorized:
		return ErrUnauthorized
	default:
		return fmt.Errorf("discord api retornou status não tratado: %d", resp.StatusCode)
	}
}

func (g *RESTGateway) ExecuteKick(ctx context.Context, bot *core.BotInstance, targetUserID uint64, reason string) error {
	endpoint := discordAPIBase + "/guilds/" + bot.GuildID + "/members/" + strconv.FormatUint(targetUserID, 10)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bot "+bot.Token)
	if reason != "" {
		req.Header.Set("X-Audit-Log-Reason", url.PathEscape(reason))
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("erro de I/O na API do discord: %w", err)
	}
	defer resp.Body.Close()

	if rlErr := checkRateLimitHeaders(resp); rlErr != nil && resp.StatusCode == http.StatusTooManyRequests {
		return rlErr
	}

	switch resp.StatusCode {
	case http.StatusNoContent, http.StatusOK:
		return checkRateLimitHeaders(resp)
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusUnauthorized:
		return ErrUnauthorized
	default:
		return fmt.Errorf("discord api retornou status não tratado: %d", resp.StatusCode)
	}
}
