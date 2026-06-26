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

	"golang.org/x/time/rate"
)

const discordAPIBase = "https://discord.com/api/v10"

var (
	ErrForbidden    = errors.New("403 forbidden: bot sem permissão (falta de intent ou hierarquia de cargos insuficiente)")
	ErrRateLimited  = errors.New("429 too many requests: limite de bucket exaurido")
	ErrUnauthorized = errors.New("401 unauthorized: token inválido ou revogado")
)

// RESTGateway implementa a interface DiscordGateway definida em ports.go.
// Ele é responsável pelo I/O de rede estrito com o Discord.
type RESTGateway struct {
	client  *http.Client
	limiter *rate.Limiter
}

// NewRESTGateway constrói um novo cliente HTTP isolado.
// Esta instância deve ser tratada como um Singleton injetado no Service.
func NewRESTGateway() *RESTGateway {
	// Clonamos e otimizamos o Transport padrão do Go para maximizar a
	// reutilização de conexões TCP (Connection Pooling) e evitar a exaustão de portas (TIME_WAIT).
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 1000
	t.MaxConnsPerHost = 100
	t.MaxIdleConnsPerHost = 100
	t.IdleConnTimeout = 90 * time.Second

	return &RESTGateway{
		client: &http.Client{
			Transport: t,
			// Um timeout de I/O absoluto para evitar goroutines presas
			// se a API do Discord sofrer uma degradação de rede severa.
			Timeout: 10 * time.Second,
		},
		// O Discord impõe um hard-limit global de 50 requests/segundo por token.
		// Adotamos rate.Limit(45) com burst de 50 para deixar uma margem de segurança de 10%
		// para requisições de heartbeat ou outras métricas cruciais de Gateway.
		limiter: rate.NewLimiter(rate.Limit(45), 50),
	}
}

// ExecuteBan realiza o disparo HTTP REST puro, resolvendo a mutação de estado no Discord.
func (g *RESTGateway) ExecuteBan(ctx context.Context, bot *app.BotInstance, targetUserID string, reason string, deleteSeconds int) error {
	// 1. Throttle Preemptivo (CSP Blocking)
	// Se o limitador estourar, a goroutine dorme de forma não onerosa até o bucket ter tokens.
	// Se o contexto for cancelado (ex: Graceful Shutdown) enquanto dorme, ele retorna erro instantaneamente.
	if err := g.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limiter abortou a execução: %w", err)
	}

	// 2. Composição Zero-Allocation (Evitando reflexão via encoding/json)
	// Como o payload de Ban é ínfimo, concatenar strings economiza ciclos de CPU
	// aliviando o trabalho do Garbage Collector (GC).
	payloadStr := `{"delete_message_seconds":` + strconv.Itoa(deleteSeconds) + `}`

	// Fast path para URL builder
	endpoint := discordAPIBase + "/guilds/" + bot.GuildID + "/bans/" + targetUserID

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader([]byte(payloadStr)))
	if err != nil {
		return fmt.Errorf("falha ao criar requisição HTTP: %w", err)
	}

	// 3. Injeção de Identidade e Segurança (Isolamento Multi-Tenant)
	req.Header.Set("Authorization", "Bot "+bot.Token)
	req.Header.Set("Content-Type", "application/json")

	if reason != "" {
		// A especificação do Discord obriga que o X-Audit-Log-Reason seja encodado em formato URI (RFC 3986)
		// para suportar caracteres Unicode (acentos, emojis) em requisições seguras.
		req.Header.Set("X-Audit-Log-Reason", url.PathEscape(reason))
	}

	// 4. Efetuação da Chamada de Rede
	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("erro de I/O na API do discord: %w", err)
	}
	defer resp.Body.Close()

	// 5. Avaliação Semântica da Resposta
	switch resp.StatusCode {
	case http.StatusNoContent, http.StatusOK:
		return nil // Ação bem-sucedida, não há corpo para ler.
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusTooManyRequests:
		// Se atingirmos o Rate Limit específico deste bucket da Guilda.
		// Uma evolução arquitetural futura pode capturar o header `X-RateLimit-Reset-After`,
		// paralisar este Worker específico usando `time.Sleep` e reinjetar o Job na fila.
		return ErrRateLimited
	default:
		// Erros 5xx ou cenários inesperados.
		return fmt.Errorf("discord api retornou status não tratado: %d", resp.StatusCode)
	}
}

// ExecuteKick realiza o disparo HTTP REST puro para remover um membro da guilda.
func (g *RESTGateway) ExecuteKick(ctx context.Context, bot *app.BotInstance, targetUserID string, reason string) error {
	// 1. Throttle Preemptivo (CSP Blocking)
	// Se o limitador estourar, a goroutine dorme de forma não onerosa até o bucket ter tokens.
	// Se o contexto for cancelado (ex: Graceful Shutdown) enquanto dorme, ele retorna erro instantaneamente.
	if err := g.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limiter abortou a execução do kick: %w", err)
	}

	// 2. Fast path para URL builder e Construção da Requisição
	endpoint := discordAPIBase + "/guilds/" + bot.GuildID + "/members/" + targetUserID
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil) // Kick não tem payload no corpo
	if err != nil {
		return err
	}

	// 3. Injeção de Identidade e Segurança (Isolamento Multi-Tenant)
	req.Header.Set("Authorization", "Bot "+bot.Token)
	if reason != "" {
		// A especificação do Discord obriga que o X-Audit-Log-Reason seja encodado em formato URI (RFC 3986)
		// para suportar caracteres Unicode (acentos, emojis) em requisições seguras.
		req.Header.Set("X-Audit-Log-Reason", url.PathEscape(reason))
	}

	// 4. Efetuação da Chamada de Rede
	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("erro de I/O na API do discord: %w", err)
	}
	defer resp.Body.Close()

	// 5. Avaliação Semântica da Resposta
	switch resp.StatusCode {
	case http.StatusNoContent, http.StatusOK:
		return nil // Ação bem-sucedida, não há corpo para ler.
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusTooManyRequests:
		// Se atingirmos o Rate Limit específico deste bucket da Guilda.
		// Uma evolução arquitetural futura pode capturar o header `X-RateLimit-Reset-After`,
		// paralisar este Worker específico usando `time.Sleep` e reinjetar o Job na fila.
		return ErrRateLimited
	default:
		// Erros 5xx ou cenários inesperados.
		return fmt.Errorf("discord api retornou status não tratado: %d", resp.StatusCode)
	}
}
