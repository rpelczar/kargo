package directives

import (
	"context"

	dirsdk "github.com/akuity/kargo/pkg/directives"
)

// mockHealthChecker is a mock implementation of the HealthChecker interface,
// which can be used for testing.
type mockHealthChecker struct {
	*mockPromoter
	// checkFunc is the function that the step should call when CheckHealth is
	// called. If set, this function will be called instead of returning
	// checkResult.
	checkFunc func(context.Context, *dirsdk.HealthCheckStepContext) dirsdk.HealthCheckStepResult
	// checkResult is the result that the HealthChecker should return when
	// CheckHealth is called.
	checkResult dirsdk.HealthCheckStepResult
}

// Name implements the Namer interface.
func (m *mockHealthChecker) Name() string {
	return m.name
}

// CheckHealth implements the HealthChecker interface.
func (m *mockHealthChecker) CheckHealth(
	ctx context.Context,
	stepCtx *dirsdk.HealthCheckStepContext,
) dirsdk.HealthCheckStepResult {
	if m.checkFunc != nil {
		return m.checkFunc(ctx, stepCtx)
	}
	return m.checkResult
}
