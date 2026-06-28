package discord

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
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

type BucketState struct {
	mu        sync.Mutex
	Remaining int
	ResetAt   time.Time
}

type GlobalRateLimiter struct {
	mu       sync.Mutex
	lastTick time.Time
	tokens   float64
}

func (l *GlobalRateLimiter) Wait(ctx context.Context) error {
	l.mu.Lock()
	now := time.Now()
	if l.lastTick.IsZero() {
		l.lastTick = now
		l.tokens = 50
	} else {
		elapsed := now.Sub(l.lastTick)
		l.tokens += elapsed.Seconds() * 50
		if l.tokens > 50 {
			l.tokens = 50
		}
		l.lastTick = now
	}

	if l.tokens >= 1 {
		l.tokens -= 1
		l.mu.Unlock()
		return nil
	}

	waitDuration := time.Duration((1 - l.tokens) / 50 * float64(time.Second))
	l.tokens = 0
	l.lastTick = now.Add(waitDuration)
	l.mu.Unlock()

	select {
	case <-time.After(waitDuration):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// RESTGateway implementa a interface DiscordGateway definida em discord_gateway_port.go.
type RESTGateway struct {
	client         *http.Client
	buckets        sync.Map // map[string]*BucketState
	globalLimiters sync.Map // map[string]*GlobalRateLimiter (key: token)
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

func (g *RESTGateway) getGlobalLimiter(token string) *GlobalRateLimiter {
	v, ok := g.globalLimiters.Load(token)
	if !ok {
		limiter := &GlobalRateLimiter{}
		v, _ = g.globalLimiters.LoadOrStore(token, limiter)
	}
	return v.(*GlobalRateLimiter)
}

func (g *RESTGateway) getBucketState(key string) *BucketState {
	v, ok := g.buckets.Load(key)
	if !ok {
		bs := &BucketState{Remaining: 1}
		v, _ = g.buckets.LoadOrStore(key, bs)
	}
	return v.(*BucketState)
}

func (g *RESTGateway) acquireBucket(key string) error {
	bs := g.getBucketState(key)
	bs.mu.Lock()
	defer bs.mu.Unlock()

	now := time.Now()
	if now.After(bs.ResetAt) {
		bs.Remaining = 1
	}

	if bs.Remaining <= 0 {
		return &RateLimitError{RetryAfter: bs.ResetAt.Sub(now)}
	}
	bs.Remaining--
	return nil
}

func (g *RESTGateway) updateBucket(key string, resp *http.Response) {
	bs := g.getBucketState(key)
	remStr := resp.Header.Get("X-RateLimit-Remaining")
	resetAfterStr := resp.Header.Get("X-RateLimit-Reset-After")

	bs.mu.Lock()
	defer bs.mu.Unlock()

	if remStr != "" {
		if rem, err := strconv.Atoi(remStr); err == nil {
			bs.Remaining = rem
		}
	}
	if resetAfterStr != "" {
		if resetSecs, err := strconv.ParseFloat(resetAfterStr, 64); err == nil {
			bs.ResetAt = time.Now().Add(time.Duration(resetSecs * float64(time.Second)))
		}
	}
}

func (g *RESTGateway) ExecuteBan(ctx context.Context, bot core.BotInstance, targetUserID uint64, reason string, deleteSeconds int) error {
	tokenStr := string(bot.Token)
	if err := g.getGlobalLimiter(tokenStr).Wait(ctx); err != nil {
		return err
	}

	bucketKey := tokenStr + ":guild:" + bot.GuildID + ":bans"
	if err := g.acquireBucket(bucketKey); err != nil {
		return err
	}

	payloadStr := `{"delete_message_seconds":` + strconv.Itoa(deleteSeconds) + `}`

	endpoint := discordAPIBase + "/guilds/" + bot.GuildID + "/bans/" + strconv.FormatUint(targetUserID, 10)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader([]byte(payloadStr)))
	if err != nil {
		return fmt.Errorf("falha ao criar requisição HTTP: %w", err)
	}

	req.Header.Set("Authorization", "Bot "+string(bot.Token))
	req.Header.Set("Content-Type", "application/json")

	if reason != "" {
		req.Header.Set("X-Audit-Log-Reason", url.PathEscape(reason))
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("erro de I/O na API do discord: %w", err)
	}
	defer resp.Body.Close()

	g.updateBucket(bucketKey, resp)

	if resp.StatusCode == http.StatusTooManyRequests {
		return &RateLimitError{RetryAfter: 5 * time.Second}
	}

	switch resp.StatusCode {
	case http.StatusNoContent, http.StatusOK:
		return nil
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusUnauthorized:
		return ErrUnauthorized
	default:
		return fmt.Errorf("discord api retornou status não tratado: %d", resp.StatusCode)
	}
}

func (g *RESTGateway) ExecuteKick(ctx context.Context, bot core.BotInstance, targetUserID uint64, reason string) error {
	tokenStr := string(bot.Token)
	if err := g.getGlobalLimiter(tokenStr).Wait(ctx); err != nil {
		return err
	}

	bucketKey := tokenStr + ":guild:" + bot.GuildID + ":kicks"
	if err := g.acquireBucket(bucketKey); err != nil {
		return err
	}

	endpoint := discordAPIBase + "/guilds/" + bot.GuildID + "/members/" + strconv.FormatUint(targetUserID, 10)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bot "+string(bot.Token))
	if reason != "" {
		req.Header.Set("X-Audit-Log-Reason", url.PathEscape(reason))
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("erro de I/O na API do discord: %w", err)
	}
	defer resp.Body.Close()

	g.updateBucket(bucketKey, resp)

	if resp.StatusCode == http.StatusTooManyRequests {
		return &RateLimitError{RetryAfter: 5 * time.Second}
	}

	switch resp.StatusCode {
	case http.StatusNoContent, http.StatusOK:
		return nil
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusUnauthorized:
		return ErrUnauthorized
	default:
		return fmt.Errorf("discord api retornou status não tratado: %d", resp.StatusCode)
	}
}
