package directives

import (
	"context"
	"fmt"

	"github.com/xeipuuv/gojsonschema"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	"github.com/akuity/kargo/internal/controller/git"
	"github.com/akuity/kargo/internal/credentials"
	"github.com/akuity/kargo/internal/gitprovider"
	dirsdk "github.com/akuity/kargo/pkg/directives"
	builtins "github.com/akuity/kargo/pkg/x/directives/builtins"
)

func init() {
	Register(newGitPRWaiter())
}

// gitPRWaiter is an implementation of the Promoter interface that waits for a
// pull request to be merged or closed unmerged.
type gitPRWaiter struct {
	schemaLoader gojsonschema.JSONLoader
}

// newGitPRWaiter returns an initialized gitPRWaiter.
func newGitPRWaiter() *gitPRWaiter {
	r := &gitPRWaiter{}
	r.schemaLoader = getConfigSchemaLoader(r.Name())
	return r
}

// Name implements the Namer interface.
func (g *gitPRWaiter) Name() string {
	return "git-wait-for-pr"
}

// Promote implements the Promoter interface.
func (g *gitPRWaiter) Promote(
	ctx context.Context,
	stepCtx *dirsdk.PromotionStepContext,
) (*dirsdk.PromotionStepResult, error) {
	if err := g.validate(stepCtx.Config); err != nil {
		return &dirsdk.PromotionStepResult{Status: kargoapi.PromotionPhaseErrored}, err
	}
	cfg, err := ConfigToStruct[builtins.GitWaitForPRConfig](stepCtx.Config)
	if err != nil {
		return &dirsdk.PromotionStepResult{Status: kargoapi.PromotionPhaseErrored},
			fmt.Errorf("could not convert config into git-wait-for-pr config: %w", err)
	}
	return g.wait(ctx, stepCtx, cfg)
}

// validate validates gitPRWaiter configuration against a JSON schema.
func (g *gitPRWaiter) validate(cfg dirsdk.Config) error {
	return validate(g.schemaLoader, gojsonschema.NewGoLoader(cfg), g.Name())
}

func (g *gitPRWaiter) wait(
	ctx context.Context,
	stepCtx *dirsdk.PromotionStepContext,
	cfg builtins.GitWaitForPRConfig,
) (*dirsdk.PromotionStepResult, error) {
	credsDB := credentialsDBFromContext(ctx)
	creds, found, err := credsDB.Get(
		ctx,
		stepCtx.Project,
		credentials.TypeGit,
		cfg.RepoURL,
	)
	if err != nil {
		return &dirsdk.PromotionStepResult{Status: kargoapi.PromotionPhaseErrored},
			fmt.Errorf("error getting credentials for %s: %w", cfg.RepoURL, err)
	}
	var repoCreds *git.RepoCredentials
	if found {
		repoCreds = &git.RepoCredentials{
			Username:      creds.Username,
			Password:      creds.Password,
			SSHPrivateKey: creds.SSHPrivateKey,
		}
	}

	gpOpts := &gitprovider.Options{
		InsecureSkipTLSVerify: cfg.InsecureSkipTLSVerify,
	}
	if repoCreds != nil {
		gpOpts.Token = repoCreds.Password
	}
	if cfg.Provider != nil {
		gpOpts.Name = string(*cfg.Provider)
	}
	gitProv, err := gitprovider.New(cfg.RepoURL, gpOpts)
	if err != nil {
		return &dirsdk.PromotionStepResult{Status: kargoapi.PromotionPhaseErrored},
			fmt.Errorf("error creating git provider service: %w", err)
	}

	pr, err := gitProv.GetPullRequest(ctx, cfg.PRNumber)
	if err != nil {
		return &dirsdk.PromotionStepResult{Status: kargoapi.PromotionPhaseErrored},
			fmt.Errorf("error getting pull request %d: %w", cfg.PRNumber, err)
	}

	if pr.Open {
		return &dirsdk.PromotionStepResult{Status: kargoapi.PromotionPhaseRunning}, nil
	}
	if !pr.Merged {
		return &dirsdk.PromotionStepResult{Status: kargoapi.PromotionPhaseFailed},
			&terminalError{err: fmt.Errorf("pull request %d was closed without being merged", cfg.PRNumber)}
	}
	return &dirsdk.PromotionStepResult{
		Status: kargoapi.PromotionPhaseSucceeded,
		Output: map[string]any{stateKeyCommit: pr.MergeCommitSHA},
	}, nil
}
