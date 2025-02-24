package directives

import (
	"context"
	"encoding/json"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	dirsdk "github.com/akuity/kargo/pkg/directives"
)

// CheckHealth implements the Engine interface.
func (e *SimpleEngine) CheckHealth(
	ctx context.Context,
	healthCtx dirsdk.HealthCheckContext,
	steps []dirsdk.HealthCheckStep,
) kargoapi.Health {
	status, issues, output := e.executeHealthChecks(ctx, healthCtx, steps)
	if len(output) == 0 {
		return kargoapi.Health{
			Status: status,
			Issues: issues,
		}
	}

	b, err := json.Marshal(output)
	if err != nil {
		issues = append(issues, fmt.Sprintf("failed to marshal health output: %s", err.Error()))
	}

	return kargoapi.Health{
		Status: status,
		Issues: issues,
		Output: &apiextensionsv1.JSON{Raw: b},
	}
}

// executeHealthChecks executes a list of HealthCheckSteps in sequence.
func (e *SimpleEngine) executeHealthChecks(
	ctx context.Context,
	healthCtx dirsdk.HealthCheckContext,
	steps []dirsdk.HealthCheckStep,
) (kargoapi.HealthState, []string, []dirsdk.State) {
	var (
		aggregatedStatus = kargoapi.HealthStateHealthy
		aggregatedIssues []string
		aggregatedOutput = make([]dirsdk.State, 0, len(steps))
	)

	for _, step := range steps {
		select {
		case <-ctx.Done():
			aggregatedStatus = aggregatedStatus.Merge(kargoapi.HealthStateUnknown)
			aggregatedIssues = append(aggregatedIssues, ctx.Err().Error())
			return aggregatedStatus, aggregatedIssues, aggregatedOutput
		default:
		}

		result := e.executeHealthCheck(ctx, healthCtx, step)
		aggregatedStatus = aggregatedStatus.Merge(result.Status)
		aggregatedIssues = append(aggregatedIssues, result.Issues...)

		if result.Output != nil {
			aggregatedOutput = append(aggregatedOutput, result.Output)
		}
	}

	return aggregatedStatus, aggregatedIssues, aggregatedOutput
}

// executeHealthCheck executes a single HealthCheckStep.
func (e *SimpleEngine) executeHealthCheck(
	ctx context.Context,
	healthCtx dirsdk.HealthCheckContext,
	step dirsdk.HealthCheckStep,
) dirsdk.HealthCheckStepResult {
	healthChecker, err := e.registry.GetHealthChecker(step.Kind)
	if err != nil {
		return dirsdk.HealthCheckStepResult{
			Status: kargoapi.HealthStateUnknown,
			Issues: []string{
				fmt.Sprintf("no HealthChecker registered for health check kind %q: %s", step.Kind, err.Error()),
			},
		}
	}
	return healthChecker.CheckHealth(
		e.prepareHealthCheckStepContext(ctx, healthCtx, step),
	)
}

// prepareHealthCheckStepContext prepares a dirsdk.HealthCheckStepContext for a HealthCheckStep.
func (e *SimpleEngine) prepareHealthCheckStepContext(
	ctx context.Context,
	healthCtx dirsdk.HealthCheckContext,
	step dirsdk.HealthCheckStep,
) (context.Context, *dirsdk.HealthCheckStepContext) {
	stepCtx := &dirsdk.HealthCheckStepContext{
		Config:  step.Config.DeepCopy(),
		Project: healthCtx.Project,
		Stage:   healthCtx.Stage,
	}
	ctx = contextWithKargoClient(ctx, e.kargoClient)
	ctx = contextWithArgocdClient(ctx, e.argoCDClient)
	ctx = contextWithCredentialsDB(ctx, e.credentialsDB)
	return ctx, stepCtx
}
