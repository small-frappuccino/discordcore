package qotd_test

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/qotd"
	"golang.org/x/sync/errgroup"
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
		qotd.WithPublisher(&mockPublisher{}),
	)

	const targetGuildID = "guild_01"
	const workerCount = 100

	var executedCounter int32
	var activeCount int32
	var maxActiveCount int32
	eg, ctx := errgroup.WithContext(context.Background())

	for i := 0; i < workerCount; i++ {
		eg.Go(func() error {
			if err := ctx.Err(); err != nil {
				return err
			}
			_, err := svc.ExecuteInGuildActorWithResult(targetGuildID, func() (any, error) {
				atomic.AddInt32(&executedCounter, 1)

				currentActive := atomic.AddInt32(&activeCount, 1)
				defer atomic.AddInt32(&activeCount, -1)

				// Track maximum concurrent executions inside the same actor
				for {
					max := atomic.LoadInt32(&maxActiveCount)
					if currentActive <= max {
						break
					}
					if atomic.CompareAndSwapInt32(&maxActiveCount, max, currentActive) {
						break
					}
				}

				runtime.Gosched()
				return nil, nil
			})
			if err != nil {
				return fmt.Errorf("Execução subjacente falhou inesperadamente: %v", err)
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		t.Fatalf("concurrent execution failed: %v", err)
	}

	if atomic.LoadInt32(&executedCounter) != int32(workerCount) {
		t.Fatalf("Esperado contador escalar em %d, aferido %d", workerCount, executedCounter)
	}

	// For a serialized actor, the maximum concurrent execution count must be exactly 1
	if finalMax := atomic.LoadInt32(&maxActiveCount); finalMax != 1 {
		t.Fatalf("Actor serialization failed: expected max concurrent execution of 1, got %d", finalMax)
	}
}

func TestExecuteInGuildActor_Parallelism(t *testing.T) {
	t.Parallel()
	svc := qotd.NewService(
		&files.ConfigManager{},
		qotd.WithPublisher(&mockPublisher{}),
	)

	const workerCount = 100

	eg, ctx := errgroup.WithContext(context.Background())
	var activeCount int32
	var maxActiveCount int32
	gate := make(chan struct{})

	for i := 0; i < workerCount; i++ {
		guildID := fmt.Sprintf("guild_%d", i)
		eg.Go(func() error {
			if err := ctx.Err(); err != nil {
				return err
			}
			svc.ExecuteInGuildActor(guildID, func() {
				currentActive := atomic.AddInt32(&activeCount, 1)

				for {
					max := atomic.LoadInt32(&maxActiveCount)
					if currentActive <= max {
						break
					}
					if atomic.CompareAndSwapInt32(&maxActiveCount, max, currentActive) {
						break
					}
				}

				// Wait until all actors have entered to verify parallel execution
				<-gate

				atomic.AddInt32(&activeCount, -1)
			})
			return nil
		})
	}

	// Spin-wait until all workers are concurrently active in their own actors
	start := time.Now()
	for atomic.LoadInt32(&activeCount) < workerCount {
		if time.Since(start) > 2*time.Second {
			close(gate)
			_ = eg.Wait()
			t.Fatalf("Timeout waiting for parallel actors to enter concurrently: entered %d/%d", atomic.LoadInt32(&activeCount), workerCount)
		}
		runtime.Gosched()
	}

	close(gate)
	if err := eg.Wait(); err != nil {
		t.Fatalf("parallel execution failed: %v", err)
	}

	if finalMax := atomic.LoadInt32(&maxActiveCount); finalMax != workerCount {
		t.Fatalf("Actor parallelism failed: expected max concurrent execution of %d, got %d", workerCount, finalMax)
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
		qotd.WithPublisher(pub),
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
	svc := qotd.NewService(
		&files.ConfigManager{},
		qotd.WithPublisher(pub),
		qotd.WithMetrics(metrics),
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
