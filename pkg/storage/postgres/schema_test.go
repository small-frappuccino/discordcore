package postgres

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupTestDB starts a postgres testcontainer, creates the full DDL schema, and returns a db connection string.
func setupTestDB(ctx context.Context, t *testing.T) (string, func()) {
	t.Helper()

	postgresContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	cleanup := func() {
		if err := postgresContainer.Terminate(context.Background()); err != nil {
			t.Fatalf("failed to terminate container: %v", err)
		}
	}

	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		cleanup()
		t.Fatalf("failed to get connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		cleanup()
		t.Fatalf("failed to connect to postgres: %v", err)
	}
	defer pool.Close()

	// Apply base DDL mapping from schema_test.go
	ddl := `
CREATE TABLE member_current (
	guild_id text NOT NULL,
	user_id text NOT NULL,
	roles text[],
	updated_at timestamptz
);
CREATE TABLE avatars_current (
	guild_id text NOT NULL,
	user_id text NOT NULL,
	avatar_hash text,
	updated_at timestamptz
);
CREATE TABLE member_joins (
	guild_id text NOT NULL,
	user_id text NOT NULL,
	joined_at timestamptz,
	last_seen_at timestamptz,
	is_bot boolean,
	left_at timestamptz
);
CREATE TABLE messages_history (
	guild_id text NOT NULL,
	message_id text NOT NULL,
	channel_id text NOT NULL,
	author_id text NOT NULL,
	version int NOT NULL,
	event_type text NOT NULL,
	content text,
	attachments int,
	embeds int,
	stickers int,
	created_at timestamptz NOT NULL
);
CREATE TABLE message_version_counters (
	guild_id text NOT NULL,
	message_id text NOT NULL
);
CREATE TABLE guild_meta (
	guild_id text NOT NULL,
	heartbeat timestamptz,
	last_event timestamptz
);
CREATE TABLE runtime_meta (
	guild_id text NOT NULL,
	bot_heartbeat timestamptz,
	bot_last_event timestamptz
);
CREATE TABLE moderation_cases (
	guild_id text NOT NULL,
	case_id int NOT NULL
);
CREATE TABLE moderation_warnings (
	guild_id text NOT NULL,
	warning_id int NOT NULL
);
CREATE TABLE qotd_questions (
	guild_id text NOT NULL,
	question_id int NOT NULL
);
CREATE TABLE qotd_answers (
	guild_id text NOT NULL,
	answer_id int NOT NULL
);
CREATE TABLE system_events (
	guild_id text NOT NULL,
	event_id int NOT NULL
);
CREATE TABLE app_meta (
	key text NOT NULL,
	value text NOT NULL
);
CREATE TABLE daily_member_joins (
	guild_id text NOT NULL,
	date date NOT NULL,
	count int NOT NULL
);
CREATE TABLE daily_member_leaves (
	guild_id text NOT NULL,
	date date NOT NULL,
	count int NOT NULL
);
CREATE TABLE daily_message_metrics (
	guild_id text NOT NULL,
	date date NOT NULL,
	count int NOT NULL
);
`
	if _, err := pool.Exec(ctx, ddl); err != nil {
		cleanup()
		t.Fatalf("failed to apply DDL: %v", err)
	}

	return connStr, cleanup
}

func TestStore_Schema_ParametricDeletion(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("skipping testcontainers-go tests on local windows environment due to rootless docker limitation")
	}
	// REQUIREMENT: Deleção Paramétrica usando testcontainers-go
	ctx := context.Background()
	connStr, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("failed to connect pool: %v", err)
	}
	defer pool.Close()

	// Simulate parametric deletion
	if _, err := pool.Exec(ctx, "ALTER TABLE avatars_current DROP COLUMN updated_at"); err != nil {
		t.Fatalf("failed to drop column: %v", err)
	}

	store, err := NewStore(pool, nil)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	err = store.Init()
	if err == nil {
		t.Fatal("expected Init() to fail due to missing column, but it succeeded")
	}
	if !strings.Contains(err.Error(), "column avatars_current.updated_at missing") {
		t.Errorf("expected missing column error, got: %v", err)
	}
}

func TestStore_Schema_TypeRegression(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("skipping testcontainers-go tests on local windows environment due to rootless docker limitation")
	}
	// REQUIREMENT: Regressão de Tipo usando testcontainers-go
	ctx := context.Background()
	connStr, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("failed to connect pool: %v", err)
	}
	defer pool.Close()

	// Simulate type regression
	if _, err := pool.Exec(ctx, "ALTER TABLE avatars_current ALTER COLUMN guild_id TYPE bigint USING guild_id::bigint"); err != nil {
		// Because it's an empty table, the cast works.
	}

	store, err := NewStore(pool, nil)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	err = store.Init()
	if err == nil {
		t.Fatal("expected Init() to fail due to type regression, but it succeeded")
	}
	if !strings.Contains(err.Error(), "type mismatch") {
		t.Errorf("expected type mismatch error, got: %v", err)
	}
}
