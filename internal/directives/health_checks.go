package directives

import (
	"context"

	dirsdk "github.com/akuity/kargo/pkg/directives"
)

// HealthChecker is an interface for directives that can execute a HealthCheck.
type HealthChecker interface {
	// CheckHealth executes a health check using the provided
	// dirsdk.HealthCheckContext.
	CheckHealth(context.Context, *dirsdk.HealthCheckStepContext) dirsdk.HealthCheckStepResult
}

// HealthCheck describes a health check. HealthChecks are executed in sequence
// by the Engine, which delegates the execution of each step to a HealthChecker.
type HealthCheck struct {
	// Kind identifies a registered HealthChecker.
	Kind string
	// Config is an opaque map of configuration values to be passed to the
	// HealthChecker.
	Config dirsdk.Config
}
