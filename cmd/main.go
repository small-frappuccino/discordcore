package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/lib/pq"
	"github.com/small-frappuccino/discordcore/pkg/app"
)

func main() {
	// 1. Configurar Logger Estruturado (Nativo e de Alta Performance)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// 2. Contexto de Graceful Shutdown amarrado a sinais do SO
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	slog.Info("A iniciar a frota multi-tenant discordcore...")

	// Inicializar a ligação à base de dados PostgreSQL
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/discordcore?sslmode=disable"
	}
	dbConn, err := sql.Open("postgres", dsn)
	if err != nil {
		slog.Error("Falha ao abrir ligação à base de dados", "erro", err)
		os.Exit(1)
	}
	defer dbConn.Close()

	// 3. O "Bootstrapper" da aplicação (Injeção de Dependências pura)
	application, err := app.NewApplication(dbConn)
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
