# Domain Architecture: discord

## Layout Topology
```text
discord/
└── moderation
    ├── fast_parser.go
    ├── moderation_gateway.go
    ├── ports.go
    ├── router.go
    └── service.go
```

## Source Stream Aggregation

// === FILE: pkg/discord/moderation/fast_parser.go ===
```go
package moderation

import (
	"github.com/buger/jsonparser"
)

// extractStringFast navega pelos bytes do JSON puro e extrai o valor.
// Funciona em tempo O(N) (sendo N a profundidade da chave) sem criar structs na Heap.
func extractStringFast(payload []byte, keys ...string) string {
	val, err := jsonparser.GetString(payload, keys...)
	if err != nil {
		return "" // Retorna vazio se a chave não existir (seguro para campos opcionais)
	}
	return val
}

// extractCommandNameFast é um helper direto para ir buscar "data.name".
func extractCommandNameFast(payload []byte) string {
	return extractStringFast(payload, "data", "name")
}

// extractOptionString varre o array "options" do Discord de forma ultrarrápida.
// O Discord aninha os argumentos dos Slash Commands como um array de objetos:
// "options": [{"name": "target_id", "value": "123"}, {"name": "reason", "value": "spam"}]
func extractOptionString(payload []byte, targetOptionName string) string {
	var result string

	// ArrayEach iterará sobre os bytes do array diretamente (Zero-Allocation).
	// Em vez de decodificar o array todo, paramos quando encontramos a opção.
	jsonparser.ArrayEach(payload, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		name, _ := jsonparser.GetString(value, "name")
		if name == targetOptionName {
			result, _ = jsonparser.GetString(value, "value")
		}
	}, "data", "options")

	return result
}

// extractOptionInt extrai um inteiro de forma segura do array "options".
func extractOptionInt(payload []byte, targetOptionName string) int {
	var result int64

	jsonparser.ArrayEach(payload, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		name, _ := jsonparser.GetString(value, "name")
		if name == targetOptionName {
			result, _ = jsonparser.GetInt(value, "value")
		}
	}, "data", "options")

	return int(result)
}

```

// === FILE: pkg/discord/moderation/moderation_gateway.go ===
```go
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

```

// === FILE: pkg/discord/moderation/ports.go ===
```go
package moderation

import (
	"context"
)

// DiscordGateway define a interface de saída (Port) estrita para a API do Discord.
type DiscordGateway interface {
	ExecuteBan(ctx context.Context, bot *app.BotInstance, targetUserID string, reason string, deleteSeconds int) error
	ExecuteKick(ctx context.Context, bot *app.BotInstance, targetUserID string, reason string) error
}

```

// === FILE: pkg/discord/moderation/router.go ===
```go
package moderation

import (
	"context"
	"fmt"
)

type Router struct {
	registry app.FeatureRegistry
	service  *Service
}

// HandleInteraction pode rodar milhares de vezes concorrentemente.
// Como não há locks (Mutex) de escrita aqui, não há concorrência (race conditions).
func (r *Router) HandleInteraction(ctx context.Context, payload []byte) error {
	// Extração focada de JSON (Ex: fastjson.GetString(payload, "guild_id"))
	// Isso evita reflexão e o Garbage Collector fica intocado.
	guildID := extractStringFast(payload, "guild_id")
	appID := extractStringFast(payload, "application_id")

	// Validação Multi-Tenant: O bot tem a feature de moderação nesta guilda?
	botInstance, err := r.registry.ResolveOwner(ctx, guildID, "moderation")
	if err != nil || botInstance.ApplicationID != appID {
		return ErrFeatureUnauthorized
	}

	commandName := extractCommandNameFast(payload)

	// Cria o Job estrito na Stack (sem new(), sem alocação na Heap)
	job := ModerationJob{
		Ctx:          ctx,
		Bot:          botInstance,
		TargetUserID: extractStringFast(payload, "data", "options", "target_id"),
	}

	switch commandName {
	case "ban":
		job.Action = ActionBan
		job.Reason = extractStringFast(payload, "data", "options", "reason")
		job.DeleteDays = extractIntFast(payload, "data", "options", "delete_days")
	case "kick":
		job.Action = ActionKick
		job.Reason = extractStringFast(payload, "data", "options", "reason")
	default:
		return fmt.Errorf("comando desconhecido: %s", commandName)
	}

	targetUserID := extractOptionString(payload, "target_id")
	reason := extractOptionString(payload, "reason")

	switch commandName {
	case "ban":
		deleteDays := extractOptionInt(payload, "delete_days")

		return r.service.EnqueueTask(ModerationJob{
			Ctx:          ctx,
			Bot:          botInstance,
			Action:       ActionBan,
			TargetUserID: targetUserID,
			Reason:       reason,
			DeleteDays:   deleteDays,
		})
	case "kick":
		return r.service.EnqueueTask(ModerationJob{
			Ctx:          ctx,
			Bot:          botInstance,
			Action:       ActionKick,
			TargetUserID: targetUserID,
			Reason:       reason,
		})
	default:
		return fmt.Errorf("comando desconhecido: %s", commandName)
	}
}

```

// === FILE: pkg/discord/moderation/service.go ===
```go
package moderation

import (
	"context"
	"log/slog"
)

// Definimos os tipos de ações possíveis (Enum)
type ActionType uint8

const (
	ActionBan ActionType = iota
	ActionKick
	// ActionMute, ActionUnban etc... no futuro
)

// ModerationJob agora carrega a "Intenção" (Action)
type ModerationJob struct {
	Ctx          context.Context
	Bot          *app.BotInstance
	Action       ActionType
	TargetUserID string
	Reason       string
	// Propriedades específicas (o Worker ignora o que não for relevante para a Action)
	DeleteDays int
}

func (s *Service) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-s.jobQueue:
			var err error

			// O Roteador Interno do Worker
			switch job.Action {
			case ActionBan:
				err = s.discord.ExecuteBan(job.Ctx, job.Bot, job.TargetUserID, job.Reason, job.DeleteDays*86400)
			case ActionKick:
				err = s.discord.ExecuteKick(job.Ctx, job.Bot, job.TargetUserID, job.Reason)
			}

			if err != nil {
				slog.Error("Falha na tarefa de moderação",
					"action", job.Action,
					"guild_id", job.Bot.GuildID,
					"error", err)
			}
		}
	}
}

// O método de entrada genérico, que substitui o s.Ban()
func (s *Service) EnqueueTask(job ModerationJob) error {
	select {
	case s.jobQueue <- job:
		// SUCESSO: A tarefa entrou no funil!
		return nil
	default:
		// REJEIÇÃO TÁTICA (LOAD SHEDDING):
		// Se os administradores dispararam 10.000 bans, mas a fila só aguenta 1.000,
		// 9.000 comandos caem aqui instantaneamente em vez de quebrar a memória do bot.
		return ErrModerationQueueFull
	}
}

```

