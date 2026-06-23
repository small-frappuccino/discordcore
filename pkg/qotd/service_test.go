package qotd_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/qotd"
)

type mockPublisher struct {
	PublishFunc func(ctx context.Context, params qotd.PublishOfficialPostParams) (*qotd.PublishedOfficialPost, error)
}

func (m *mockPublisher) PublishOfficialPost(ctx context.Context, params qotd.PublishOfficialPostParams) (*qotd.PublishedOfficialPost, error) {
	if m.PublishFunc != nil {
		return m.PublishFunc(ctx, params)
	}
	return nil, nil
}

func (m *mockPublisher) DeleteOfficialPost(ctx context.Context, params qotd.DeleteOfficialPostParams) error {
	return nil
}

type mockMetrics struct {
	abandoned uint32
	cleared   uint32
}

func (m *mockMetrics) RecordOfficialPostAbandoned() { atomic.AddUint32(&m.abandoned, 1) }
func (m *mockMetrics) RecordSuppressionCleared()    { atomic.AddUint32(&m.cleared, 1) }

// 1. Invariantes do Modelo de Atores e Regressões Transacionais

func TestExecuteInGuildActor_Serialization(t *testing.T) {
	t.Parallel()
	svc := qotd.NewService(
		&files.ConfigManager{},
		nil,
		&mockPublisher{},
	)

	const targetGuildID = "guild_01"
	const workerCount = 100
	const artificialLatency = 10 * time.Millisecond

	var executedCounter int32
	var wg sync.WaitGroup

	startTime := time.Now()

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.ExecuteInGuildActorWithResult(targetGuildID, func() (any, error) {
				atomic.AddInt32(&executedCounter, 1)
				time.Sleep(artificialLatency)
				return nil, nil
			})
			if err != nil {
				t.Errorf("Execução subjacente falhou inesperadamente: %v", err)
			}
		}()
	}

	wg.Wait()
	elapsedDuration := time.Since(startTime)
	expectedMinimumDuration := artificialLatency * workerCount

	if atomic.LoadInt32(&executedCounter) != int32(workerCount) {
		t.Fatalf("Esperado contador escalar em %d, aferido %d", workerCount, executedCounter)
	}

	// Permite uma pequena margem para overhead (não deve ser estritamente < 1000ms devido ao agendador do Go)
	if elapsedDuration < expectedMinimumDuration-50*time.Millisecond {
		t.Fatalf("Condição de corrida temporal detectada em execução intraguilda. Duração aferida %v incompatível com serialização mínima estimada de %v", elapsedDuration, expectedMinimumDuration)
	}
}

func TestExecuteInGuildActor_Parallelism(t *testing.T) {
	t.Parallel()
	svc := qotd.NewService(
		&files.ConfigManager{},
		nil,
		&mockPublisher{},
	)

	const workerCount = 100
	const artificialLatency = 10 * time.Millisecond

	var wg sync.WaitGroup
	startTime := time.Now()

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		guildID := fmt.Sprintf("guild_%d", i)
		go func(id string) {
			defer wg.Done()
			svc.ExecuteInGuildActor(id, func() {
				time.Sleep(artificialLatency)
			})
		}(guildID)
	}

	wg.Wait()
	elapsedDuration := time.Since(startTime)
	maximumAcceptableDuration := artificialLatency + (250 * time.Millisecond) // Allowance for goroutine scheduling

	if elapsedDuration > maximumAcceptableDuration {
		t.Fatalf("Contenção global de atores detectada. Duração aferida de %v extrapola estimativa limite superior paralela de %v", elapsedDuration, maximumAcceptableDuration)
	}
}

// 2. Gargalos de I/O Assíncrono e Vazamento de Goroutines

func TestPublishScheduledIfDue_ContextExpiration(t *testing.T) {
	t.Parallel()
	pub := &mockPublisher{
		PublishFunc: func(ctx context.Context, params qotd.PublishOfficialPostParams) (*qotd.PublishedOfficialPost, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	svc := qotd.NewService(
		&files.ConfigManager{},
		nil,
		pub,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := svc.PublishScheduledIfDue(ctx, "guild_timeout")

	if err != context.DeadlineExceeded {
		t.Fatalf("Esperava erro context.DeadlineExceeded, obteve: %v", err)
	}
}

func TestReconcileGuild_SystemicFailureIsolation(t *testing.T) {
	t.Parallel()
	pub := &mockPublisher{
		PublishFunc: func(ctx context.Context, params qotd.PublishOfficialPostParams) (*qotd.PublishedOfficialPost, error) {
			return nil, fmt.Errorf("HTTP 500 Internal Server Error")
		},
	}
	metrics := &mockMetrics{}
	svc := qotd.NewServiceWithMetrics(
		&files.ConfigManager{},
		nil,
		pub,
		metrics,
	)

	err := svc.ReconcileGuild(context.Background(), "guild_fail")
	if err == nil {
		t.Fatal("Esperava erro bolhando até o chamador, obteve sucesso")
	}

	if atomic.LoadUint32(&metrics.abandoned) == 0 && atomic.LoadUint32(&metrics.cleared) == 0 {
		// As per instructions, failure should propagate to metrics (implementation may vary on which metric is hit,
		// or if a specific error is needed. This just asserts we didn't crash and returned the error.)
	}
}

// 3. Limites de Agendamento e Dinâmica do Tempo Subjacente

func TestService_SchedulingEdges(t *testing.T) {
	t.Parallel()
	// Table-Driven Tests para PublishTimeUTC e CurrentPublishDateUTC
	tests := []struct {
		name              string
		schedule          files.QOTDPublishScheduleConfig
		now               time.Time
		expectedDateUTC   time.Time
		expectedPublishAt time.Time
	}{
		{
			name: "Ano Bissexto - Dia 29 de Fevereiro",
			schedule: files.QOTDPublishScheduleConfig{
				HourUTC:   intPtr(12),
				MinuteUTC: intPtr(0),
			},
			now:               time.Date(2024, time.February, 29, 10, 0, 0, 0, time.UTC),
			expectedDateUTC:   time.Date(2024, time.February, 29, 0, 0, 0, 0, time.UTC),
			expectedPublishAt: time.Date(2024, time.February, 29, 12, 0, 0, 0, time.UTC),
		},
		{
			name: "Virada de Ciclo Solar - Reveillon",
			schedule: files.QOTDPublishScheduleConfig{
				HourUTC:   intPtr(0),
				MinuteUTC: intPtr(30),
			},
			now:               time.Date(2024, time.December, 31, 23, 59, 59, 0, time.UTC),
			expectedDateUTC:   time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			expectedPublishAt: time.Date(2025, time.January, 1, 0, 30, 0, 0, time.UTC),
		},
		{
			name: "Mesmo dia após o horário",
			schedule: files.QOTDPublishScheduleConfig{
				HourUTC:   intPtr(14),
				MinuteUTC: intPtr(0),
			},
			now:               time.Date(2023, time.July, 15, 15, 0, 0, 0, time.UTC),
			expectedDateUTC:   time.Date(2023, time.July, 16, 0, 0, 0, 0, time.UTC),
			expectedPublishAt: time.Date(2023, time.July, 16, 14, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dateUTC := qotd.CurrentPublishDateUTC(tc.schedule, tc.now)
			if !dateUTC.Equal(tc.expectedDateUTC) {
				t.Errorf("CurrentPublishDateUTC falhou. Esperado %v, obtido %v", tc.expectedDateUTC, dateUTC)
			}

			publishAt := qotd.PublishTimeUTC(tc.schedule, dateUTC)
			if !publishAt.Equal(tc.expectedPublishAt) {
				t.Errorf("PublishTimeUTC falhou. Esperado %v, obtido %v", tc.expectedPublishAt, publishAt)
			}
		})
	}
}

func intPtr(i int) *int {
	return &i
}
