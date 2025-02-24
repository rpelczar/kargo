package directives

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	"github.com/akuity/kargo/internal/credentials"
	dirsdk "github.com/akuity/kargo/pkg/directives"
)

func TestSimpleEngine_CheckHealth(t *testing.T) {
	tests := []struct {
		name       string
		healthCtx  dirsdk.HealthCheckContext
		steps      []dirsdk.HealthCheckStep
		assertions func(*testing.T, kargoapi.Health)
	}{
		{
			name: "successful health check",
			steps: []dirsdk.HealthCheckStep{
				{Kind: "success-check"},
			},
			assertions: func(t *testing.T, health kargoapi.Health) {
				assert.Equal(t, kargoapi.HealthStateHealthy, health.Status)
				assert.Empty(t, health.Issues)
				assert.NotNil(t, health.Output)
				assert.JSONEq(t, `[{"test":"success"}]`, string(health.Output.Raw))
			},
		},
		{
			name: "multiple successful health checks",
			steps: []dirsdk.HealthCheckStep{
				{Kind: "success-check"},
				{Kind: "success-check"},
			},
			assertions: func(t *testing.T, health kargoapi.Health) {
				assert.Equal(t, kargoapi.HealthStateHealthy, health.Status)
				assert.Empty(t, health.Issues)
				assert.NotNil(t, health.Output)
				assert.JSONEq(t, `[{"test":"success"},{"test":"success"}]`, string(health.Output.Raw))
			},
		},
		{
			name: "failed health check",
			steps: []dirsdk.HealthCheckStep{
				{Kind: "error-check"},
			},
			assertions: func(t *testing.T, health kargoapi.Health) {
				assert.Equal(t, kargoapi.HealthStateUnhealthy, health.Status)
				assert.Contains(t, health.Issues, "health check failed")
				assert.NotNil(t, health.Output)
			},
		},
		{
			name: "context cancellation",
			steps: []dirsdk.HealthCheckStep{
				{Kind: "context-waiter"},
			},
			assertions: func(t *testing.T, health kargoapi.Health) {
				assert.Equal(t, kargoapi.HealthStateUnknown, health.Status)
				assert.Contains(t, health.Issues, context.Canceled.Error())
				assert.Nil(t, health.Output)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			testRegistry := directivesRegistry{}
			testRegistry.register(
				&mockHealthChecker{
					mockPromoter: &mockPromoter{
						name: "success-check",
					},
					checkResult: dirsdk.HealthCheckStepResult{
						Status: kargoapi.HealthStateHealthy,
						Output: dirsdk.State{"test": "success"},
					},
				},
			)
			testRegistry.register(
				&mockHealthChecker{
					mockPromoter: &mockPromoter{
						name: "error-check",
					},
					checkResult: dirsdk.HealthCheckStepResult{
						Status: kargoapi.HealthStateUnhealthy,
						Issues: []string{"health check failed"},
						Output: dirsdk.State{"test": "error"},
					},
				},
			)
			testRegistry.register(
				&mockHealthChecker{
					mockPromoter: &mockPromoter{
						name: "context-waiter",
					},
					checkFunc: func(ctx context.Context, _ *dirsdk.HealthCheckStepContext) dirsdk.HealthCheckStepResult {
						cancel()
						<-ctx.Done()
						return dirsdk.HealthCheckStepResult{
							Status: kargoapi.HealthStateUnknown,
							Issues: []string{ctx.Err().Error()},
						}
					},
				},
			)

			engine := &SimpleEngine{
				registry: testRegistry,
			}

			health := engine.CheckHealth(ctx, tt.healthCtx, tt.steps)
			tt.assertions(t, health)
		})
	}
}

func TestSimpleEngine_executeHealthChecks(t *testing.T) {
	tests := []struct {
		name       string
		healthCtx  dirsdk.HealthCheckContext
		steps      []dirsdk.HealthCheckStep
		assertions func(*testing.T, kargoapi.HealthState, []string, []dirsdk.State)
	}{
		{
			name: "aggregate multiple healthy checks",
			steps: []dirsdk.HealthCheckStep{
				{Kind: "success-check"},
				{Kind: "success-check"},
			},
			assertions: func(t *testing.T, status kargoapi.HealthState, issues []string, output []dirsdk.State) {
				assert.Equal(t, kargoapi.HealthStateHealthy, status)
				assert.Empty(t, issues)
				assert.Len(t, output, 2)
				for _, o := range output {
					assert.Equal(t, "success", o["test"])
				}
			},
		},
		{
			name: "merge different health states",
			steps: []dirsdk.HealthCheckStep{
				{Kind: "success-check"},
				{Kind: "error-check"},
			},
			assertions: func(t *testing.T, status kargoapi.HealthState, issues []string, output []dirsdk.State) {
				assert.Equal(t, kargoapi.HealthStateUnhealthy, status)
				assert.Contains(t, issues, "health check failed")
				assert.Len(t, output, 2)
			},
		},
		{
			name: "context cancellation",
			steps: []dirsdk.HealthCheckStep{
				{Kind: "context-waiter"},
				{Kind: "success-check"}, // Should not execute
			},
			assertions: func(t *testing.T, status kargoapi.HealthState, issues []string, output []dirsdk.State) {
				assert.Equal(t, kargoapi.HealthStateUnknown, status)
				assert.Contains(t, issues, context.Canceled.Error())
				assert.Empty(t, output)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			testRegistry := directivesRegistry{}
			testRegistry.register(
				&mockHealthChecker{
					mockPromoter: &mockPromoter{
						name: "success-check",
					},
					checkResult: dirsdk.HealthCheckStepResult{
						Status: kargoapi.HealthStateHealthy,
						Output: dirsdk.State{"test": "success"},
					},
				},
			)
			testRegistry.register(
				&mockHealthChecker{
					mockPromoter: &mockPromoter{
						name: "error-check",
					},
					checkResult: dirsdk.HealthCheckStepResult{
						Status: kargoapi.HealthStateUnhealthy,
						Issues: []string{"health check failed"},
						Output: dirsdk.State{"test": "error"},
					},
				},
			)
			testRegistry.register(
				&mockHealthChecker{
					mockPromoter: &mockPromoter{
						name: "context-waiter",
					},
					checkFunc: func(ctx context.Context, _ *dirsdk.HealthCheckStepContext) dirsdk.HealthCheckStepResult {
						cancel()
						<-ctx.Done()
						return dirsdk.HealthCheckStepResult{
							Status: kargoapi.HealthStateUnknown,
							Issues: []string{ctx.Err().Error()},
						}
					},
				},
			)

			engine := &SimpleEngine{
				registry: testRegistry,
			}

			status, issues, output := engine.executeHealthChecks(ctx, tt.healthCtx, tt.steps)
			tt.assertions(t, status, issues, output)
		})
	}
}

func TestSimpleEngine_executeHealthCheck(t *testing.T) {
	tests := []struct {
		name       string
		healthCtx  dirsdk.HealthCheckContext
		step       dirsdk.HealthCheckStep
		assertions func(*testing.T, dirsdk.HealthCheckStepResult)
	}{
		{
			name: "successful execution",
			step: dirsdk.HealthCheckStep{Kind: "success-check"},
			assertions: func(t *testing.T, result dirsdk.HealthCheckStepResult) {
				assert.Equal(t, kargoapi.HealthStateHealthy, result.Status)
				assert.Empty(t, result.Issues)
			},
		},
		{
			name: "unregistered directive",
			step: dirsdk.HealthCheckStep{Kind: "unknown"},
			assertions: func(t *testing.T, result dirsdk.HealthCheckStepResult) {
				assert.Equal(t, kargoapi.HealthStateUnknown, result.Status)
				assert.Contains(t, result.Issues[0], "no HealthChecker registered for health check kind")
				assert.Contains(t, result.Issues[0], "unknown")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testRegistry := directivesRegistry{}
			testRegistry.register(
				&mockHealthChecker{
					mockPromoter: &mockPromoter{
						name: "success-check",
					},
					checkResult: dirsdk.HealthCheckStepResult{
						Status: kargoapi.HealthStateHealthy,
					},
				},
			)

			engine := &SimpleEngine{
				registry: testRegistry,
			}

			result := engine.executeHealthCheck(context.Background(), tt.healthCtx, tt.step)
			tt.assertions(t, result)
		})
	}
}

func TestSimpleEngine_prepareHealthCheckStepContext(t *testing.T) {
	tests := []struct {
		name       string
		healthCtx  dirsdk.HealthCheckContext
		step       dirsdk.HealthCheckStep
		assertions func(*testing.T, context.Context, *dirsdk.HealthCheckStepContext)
	}{
		{
			name: "success",
			healthCtx: dirsdk.HealthCheckContext{
				Project: "test-project",
				Stage:   "test-stage",
			},
			step: dirsdk.HealthCheckStep{
				Config: map[string]any{
					"key": "value",
				},
			},
			assertions: func(t *testing.T, ctx context.Context, stepCtx *dirsdk.HealthCheckStepContext) {
				assert.Equal(t, "test-project", stepCtx.Project)
				assert.Equal(t, "test-stage", stepCtx.Stage)
				assert.NotNil(t, stepCtx.Config)
				assert.NotNil(t, ctx.Value(kargoClientContextKey{}))
				assert.NotNil(t, ctx.Value(argoCDClientContextKey{}))
				assert.NotNil(t, ctx.Value(credentialsDBContextKey{}))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := &SimpleEngine{
				credentialsDB: &credentials.FakeDB{},
				kargoClient:   fake.NewClientBuilder().Build(),
				argoCDClient:  fake.NewClientBuilder().Build(),
			}

			ctx, stepCtx := engine.prepareHealthCheckStepContext(
				context.Background(),
				tt.healthCtx,
				tt.step,
			)
			tt.assertions(t, ctx, stepCtx)
		})
	}
}
