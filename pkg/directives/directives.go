package directives

import (
	"context"
	"time"
)

// TODO: Document this
type Directive interface {
	// Name returns the name of the PromotionStepRunner.
	Name() string
	// Promote executes an individual step of a user-defined promotion process
	// using the provided PromotionStepContext. Implementations may indirectly
	// modify that context through the returned PromotionStepResult to allow
	// subsequent promotion steps to access the results of its execution.
	Promote(context.Context, *PromotionStepContext) (*PromotionStepResult, error)
	// DefaultTimeout returns the default timeout for the step.
	DefaultTimeout() *time.Duration
	// DefaultErrorThreshold returns the number of consecutive times the step must
	// fail (for any reason) before retries are abandoned and the entire Promotion
	// is marked as failed.
	DefaultErrorThreshold() uint32
	// CheckHealth executes a health check using the provided
	// HealthCheckStepContext.
	CheckHealth(context.Context, *HealthCheckStepContext) *HealthCheckStepResult
}
