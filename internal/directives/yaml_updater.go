package directives

import (
	"context"
	"fmt"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/xeipuuv/gojsonschema"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	intyaml "github.com/akuity/kargo/internal/yaml"
	dirsdk "github.com/akuity/kargo/pkg/directives"
	builtins "github.com/akuity/kargo/pkg/x/directives/builtins"
)

func init() {
	Register(newYAMLUpdater())
}

// yamlUpdater is an implementation of the Promoter interface that updates the
// values of specified keys in a YAML file.
type yamlUpdater struct {
	schemaLoader gojsonschema.JSONLoader
}

// newYAMLUpdater returns an initialized yamlUpdater.
func newYAMLUpdater() *yamlUpdater {
	r := &yamlUpdater{}
	r.schemaLoader = getConfigSchemaLoader(r.Name())
	return r
}

// Name implements the Namer interface.
func (y *yamlUpdater) Name() string {
	return "yaml-update"
}

// Promote implements the Promoter interface.
func (y *yamlUpdater) Promote(
	ctx context.Context,
	stepCtx *dirsdk.PromotionStepContext,
) (*dirsdk.PromotionStepResult, error) {
	failure := &dirsdk.PromotionStepResult{Status: kargoapi.PromotionPhaseErrored}

	if err := y.validate(stepCtx.Config); err != nil {
		return failure, err
	}

	// Convert the configuration into a typed struct
	cfg, err := ConfigToStruct[builtins.YAMLUpdateConfig](stepCtx.Config)
	if err != nil {
		return failure, fmt.Errorf("could not convert config into %s config: %w", y.Name(), err)
	}

	return y.update(ctx, stepCtx, cfg)
}

// validate validates yamlImageUpdater configuration against a JSON schema.
func (y *yamlUpdater) validate(cfg dirsdk.Config) error {
	return validate(y.schemaLoader, gojsonschema.NewGoLoader(cfg), y.Name())
}

func (y *yamlUpdater) update(
	_ context.Context,
	stepCtx *dirsdk.PromotionStepContext,
	cfg builtins.YAMLUpdateConfig,
) (*dirsdk.PromotionStepResult, error) {
	updates := make([]intyaml.Update, len(cfg.Updates))
	for i, update := range cfg.Updates {
		updates[i] = intyaml.Update{
			Key:   update.Key,
			Value: update.Value,
		}
	}

	result := &dirsdk.PromotionStepResult{Status: kargoapi.PromotionPhaseSucceeded}
	if len(updates) > 0 {
		if err := y.updateFile(stepCtx.WorkDir, cfg.Path, updates); err != nil {
			return &dirsdk.PromotionStepResult{Status: kargoapi.PromotionPhaseErrored},
				fmt.Errorf("values file update failed: %w", err)
		}

		if commitMsg := y.generateCommitMessage(cfg.Path, cfg.Updates); commitMsg != "" {
			result.Output = map[string]any{
				"commitMessage": commitMsg,
			}
		}
	}
	return result, nil
}

func (y *yamlUpdater) updateFile(workDir string, path string, updates []intyaml.Update) error {
	absValuesFile, err := securejoin.SecureJoin(workDir, path)
	if err != nil {
		return fmt.Errorf("error joining path %q: %w", path, err)
	}
	if err := intyaml.SetStringsInFile(absValuesFile, updates); err != nil {
		return fmt.Errorf("error updating image references in values file %q: %w", path, err)
	}
	return nil
}

func (y *yamlUpdater) generateCommitMessage(path string, updates []builtins.YAMLUpdate) string {
	if len(updates) == 0 {
		return ""
	}

	var commitMsg strings.Builder
	_, _ = commitMsg.WriteString(fmt.Sprintf("Updated %s\n", path))
	for _, update := range updates {
		_, _ = commitMsg.WriteString(fmt.Sprintf("\n- %s: %q", update.Key, update.Value))
	}

	return commitMsg.String()
}
