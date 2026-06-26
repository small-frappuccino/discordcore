package app

import (
	"context"
	"time"
)

// Suponha que esta função seja o loop contínuo de leitura do WebSocket do Discord
// ou o Handler do servidor HTTP Webhook.
func (g *DiscordGateway) ListenLoop(ctx context.Context) {
	for {
		// Lê os bytes puros da conexão (Zero-allocation até aqui)
		payload, err := g.connection.ReadMessage()
		if err != nil {
			break
		}

		// DISPARO MASSIVO: Criamos uma goroutine IMEDIATAMENTE para processar o payload.
		// Go consegue levantar 100.000 goroutines em milissegundos.
		// Isso libera o loop instantaneamente para ler o próximo evento da rede.
		go func(p []byte) {
			// Usamos um timeout no contexto para garantir que nenhuma goroutine viva para sempre
			routeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			// Chama o roteador central
			if err := g.router.HandleInteraction(routeCtx, p); err != nil {
				// Ignoramos erros normais como ErrModerationQueueFull no log crítico,
				// pois o Load Shedding é um comportamento esperado sob ataque.
			}
		}(payload)
	}
}
