package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/small-frappuccino/discordcore/pkg/app"
)

func main() {
	// 1. Configurar Logger Estruturado (Nativo e de Alta Performance)
	// Facilita a observabilidade ao injetar JSON direto no stdout.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// 2. Contexto de Graceful Shutdown amarrado a sinais do SO
	// Interceta Ctrl+C (SIGINT) e pedidos de término do Kubernetes/Docker (SIGTERM)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	slog.Info("A iniciar a frota multi-tenant discordcore...")

	// 3. O "Bootstrapper" da aplicação (Injeção de Dependências pura)
	application, err := app.NewApplication()
	if err != nil {
		slog.Error("Falha fatal ao inicializar a arquitetura", "erro", err)
		os.Exit(1)
	}

	// 4. Bloqueia a thread principal (main) até que o contexto seja cancelado
	if err := application.Run(ctx); err != nil {
		slog.Error("A aplicação encerrou com um erro", "erro", err)
	}

	slog.Info("Desligamento (Graceful Shutdown) concluído de forma segura.")
}
