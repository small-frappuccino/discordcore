package errors

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

// ErrorCategory represents different types of errors in the system
type ErrorCategory string

const (
	CategoryService    ErrorCategory = "service"
	CategoryDiscord    ErrorCategory = "discord"
	CategoryCache      ErrorCategory = "cache"
	CategoryConfig     ErrorCategory = "config"
	CategoryCommand    ErrorCategory = "command"
	CategoryValidation ErrorCategory = "validation"
	CategoryNetwork    ErrorCategory = "network"
	CategoryInternal   ErrorCategory = "internal"
)

// ErrorSeverity represents the severity level of errors
type ErrorSeverity string

const (
	SeverityLow      ErrorSeverity = "low"
	SeverityMedium   ErrorSeverity = "medium"
	SeverityHigh     ErrorSeverity = "high"
	SeverityCritical ErrorSeverity = "critical"
)

// ErrorAction represents what action should be taken for an error
type ErrorAction string

const (
	ActionLog       ErrorAction = "log"
	ActionRetry     ErrorAction = "retry"
	ActionRestart   ErrorAction = "restart"
	ActionNotify    ErrorAction = "notify"
	ActionTerminate ErrorAction = "terminate"
)

// ServiceError represents a standardized error in the system
type ServiceError struct {
	Category    ErrorCategory          `json:"category"`
	Severity    ErrorSeverity          `json:"severity"`
	Message     string                 `json:"message"`
	Operation   string                 `json:"operation"`
	Component   string                 `json:"component"`
	Cause       error                  `json:"-"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Recoverable bool                   `json:"recoverable"`
	Actions     []ErrorAction          `json:"actions"`
}

func (e ServiceError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s:%s] %s in %s.%s: %v", e.Category, e.Severity, e.Message, e.Component, e.Operation, e.Cause)
	}
	return fmt.Sprintf("[%s:%s] %s in %s.%s", e.Category, e.Severity, e.Message, e.Component, e.Operation)
}

func (e ServiceError) Unwrap() error {
	return e.Cause
}

// ErrorHandler provides centralized error handling for the entire system
type ErrorHandler struct {
	logger          *log.Logger
	notifiers       []ErrorNotifier
	retryStrategies map[ErrorCategory]RetryStrategy
	circuits        map[string]*CircuitBreaker
}

// ErrorNotifier defines how errors should be reported
type ErrorNotifier interface {
	NotifyError(ctx context.Context, err *ServiceError) error
}

// RetryStrategy defines retry behavior for different error categories
type RetryStrategy struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
}

// CircuitBreaker prevents cascading failures
type CircuitBreaker struct {
	Name         string
	FailureCount int
	LastFailure  time.Time
	State        CircuitState
	Threshold    int
	Timeout      time.Duration
}

type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitOpen     CircuitState = "open"
	CircuitHalfOpen CircuitState = "half_open"
)

// NewErrorHandler creates a new unified error handler
func NewErrorHandler() *ErrorHandler {
	return &ErrorHandler{
		logger:    log.GlobalLogger, // Field-based loggers are deprecated
		notifiers: make([]ErrorNotifier, 0),
		retryStrategies: map[ErrorCategory]RetryStrategy{
			CategoryDiscord: {
				MaxAttempts: 3,
				BaseDelay:   1 * time.Second,
				MaxDelay:    10 * time.Second,
				Multiplier:  2.0,
			},
			CategoryNetwork: {
				MaxAttempts: 5,
				BaseDelay:   500 * time.Millisecond,
				MaxDelay:    30 * time.Second,
				Multiplier:  2.0,
			},
			CategoryService: {
				MaxAttempts: 2,
				BaseDelay:   2 * time.Second,
				MaxDelay:    20 * time.Second,
				Multiplier:  3.0,
			},
		},
		circuits: make(map[string]*CircuitBreaker),
	}
}

// AddNotifier adds an error notifier
func (eh *ErrorHandler) AddNotifier(notifier ErrorNotifier) {
	eh.notifiers = append(eh.notifiers, notifier)
}

// Handle processes an error according to the unified strategy
func (eh *ErrorHandler) Handle(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	serviceErr := eh.normalizeError(err)
	eh.logError(serviceErr)

	// Execute actions based on error configuration
	for _, action := range serviceErr.Actions {
		switch action {
		case ActionNotify:
			eh.notifyError(ctx, serviceErr)
		case ActionRetry:
			// Retry logic is handled by HandleWithRetry
		case ActionRestart:
			log.ApplicationLogger().Warn("Service restart required", "component", serviceErr.Component, "operation", serviceErr.Operation)
		case ActionTerminate:
			log.ErrorLoggerRaw().Error("Critical error - terminating", "component", serviceErr.Component, "operation", serviceErr.Operation)
			os.Exit(1)
		}
	}

	return serviceErr
}

// HandleWithRetry executes an operation with retry logic
func (eh *ErrorHandler) HandleWithRetry(ctx context.Context, operation string, component string, fn func() error) error {
	var lastErr error

	for attempt := 1; attempt <= 3; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		serviceErr := eh.normalizeError(err)
		serviceErr.Component = component
		serviceErr.Operation = operation

		if !serviceErr.Recoverable {
			return eh.Handle(ctx, serviceErr)
		}

		strategy, hasStrategy := eh.retryStrategies[serviceErr.Category]
		if !hasStrategy || attempt >= strategy.MaxAttempts {
			return eh.Handle(ctx, serviceErr)
		}

		delay := eh.calculateDelay(strategy, attempt)
		log.ApplicationLogger().Warn("Operation failed, retrying", "attempt", attempt, "delay", delay, "component", component, "operation", operation, "err", err)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			lastErr = serviceErr
		}
	}

	return eh.Handle(ctx, lastErr)
}

// HandleDiscordError specifically handles Discord API errors
func (eh *ErrorHandler) HandleDiscordError(ctx context.Context, operation string, component string, err error) error {
	if err == nil {
		return nil
	}

	serviceErr := &ServiceError{
		Category:    CategoryDiscord,
		Message:     "Discord API operation failed",
		Operation:   operation,
		Component:   component,
		Cause:       err,
		Timestamp:   time.Now(),
		Recoverable: eh.isDiscordErrorRecoverable(err),
		Context:     make(map[string]interface{}),
	}

	// Parse Discord-specific error details
	if restErr, ok := err.(*discordgo.RESTError); ok {
		serviceErr.Context["discord_code"] = restErr.Message.Code
		serviceErr.Context["discord_message"] = restErr.Message.Message
		serviceErr.Severity = eh.getDiscordErrorSeverity(restErr.Message.Code)

		// Rate limiting
		if restErr.Message.Code == 429 {
			serviceErr.Actions = []ErrorAction{ActionLog, ActionRetry}
			if retryAfter, ok := restErr.Response.Header["Retry-After"]; ok && len(retryAfter) > 0 {
				serviceErr.Context["retry_after"] = retryAfter[0]
			}
		} else {
			serviceErr.Actions = eh.getDiscordErrorActions(restErr.Message.Code)
		}
	} else {
		serviceErr.Severity = SeverityMedium
		serviceErr.Actions = []ErrorAction{ActionLog, ActionRetry}
	}

	return eh.Handle(ctx, serviceErr)
}

// normalizeError converts any error into a ServiceError
func (eh *ErrorHandler) normalizeError(err error) *ServiceError {
	if serviceErr, ok := err.(*ServiceError); ok {
		return serviceErr
	}

	// Try to categorize based on error type and message
	category := eh.categorizeError(err)
	severity := eh.getSeverityForCategory(category)

	return &ServiceError{
		Category:    category,
		Severity:    severity,
		Message:     err.Error(),
		Operation:   "unknown",
		Component:   "unknown",
		Cause:       err,
		Timestamp:   time.Time{},
		Recoverable: eh.isErrorRecoverable(err),
		Actions:     eh.getDefaultActions(category),
		Context:     make(map[string]interface{}),
	}
}

// categorizeError attempts to categorize an error based on its type and message
func (eh *ErrorHandler) categorizeError(err error) ErrorCategory {
	errStr := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errStr, "discord") || strings.Contains(errStr, "gateway"):
		return CategoryDiscord
	case strings.Contains(errStr, "cache") || strings.Contains(errStr, "key not found"):
		return CategoryCache
	case strings.Contains(errStr, "config") || strings.Contains(errStr, "guild"):
		return CategoryConfig
	case strings.Contains(errStr, "command") || strings.Contains(errStr, "interaction"):
		return CategoryCommand
	case strings.Contains(errStr, "network") || strings.Contains(errStr, "connection") || strings.Contains(errStr, "timeout"):
		return CategoryNetwork
	case strings.Contains(errStr, "validation") || strings.Contains(errStr, "invalid"):
		return CategoryValidation
	default:
		return CategoryInternal
	}
}

// logError logs the error using the appropriate severity level
func (eh *ErrorHandler) logError(err *ServiceError) {
	// Flatten context into the message string
	var contextBuilder strings.Builder
	first := true
	for k, v := range err.Context {
		if !first {
			contextBuilder.WriteString(", ")
		}
		contextBuilder.WriteString(fmt.Sprintf("%s: %v", k, v))
		first = false
	}

	fullMessage := fmt.Sprintf("%s. Category: %s, Severity: %s, Component: %s, Operation: %s, Recoverable: %v, Actions: %v, Context: {%s}",
		err.Message, err.Category, err.Severity, err.Component, err.Operation, err.Recoverable, err.Actions, contextBuilder.String())

	switch err.Severity {
	case SeverityLow:
		log.ApplicationLogger().Info(fullMessage)
	case SeverityMedium:
		log.ApplicationLogger().Info(fullMessage)
	case SeverityHigh:
		log.ApplicationLogger().Warn(fullMessage)
	case SeverityCritical:
		log.ErrorLoggerRaw().Error(fullMessage)
	}
}

// notifyError sends the error to all registered notifiers
func (eh *ErrorHandler) notifyError(ctx context.Context, err *ServiceError) {
	for _, notifier := range eh.notifiers {
		if notifyErr := notifier.NotifyError(ctx, err); notifyErr != nil {
			log.ErrorLoggerRaw().Error("Failed to notify error", "err", notifyErr)
		}
	}
}

// Helper methods for error classification and handling

func (eh *ErrorHandler) isDiscordErrorRecoverable(err error) bool {
	if restErr, ok := err.(*discordgo.RESTError); ok {
		code := restErr.Message.Code
		// Rate limits, server errors, and temporary issues are recoverable
		return code == 429 || (code >= 500 && code < 600) || code == 502 || code == 503 || code == 504
	}
	return true // Default to recoverable for unknown Discord errors
}

func (eh *ErrorHandler) isErrorRecoverable(err error) bool {
	// Default recovery logic based on error patterns
	errStr := strings.ToLower(err.Error())
	nonRecoverablePatterns := []string{"permission denied", "unauthorized", "not found", "invalid token"}

	for _, pattern := range nonRecoverablePatterns {
		if strings.Contains(errStr, pattern) {
			return false
		}
	}
	return true
}

func (eh *ErrorHandler) getDiscordErrorSeverity(code int) ErrorSeverity {
	switch {
	case code >= 400 && code < 500:
		if code == 429 {
			return SeverityMedium // Rate limiting
		}
		return SeverityHigh // Client errors
	case code >= 500:
		return SeverityCritical // Server errors
	default:
		return SeverityMedium
	}
}

func (eh *ErrorHandler) getDiscordErrorActions(code int) []ErrorAction {
	switch {
	case code == 429:
		return []ErrorAction{ActionLog, ActionRetry}
	case code >= 500:
		return []ErrorAction{ActionLog, ActionRetry, ActionNotify}
	case code == 401 || code == 403:
		return []ErrorAction{ActionLog, ActionNotify}
	default:
		return []ErrorAction{ActionLog}
	}
}

func (eh *ErrorHandler) getSeverityForCategory(category ErrorCategory) ErrorSeverity {
	switch category {
	case CategoryService:
		return SeverityHigh
	case CategoryDiscord:
		return SeverityMedium
	case CategoryCache:
		return SeverityLow
	case CategoryConfig:
		return SeverityMedium
	case CategoryValidation:
		return SeverityLow
	default:
		return SeverityMedium
	}
}

func (eh *ErrorHandler) getDefaultActions(category ErrorCategory) []ErrorAction {
	switch category {
	case CategoryService:
		return []ErrorAction{ActionLog, ActionNotify}
	case CategoryDiscord:
		return []ErrorAction{ActionLog, ActionRetry}
	case CategoryCache:
		return []ErrorAction{ActionLog}
	case CategoryConfig:
		return []ErrorAction{ActionLog, ActionNotify}
	default:
		return []ErrorAction{ActionLog}
	}
}

func (eh *ErrorHandler) calculateDelay(strategy RetryStrategy, attempt int) time.Duration {
	delay := time.Duration(float64(strategy.BaseDelay) * eh.power(strategy.Multiplier, float64(attempt-1)))
	if delay > strategy.MaxDelay {
		delay = strategy.MaxDelay
	}
	return delay
}

func (eh *ErrorHandler) power(base, exp float64) float64 {
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return result
}

// NewServiceError creates a new service error with the specified parameters
func NewServiceError(category ErrorCategory, severity ErrorSeverity, component, operation, message string, cause error) *ServiceError {
	return &ServiceError{
		Category:    category,
		Severity:    severity,
		Message:     message,
		Operation:   operation,
		Component:   component,
		Cause:       cause,
		Timestamp:   time.Now(),
		Recoverable: true, // Default to recoverable
		Actions:     []ErrorAction{ActionLog},
		Context:     make(map[string]interface{}),
	}
}
