package directives

import (
	"context"

	dirsdk "github.com/akuity/kargo/pkg/directives"
)

// mockPromoter is a mock implementation of the Promoter interface, which can be
// used for testing.
type mockPromoter struct {
	// name is the name of the mockPromoter.
	name string
	// promoteFunc is the function that should be called when RunPromotionStep is
	// called. If set, this function will be called instead of returning
	// promoteResult and promoteErr.
	promoteFunc func(context.Context, *dirsdk.PromotionStepContext) (*dirsdk.PromotionStepResult, error)
	// promoteResult is the result that should be returned RunPromotionStep is
	// called.
	promoteResult *dirsdk.PromotionStepResult
	// promoteErr is the error that should be returned when RunPromotionStep is
	// called.
	promoteErr error
}

// Name implements the Namer interface.
func (m *mockPromoter) Name() string {
	return m.name
}

// Promote implements the Promoter interface.
func (m *mockPromoter) Promote(
	ctx context.Context,
	stepCtx *dirsdk.PromotionStepContext,
) (*dirsdk.PromotionStepResult, error) {
	if m.promoteFunc != nil {
		return m.promoteFunc(ctx, stepCtx)
	}
	return m.promoteResult, m.promoteErr
}
