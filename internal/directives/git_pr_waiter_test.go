package directives

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	"github.com/akuity/kargo/internal/credentials"
	"github.com/akuity/kargo/internal/gitprovider"
	dirsdk "github.com/akuity/kargo/pkg/directives"
	builtins "github.com/akuity/kargo/pkg/x/directives/builtins"
)

func Test_gitPRWaiter_validate(t *testing.T) {
	testCases := []struct {
		name             string
		config           dirsdk.Config
		expectedProblems []string
	}{
		{
			name:   "repoURL not specified",
			config: dirsdk.Config{},
			expectedProblems: []string{
				"(root): repoURL is required",
			},
		},
		{
			name: "repoURL is empty string",
			config: dirsdk.Config{
				"repoURL": "",
			},
			expectedProblems: []string{
				"repoURL: String length must be greater than or equal to 1",
			},
		},
		{
			name: "prNumber not specified",
			config: dirsdk.Config{
				"repoURL": "https://github.com/example/repo.git",
			},
			expectedProblems: []string{
				"(root): prNumber is required",
			},
		},
		{
			name: "prNumber is less than 1",
			config: dirsdk.Config{
				"prNumber": 0,
			},
			expectedProblems: []string{
				"prNumber: Must be greater than or equal to 1",
			},
		},
		{
			name: "provider is an invalid value",
			config: dirsdk.Config{
				"provider": "bogus",
			},
			expectedProblems: []string{
				"provider: provider must be one of the following:",
			},
		},
		{
			name: "valid without explicit provider",
			config: dirsdk.Config{
				"prNumber": 42,
				"repoURL":  "https://github.com/example/repo.git",
			},
		},
		{
			name: "valid with explicit provider",
			config: dirsdk.Config{
				"provider": "github",
				"prNumber": 42,
				"repoURL":  "https://github.com/example/repo.git",
			},
		},
	}

	waiter := newGitPRWaiter()

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := waiter.validate(testCase.config)
			if len(testCase.expectedProblems) == 0 {
				require.NoError(t, err)
			} else {
				for _, problem := range testCase.expectedProblems {
					require.ErrorContains(t, err, problem)
				}
			}
		})
	}
}

func Test_gitPRWaiter_wait(t *testing.T) {
	testCases := []struct {
		name       string
		provider   gitprovider.Interface
		assertions func(*testing.T, *dirsdk.PromotionStepResult, error)
	}{
		{
			name: "error finding PR",
			provider: &gitprovider.Fake{
				GetPullRequestFn: func(
					context.Context,
					int64,
				) (*gitprovider.PullRequest, error) {
					return nil, errors.New("something went wrong")
				},
			},
			assertions: func(t *testing.T, res *dirsdk.PromotionStepResult, err error) {
				require.ErrorContains(t, err, "error getting pull request")
				require.ErrorContains(t, err, "something went wrong")
				require.Equal(t, kargoapi.PromotionPhaseErrored, res.Status)
			},
		},
		{
			name: "PR is open",
			provider: &gitprovider.Fake{
				GetPullRequestFn: func(
					context.Context,
					int64,
				) (*gitprovider.PullRequest, error) {
					return &gitprovider.PullRequest{
						Open: true,
					}, nil
				},
			},
			assertions: func(t *testing.T, res *dirsdk.PromotionStepResult, err error) {
				require.NoError(t, err)
				require.Equal(t, kargoapi.PromotionPhaseRunning, res.Status)
			},
		},
		{
			name: "PR is closed and not merged",
			provider: &gitprovider.Fake{
				GetPullRequestFn: func(
					context.Context,
					int64,
				) (*gitprovider.PullRequest, error) {
					return &gitprovider.PullRequest{
						Open:   false,
						Merged: false,
					}, nil
				},
			},
			assertions: func(t *testing.T, res *dirsdk.PromotionStepResult, err error) {
				require.ErrorContains(t, err, "closed without being merged")
				require.True(t, isTerminal(err))
				require.Equal(t, kargoapi.PromotionPhaseFailed, res.Status)
			},
		},
		{
			name: "PR is closed and merged",
			provider: &gitprovider.Fake{
				GetPullRequestFn: func(
					context.Context,
					int64,
				) (*gitprovider.PullRequest, error) {
					return &gitprovider.PullRequest{
						Open:   false,
						Merged: true,
					}, nil
				},
			},
			assertions: func(t *testing.T, res *dirsdk.PromotionStepResult, err error) {
				require.NoError(t, err)
				require.Equal(t, kargoapi.PromotionPhaseSucceeded, res.Status)
			},
		},
	}

	waiter := newGitPRWaiter()

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Cannot register multiple providers with the same name, so this takes
			// care of that problem
			testGitProviderName := uuid.NewString()

			gitprovider.Register(
				testGitProviderName,
				gitprovider.Registration{
					NewProvider: func(
						string,
						*gitprovider.Options,
					) (gitprovider.Interface, error) {
						return testCase.provider, nil
					},
				},
			)

			res, err := waiter.wait(
				context.WithValue(context.Background(), credentialsDBContextKey{}, &credentials.FakeDB{}),
				&dirsdk.PromotionStepContext{},
				builtins.GitWaitForPRConfig{
					Provider: ptr.To(builtins.Provider(testGitProviderName)),
				},
			)
			testCase.assertions(t, res, err)
		})
	}
}
