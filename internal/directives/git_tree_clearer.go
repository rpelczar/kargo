package directives

import (
	"context"
	"fmt"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/xeipuuv/gojsonschema"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	"github.com/akuity/kargo/internal/controller/git"
	dirsdk "github.com/akuity/kargo/pkg/directives"
	builtins "github.com/akuity/kargo/pkg/x/directives/builtins"
)

func init() {
	Register(newGitTreeClearer())
}

// gitTreeClearer is an implementation of the Promoter interface that removes
// the content of a Git working tree.
type gitTreeClearer struct {
	schemaLoader gojsonschema.JSONLoader
}

// newGitTreeClearer returns an initialized gitTreeClearer.
func newGitTreeClearer() *gitTreeClearer {
	r := &gitTreeClearer{}
	r.schemaLoader = getConfigSchemaLoader(r.Name())
	return r
}

// Name implements the Namer interface.
func (g *gitTreeClearer) Name() string {
	return "git-clear"
}

// Promote implements the Promoter interface.
func (g *gitTreeClearer) Promote(
	ctx context.Context,
	stepCtx *dirsdk.PromotionStepContext,
) (*dirsdk.PromotionStepResult, error) {
	if err := g.validate(stepCtx.Config); err != nil {
		return &dirsdk.PromotionStepResult{Status: kargoapi.PromotionPhaseErrored}, err
	}
	cfg, err := ConfigToStruct[builtins.GitClearConfig](stepCtx.Config)
	if err != nil {
		return &dirsdk.PromotionStepResult{Status: kargoapi.PromotionPhaseErrored},
			fmt.Errorf("could not convert config into %s config: %w", g.Name(), err)
	}
	return g.clear(ctx, stepCtx, cfg)
}

// validate validates gitTreeClearer configuration against a JSON schema.
func (g *gitTreeClearer) validate(cfg dirsdk.Config) error {
	return validate(g.schemaLoader, gojsonschema.NewGoLoader(cfg), g.Name())
}

func (g *gitTreeClearer) clear(
	_ context.Context,
	stepCtx *dirsdk.PromotionStepContext,
	cfg builtins.GitClearConfig,
) (*dirsdk.PromotionStepResult, error) {
	p, err := securejoin.SecureJoin(stepCtx.WorkDir, cfg.Path)
	if err != nil {
		return &dirsdk.PromotionStepResult{Status: kargoapi.PromotionPhaseErrored}, fmt.Errorf(
			"error joining path %s with work dir %s: %w",
			cfg.Path, stepCtx.WorkDir, err,
		)
	}
	workTree, err := git.LoadWorkTree(p, nil)
	if err != nil {
		return &dirsdk.PromotionStepResult{Status: kargoapi.PromotionPhaseErrored},
			fmt.Errorf("error loading working tree from %s: %w", cfg.Path, err)
	}
	// workTree.Clear() won't remove any files that aren't indexed. This is a bit
	// of a hack to ensure that we don't have any untracked files in the working
	// tree so that workTree.Clear() will remove everything.
	if err = workTree.AddAll(); err != nil {
		return &dirsdk.PromotionStepResult{Status: kargoapi.PromotionPhaseErrored},
			fmt.Errorf("error adding all files to working tree at %s: %w", cfg.Path, err)
	}
	if err = workTree.Clear(); err != nil {
		return &dirsdk.PromotionStepResult{Status: kargoapi.PromotionPhaseErrored},
			fmt.Errorf("error clearing working tree at %s: %w", cfg.Path, err)
	}
	return &dirsdk.PromotionStepResult{Status: kargoapi.PromotionPhaseSucceeded}, nil
}
