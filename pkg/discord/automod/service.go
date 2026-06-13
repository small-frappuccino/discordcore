package discordautomod

import (
	"context"
	"sync"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/automod"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/task"
	"github.com/small-frappuccino/discordgo"
)

// automodActionExecutionEventType is the gateway event type Discord uses for
// AutoMod action executions. Mirrored here so the raw *Event handler can
// filter without importing the discordgo-internal constant.
const automodActionExecutionEventType = "AUTO_MODERATION_ACTION_EXECUTION"

// AutoModeration trigger types.
const (
	automodTriggerKeyword       = 1
	automodTriggerHarmfulLink   = 2
	automodTriggerSpam          = 3
	automodTriggerKeywordPreset = 4
	automodTriggerMentionSpam   = 5
	automodTriggerMemberProfile = 6
)

// AutoModeration action types.
const (
	automodActionBlockMessage           = 1
	automodActionSendAlert              = 2
	automodActionTimeout                = 3
	automodActionBlockMemberInteraction = 4
)

// Notifier defines the interface for outbound moderation alerts.
type Notifier interface {
	Send(channelID string, embed *discordgo.MessageEmbed) error
}

// AutomodService listens for Discord native AutoMod executions and routes them to logging.
type AutomodService struct {
	session       *discordgo.Session
	configManager *files.ConfigManager
	adapters      *task.NotificationAdapters
	notifier      Notifier
	isRunning     bool
	handlerCancel func()

	mu        sync.RWMutex
	startTime time.Time

	botInstanceID        string
	defaultBotInstanceID string

	// Fallback dedup cache
	dedupCache *automod.FallbackDedupCache
}

// NewAutomodService news automod service.
func NewAutomodService(session *discordgo.Session, configManager *files.ConfigManager, botInstanceID string, defaultBotInstanceID string) *AutomodService {
	return &AutomodService{
		session:              session,
		configManager:        configManager,
		botInstanceID:        files.NormalizeBotInstanceID(botInstanceID),
		defaultBotInstanceID: files.NormalizeBotInstanceID(defaultBotInstanceID),
		dedupCache:           automod.NewFallbackDedupCache(),
	}
}

// SetAdapters allows wiring TaskRouter adapters for async notifications.
func (as *AutomodService) SetAdapters(adapters *task.NotificationAdapters) {
	as.adapters = adapters
}

// SetNotifier sets the outbound publisher.
func (as *AutomodService) SetNotifier(notifier Notifier) {
	as.notifier = notifier
}

// Name returns the service name.
func (as *AutomodService) Name() string { return "automod" }

// Type returns the service type.
func (as *AutomodService) Type() service.ServiceType { return service.TypeAutomod }

// Priority returns the service startup priority.
func (as *AutomodService) Priority() service.ServicePriority { return service.PriorityNormal }

// Dependencies returns the dependencies.
func (as *AutomodService) Dependencies() []string { return nil }

// IsRunning reports whether the service is running.
func (as *AutomodService) IsRunning() bool {
	as.mu.RLock()
	defer as.mu.RUnlock()
	return as.isRunning
}

// HealthCheck returns the current health status.
func (as *AutomodService) HealthCheck(ctx context.Context) service.HealthStatus {
	return service.HealthStatus{
		Healthy:   true,
		Message:   "Automod Service is active",
		LastCheck: time.Now(),
	}
}

// Stats returns runtime statistics.
func (as *AutomodService) Stats() service.ServiceStats {
	as.mu.RLock()
	defer as.mu.RUnlock()

	var uptime time.Duration
	if as.isRunning {
		uptime = time.Since(as.startTime)
	}

	return service.ServiceStats{
		StartTime: as.startTime,
		Uptime:    uptime,
		Metrics: []service.ServiceMetric{
			{Label: "Status", Value: "Running"},
		},
	}
}

// Start registers handlers.
func (as *AutomodService) Start(ctx context.Context) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	if as.isRunning {
		return nil
	}
	as.isRunning = true
	as.startTime = time.Now()

	if as.session != nil {
		as.handlerCancel = as.session.AddHandler(as.handleRawEvent)
	}
	return nil
}

// Stop stops the service.
func (as *AutomodService) Stop(ctx context.Context) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	if !as.isRunning {
		return nil
	}
	if as.handlerCancel != nil {
		as.handlerCancel()
		as.handlerCancel = nil
	}
	if as.adapters != nil && as.adapters.Router != nil {
		as.adapters.Router.Close()
	}
	as.isRunning = false
	return nil
}
