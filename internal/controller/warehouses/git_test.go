package warehouses

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	"github.com/akuity/kargo/internal/controller/git"
	"github.com/akuity/kargo/internal/credentials"
)

func TestSelectCommits(t *testing.T) {
	testCases := []struct {
		name       string
		reconciler *reconciler
		assertions func(t *testing.T, commits []kargoapi.GitCommit, err error)
	}{
		{
			name: "error getting repo credentials",
			reconciler: &reconciler{
				credentialsDB: &credentials.FakeDB{
					GetFn: func(
						context.Context,
						string,
						credentials.Type,
						string,
					) (credentials.Credentials, bool, error) {
						return credentials.Credentials{}, false,
							errors.New("something went wrong")
					},
				},
			},
			assertions: func(t *testing.T, commits []kargoapi.GitCommit, err error) {
				require.ErrorContains(t, err, "error obtaining credentials for git repo")
				require.ErrorContains(t, err, "something went wrong")
				require.Empty(t, commits)
			},
		},

		{
			name: "error getting last commit ID",
			reconciler: &reconciler{
				credentialsDB: &credentials.FakeDB{
					GetFn: func(
						context.Context,
						string,
						credentials.Type,
						string,
					) (credentials.Credentials, bool, error) {
						return credentials.Credentials{}, false, nil
					},
				},
				selectCommitMetaFn: func(
					context.Context,
					kargoapi.GitSubscription,
					*git.RepoCredentials,
					string,
				) (*gitMeta, error) {
					return nil, errors.New("something went wrong")
				},
			},
			assertions: func(t *testing.T, commits []kargoapi.GitCommit, err error) {
				require.ErrorContains(t, err, "error determining latest commit ID of git repo")
				require.ErrorContains(t, err, "something went wrong")
				require.Empty(t, commits)
			},
		},

		{
			name: "success",
			reconciler: &reconciler{
				credentialsDB: &credentials.FakeDB{
					GetFn: func(
						context.Context,
						string,
						credentials.Type,
						string,
					) (credentials.Credentials, bool, error) {
						return credentials.Credentials{}, false, nil
					},
				},
				selectCommitMetaFn: func(
					context.Context,
					kargoapi.GitSubscription,
					*git.RepoCredentials,
					string,
				) (*gitMeta, error) {
					return &gitMeta{Commit: "fake-commit", Message: "message"}, nil
				},
			},
			assertions: func(t *testing.T, commits []kargoapi.GitCommit, err error) {
				require.NoError(t, err)
				require.Len(t, commits, 1)
				require.Equal(
					t,
					kargoapi.GitCommit{
						RepoURL: "fake-url",
						ID:      "fake-commit",
						Message: "message",
					},
					commits[0],
				)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			commits, err := testCase.reconciler.selectCommits(
				context.Background(),
				"fake-namespace",
				[]kargoapi.RepoSubscription{
					{
						Git: &kargoapi.GitSubscription{
							RepoURL: "fake-url",
						},
					},
				},
				&kargoapi.FreightReference{},
			)
			testCase.assertions(t, commits, err)
		})
	}
}

func TestSelectCommitMeta(t *testing.T) {
	testCases := []struct {
		name       string
		sub        kargoapi.GitSubscription
		reconciler *reconciler
		assertions func(*testing.T, *gitMeta, error)
	}{
		{
			name: "error cloning repo",
			sub: kargoapi.GitSubscription{
				RepoURL: "fake-url", // This should force a failure
			},
			reconciler: &reconciler{},
			assertions: func(t *testing.T, _ *gitMeta, err error) {
				require.ErrorContains(t, err, "error cloning git repo")
			},
		},
		{
			name: "success",
			sub: kargoapi.GitSubscription{
				RepoURL: "https://github.com/akuity/kargo.git",
			},
			reconciler: newReconciler(fake.NewClientBuilder().Build(), nil),
			assertions: func(t *testing.T, gm *gitMeta, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, gm.Commit)
				require.NotEmpty(t, gm.Message)
				require.Len(t, strings.Split(gm.Message, "\n"), 1)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			gm, err := testCase.reconciler.selectCommitMeta(
				context.Background(),
				testCase.sub,
				nil,
				"",
			)
			testCase.assertions(t, gm, err)
		})
	}
}

func TestSelectCommitID(t *testing.T) {
	testCases := []struct {
		name       string
		sub        kargoapi.GitSubscription
		reconciler *reconciler
		assertions func(t *testing.T, tag string, commit string, err error)
	}{
		{
			name: "newest from branch; error getting commit ID",
			sub: kargoapi.GitSubscription{
				CommitSelectionStrategy: kargoapi.CommitSelectionStrategyNewestFromBranch,
			},
			reconciler: &reconciler{
				getLastCommitIDFn: func(git.Repo) (string, error) {
					return "", errors.New("something went wrong")
				},
			},
			assertions: func(t *testing.T, _, _ string, err error) {
				require.ErrorContains(t, err, "error determining commit ID at head of branch")
				require.ErrorContains(t, err, "something went wrong")
			},
		},
		{
			name: "newest from branch; success",
			sub: kargoapi.GitSubscription{
				CommitSelectionStrategy: kargoapi.CommitSelectionStrategyNewestFromBranch,
			},
			reconciler: &reconciler{
				getLastCommitIDFn: func(git.Repo) (string, error) {
					return "fake-commit", nil
				},
			},
			assertions: func(t *testing.T, tag, commit string, err error) {
				require.NoError(t, err)
				require.Empty(t, tag)
				require.Equal(t, "fake-commit", commit)
			},
		},
		{
			name: "newest from branch with path filters; error getting diffPaths",
			sub: kargoapi.GitSubscription{
				CommitSelectionStrategy: kargoapi.CommitSelectionStrategyNewestFromBranch,
				IncludePaths:            []string{".*"},
			},
			reconciler: &reconciler{
				getLastCommitIDFn: func(git.Repo) (string, error) {
					return "fake-commit", nil
				},
				getDiffPathsSinceCommitIDFn: func(git.Repo, string) ([]string, error) {
					return nil, errors.New("something went wrong")
				},
			},
			assertions: func(t *testing.T, _, _ string, err error) {
				require.ErrorContains(
					t, err, `error getting diffs since commit "sha" in git repo "":`,
				)
				require.ErrorContains(t, err, "something went wrong")
			},
		},
		{
			name: "newest from branch with path filters; error matching filters; invalid regex",
			sub: kargoapi.GitSubscription{
				CommitSelectionStrategy: kargoapi.CommitSelectionStrategyNewestFromBranch,
				IncludePaths:            []string{regexpPrefix + "["},
			},
			reconciler: &reconciler{
				getLastCommitIDFn: func(git.Repo) (string, error) {
					return "fake-commit", nil
				},
				getDiffPathsSinceCommitIDFn: func(git.Repo, string) ([]string, error) {
					return []string{"some_path_to_a/file"}, nil
				},
			},
			assertions: func(t *testing.T, _, _ string, err error) {
				require.ErrorContains(
					t, err, "error checking includePaths/excludePaths match for commit",
				)
				require.ErrorContains(t, err, "error parsing regexp: missing closing ]")
			},
		},
		{
			name: "newest from branch with path filters; error matching filters; no diff matching",
			sub: kargoapi.GitSubscription{
				CommitSelectionStrategy: kargoapi.CommitSelectionStrategyNewestFromBranch,
				IncludePaths:            []string{regexpPrefix + "^third.*"},
			},
			reconciler: &reconciler{
				getLastCommitIDFn: func(git.Repo) (string, error) {
					return "fake-commit", nil
				},
				getDiffPathsSinceCommitIDFn: func(git.Repo, string) ([]string, error) {
					return []string{"first_path_to_a/file", "second_path_to_a/file"}, nil
				},
			},
			assertions: func(t *testing.T, _, _ string, err error) {
				require.ErrorContains(
					t,
					err,
					`commit "fake-commit" not applicable due to includePaths/excludePaths configuration for repo`,
				)
			},
		},
		{
			name: "error listing tags",
			sub: kargoapi.GitSubscription{
				CommitSelectionStrategy: kargoapi.CommitSelectionStrategyLexical,
			},
			reconciler: &reconciler{
				listTagsFn: func(git.Repo) ([]string, error) {
					return nil, errors.New("something went wrong")
				},
			},
			assertions: func(t *testing.T, _, _ string, err error) {
				require.ErrorContains(t, err, "error listing tags from git repo")
				require.ErrorContains(t, err, "something went wrong")
			},
		},
		{
			name: "error compiling allow regex",
			sub: kargoapi.GitSubscription{
				CommitSelectionStrategy: kargoapi.CommitSelectionStrategyLexical,
				AllowTags:               "[", // This should force a failure
			},
			reconciler: &reconciler{
				listTagsFn: func(git.Repo) ([]string, error) {
					return []string{"abc"}, nil
				},
			},
			assertions: func(t *testing.T, _, _ string, err error) {
				require.ErrorContains(t, err, "error compiling regular expression")
			},
		},
		{
			name: "all tags get filtered out",
			sub: kargoapi.GitSubscription{
				CommitSelectionStrategy: kargoapi.CommitSelectionStrategyLexical,
				IgnoreTags:              []string{"abc"},
			},
			reconciler: &reconciler{
				listTagsFn: func(git.Repo) ([]string, error) {
					return []string{"abc"}, nil
				},
			},
			assertions: func(t *testing.T, _, _ string, err error) {
				require.ErrorContains(t, err, "found no applicable tags in repo")
			},
		},
		{
			name: "unknown selection strategy",
			sub: kargoapi.GitSubscription{
				CommitSelectionStrategy: kargoapi.CommitSelectionStrategy("invalid"),
			},
			reconciler: &reconciler{
				listTagsFn: func(git.Repo) ([]string, error) {
					return []string{"abc"}, nil
				},
			},
			assertions: func(t *testing.T, _, _ string, err error) {
				require.ErrorContains(t, err, "unknown commit selection strategy")
			},
		},
		{
			name: "error checking out tag",
			sub: kargoapi.GitSubscription{
				CommitSelectionStrategy: kargoapi.CommitSelectionStrategyLexical,
			},
			reconciler: &reconciler{
				listTagsFn: func(git.Repo) ([]string, error) {
					return []string{"abc"}, nil
				},
				checkoutTagFn: func(git.Repo, string) error {
					return errors.New("something went wrong")
				},
			},
			assertions: func(t *testing.T, _, _ string, err error) {
				require.ErrorContains(t, err, "error checking out tag")
				require.ErrorContains(t, err, "something went wrong")
			},
		},
		{
			name: "error getting commit ID",
			sub: kargoapi.GitSubscription{
				CommitSelectionStrategy: kargoapi.CommitSelectionStrategyLexical,
			},
			reconciler: &reconciler{
				listTagsFn: func(git.Repo) ([]string, error) {
					return []string{"abc"}, nil
				},
				checkoutTagFn: func(git.Repo, string) error {
					return nil
				},
				getLastCommitIDFn: func(git.Repo) (string, error) {
					return "", errors.New("something went wrong")
				},
			},
			assertions: func(t *testing.T, _, _ string, err error) {
				require.ErrorContains(t, err, "error determining commit ID of tag")
				require.ErrorContains(t, err, "something went wrong")
			},
		},
		{
			name: "lexical success",
			sub: kargoapi.GitSubscription{
				CommitSelectionStrategy: kargoapi.CommitSelectionStrategyLexical,
			},
			reconciler: &reconciler{
				listTagsFn: func(git.Repo) ([]string, error) {
					return []string{"abc", "xyz"}, nil
				},
				checkoutTagFn: func(git.Repo, string) error {
					return nil
				},
				getLastCommitIDFn: func(git.Repo) (string, error) {
					return "fake-commit", nil
				},
			},
			assertions: func(t *testing.T, tag, commit string, err error) {
				require.NoError(t, err)
				require.Equal(t, "xyz", tag)
				require.Equal(t, "fake-commit", commit)
			},
		},
		{
			name: "newest tag success",
			sub: kargoapi.GitSubscription{
				CommitSelectionStrategy: kargoapi.CommitSelectionStrategyNewestTag,
			},
			reconciler: &reconciler{
				listTagsFn: func(git.Repo) ([]string, error) {
					return []string{"abc", "xyz"}, nil
				},
				checkoutTagFn: func(git.Repo, string) error {
					return nil
				},
				getLastCommitIDFn: func(git.Repo) (string, error) {
					return "fake-commit", nil
				},
			},
			assertions: func(t *testing.T, tag, commit string, err error) {
				require.Equal(t, "abc", tag)
				require.NoError(t, err)
				require.Equal(t, "fake-commit", commit)
			},
		},
		{
			name: "semver error selecting tag",
			sub: kargoapi.GitSubscription{
				CommitSelectionStrategy: kargoapi.CommitSelectionStrategySemVer,
				SemverConstraint:        "invalid", // This should force a failure
			},
			reconciler: &reconciler{
				listTagsFn: func(git.Repo) ([]string, error) {
					return []string{"1.0.0", "2.0.0"}, nil
				},
			},
			assertions: func(t *testing.T, _, _ string, err error) {
				require.ErrorContains(t, err, "error parsing semver constraint")
			},
		},
		{
			name: "semver success",
			sub: kargoapi.GitSubscription{
				CommitSelectionStrategy: kargoapi.CommitSelectionStrategySemVer,
			},
			reconciler: &reconciler{
				listTagsFn: func(git.Repo) ([]string, error) {
					return []string{"1.0.0", "2.0.0"}, nil
				},
				checkoutTagFn: func(git.Repo, string) error {
					return nil
				},
				getLastCommitIDFn: func(git.Repo) (string, error) {
					return "fake-commit", nil
				},
			},
			assertions: func(t *testing.T, tag, commit string, err error) {
				require.NoError(t, err)
				require.Equal(t, "2.0.0", tag)
				require.Equal(t, "fake-commit", commit)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			tag, commit, err := testCase.reconciler.selectTagAndCommitID(
				nil,
				testCase.sub,
				"sha",
			)
			testCase.assertions(t, tag, commit, err)
		})
	}
}

func TestAllows(t *testing.T) {
	testCases := []struct {
		name    string
		regex   *regexp.Regexp
		tag     string
		allowed bool
	}{
		{
			name:    "no regex specified",
			tag:     "abc",
			allowed: true,
		},
		{
			name:    "allowed",
			regex:   regexp.MustCompile("[a-z]+"),
			tag:     "abc",
			allowed: true,
		},
		{
			name:    "not allowed",
			regex:   regexp.MustCompile("[a-z]+"),
			tag:     "123",
			allowed: false,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(
				t,
				testCase.allowed,
				allows(testCase.tag, testCase.regex),
			)
		})
	}
}

func TestIgnores(t *testing.T) {
	testCases := []struct {
		name    string
		ignore  []string
		tag     string
		ignored bool
	}{
		{
			name:    "ignored",
			ignore:  []string{"abc"},
			tag:     "abc",
			ignored: true,
		},
		{
			name:    "not ignored",
			ignore:  []string{"abc"},
			tag:     "123",
			ignored: false,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(
				t,
				testCase.ignored,
				ignores(testCase.tag, testCase.ignore),
			)
		})
	}
}

func TestSelectLexicallyLastTag(t *testing.T) {
	testCases := []struct {
		name     string
		tags     []string
		expected string
	}{
		{
			name:     "empty/nil tag list",
			tags:     nil,
			expected: "",
		},
		{
			name:     "non-empty tag list",
			tags:     []string{"abc", "xyz", "foo", "bar"},
			expected: "xyz",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(
				t,
				testCase.expected,
				selectLexicallyLastTag(testCase.tags),
			)
		})
	}
}

func TestSelectSemverTag(t *testing.T) {
	testCases := []struct {
		name       string
		constraint string
		tags       []string
		assertions func(*testing.T, string, error)
	}{
		{
			name:       "error parsing constraint",
			constraint: "invalid",
			tags:       nil,
			assertions: func(t *testing.T, _ string, err error) {
				require.ErrorContains(t, err, "error parsing semver constraint")
			},
		},
		{
			name: "empty/nil tag list",
			tags: nil,
			assertions: func(t *testing.T, tag string, err error) {
				require.NoError(t, err)
				require.Empty(t, tag)
			},
		},
		{
			name: "no semantic tags in tag list",
			tags: []string{"abc", "xyz", "foo", "bar"},
			assertions: func(t *testing.T, tag string, err error) {
				require.NoError(t, err)
				require.Empty(t, tag)
			},
		},
		{
			name:       "no constraint matches",
			constraint: ">=2.0.0",
			tags:       []string{"v1.0.0", "v1.2.3"},
			assertions: func(t *testing.T, tag string, err error) {
				require.NoError(t, err)
				require.Empty(t, tag)
			},
		},
		{
			name: "success with no constraint",
			tags: []string{"v1.0.0", "v1.2.3"},
			assertions: func(t *testing.T, tag string, err error) {
				require.NoError(t, err)
				require.Equal(t, "v1.2.3", tag)
			},
		},
		{
			name:       "success with constraint",
			constraint: "<2.0.0",
			tags:       []string{"v1.0.0", "v2.2.3"},
			assertions: func(t *testing.T, tag string, err error) {
				require.NoError(t, err)
				require.Equal(t, "v1.0.0", tag)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			tag, err := selectSemverTag(testCase.tags, testCase.constraint)
			testCase.assertions(t, tag, err)
		})
	}
}

func TestMatchesPathsFilters(t *testing.T) {
	testCases := []struct {
		name         string
		includePaths []string
		excludePaths []string
		diffs        []string
		assertions   func(*testing.T, bool, error)
	}{
		{
			name:         "success with no includePaths configured",
			excludePaths: []string{regexpPrefix + "nonexistent"},
			diffs:        []string{"path1/values.yaml", "path2/_helpers.tpl"},
			assertions: func(t *testing.T, matchFound bool, err error) {
				require.NoError(t, err)
				require.Equal(t, true, matchFound)
			},
		},
		{
			name:         "success with a matching regexp filters configuration",
			includePaths: []string{regexpPrefix + "values\\.ya?ml$"},
			excludePaths: []string{regexPrefix + "nonexistent"},
			diffs:        []string{"path1/values.yaml", "path2/_helpers.tpl"},
			assertions: func(t *testing.T, matchFound bool, err error) {
				require.NoError(t, err)
				require.Equal(t, true, matchFound)
			},
		},
		{
			name:         "success with unmatching regexp filters configuration",
			includePaths: []string{regexpPrefix + "values\\.ya?ml$"},
			excludePaths: []string{regexPrefix + "nonexistent", regexpPrefix + ".*val.*"},
			diffs:        []string{"path1/values.yaml", "path2/_helpers.tpl"},
			assertions: func(t *testing.T, matchFound bool, err error) {
				require.NoError(t, err)
				require.Equal(t, false, matchFound)
			},
		},
		{
			name:         "success with matching glob filters configuration",
			includePaths: []string{"glob:path2/*.tpl"},
			excludePaths: []string{"nonexistent"},
			diffs:        []string{"path1/values.yaml", "path2/_helpers.tpl"},
			assertions: func(t *testing.T, matchFound bool, err error) {
				require.NoError(t, err)
				require.Equal(t, true, matchFound)
			},
		},
		{
			name:         "success with unmatching glob filters configuration",
			includePaths: []string{"path2/*.tpl"},
			excludePaths: []string{regexPrefix + "nonexistent", "*/?helpers.tpl"},
			diffs:        []string{"path1/values.yaml", "path2/_helpers.tpl"},
			assertions: func(t *testing.T, matchFound bool, err error) {
				require.NoError(t, err)
				require.Equal(t, false, matchFound)
			},
		},
		{
			name:         "success with matching prefix filters configuration",
			includePaths: []string{"path1/"},
			excludePaths: []string{"nonexistent"},
			diffs:        []string{"path1/values.yaml", "path2/_helpers.tpl"},
			assertions: func(t *testing.T, matchFound bool, err error) {
				require.NoError(t, err)
				require.Equal(t, true, matchFound)
			},
		},
		{
			name:         "success with unmatching prefix filters configuration",
			includePaths: []string{"path3/"},
			excludePaths: []string{regexPrefix + "nonexistent", "*/?helpers.tpl"},
			diffs:        []string{"path1/values.yaml", "path2/_helpers.tpl"},
			assertions: func(t *testing.T, matchFound bool, err error) {
				require.NoError(t, err)
				require.Equal(t, false, matchFound)
			},
		},
		{
			name:         "error with invalid regexp in excludePaths configuration",
			includePaths: []string{regexPrefix + "values\\.ya?ml$"},
			excludePaths: []string{regexpPrefix + "nonexistent", regexpPrefix + ".*val.*", regexPrefix + "["},
			diffs:        []string{"path1/values.yaml", "path2/_helpers.tpl"},
			assertions: func(t *testing.T, _ bool, err error) {
				require.ErrorContains(t, err, "error parsing regexp: missing closing ]: `[`")
			},
		},
		{
			name:         "error with invalid glob syntax",
			includePaths: []string{"glob:path2/*.tpl["},
			diffs:        []string{"path1/values.yaml", "path2/_helpers.tpl"},
			assertions: func(t *testing.T, _ bool, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, "syntax error in pattern")
			},
		},
		{
			name:         "success unmatching mix1",
			includePaths: []string{"path3", regexPrefix + "nonexistent", globPrefix + "nonexistent"},
			excludePaths: []string{regexPrefix + "nonexistent", "*/?helpers.tpl", globPrefix + "nonexistent"},
			diffs:        []string{"path1/values.yaml", "path2/_helpers.tpl"},
			assertions: func(t *testing.T, matchFound bool, err error) {
				require.NoError(t, err)
				require.Equal(t, false, matchFound)
			},
		},
		{
			name:         "success unmatching mix2",
			includePaths: []string{"path1", regexPrefix + "[_-]", globPrefix + "nonexistent"},
			excludePaths: []string{
				regexPrefix + "nonexistent",
				"path1",
				"path1",
				globPrefix + "*.tpl",
				globPrefix + "*/*.tpl",
				globPrefix + "*.tpl",
				"path1",
			},
			diffs: []string{"path1/values.yaml", "path2/_helpers.tpl", "path2/ingress.yaml"},
			assertions: func(t *testing.T, matchFound bool, err error) {
				require.NoError(t, err)
				require.Equal(t, false, matchFound)
			},
		},
		{
			name: "success unmatching mix3",
			includePaths: []string{
				"path1/f",
				regexpPrefix + "path[1-3]",
				globPrefix + "file*",
			},
			excludePaths: []string{
				regexPrefix + "\\d",
				"yaml",
				globPrefix + "*.tpl",
				globPrefix + "*.tpl",
				"nonexistent",
			},
			diffs: []string{"path1/file1", "path2/file2", "path3/file3"},
			assertions: func(t *testing.T, matchFound bool, err error) {
				require.NoError(t, err)
				require.Equal(t, false, matchFound)
			},
		},
		{
			name:         "success unmatching mix2; no config",
			includePaths: []string{},
			excludePaths: []string{},
			diffs:        []string{},
			assertions: func(t *testing.T, matchFound bool, err error) {
				require.NoError(t, err)
				require.Equal(t, false, matchFound)
			},
		},
		{
			name: "success matching mix1",
			includePaths: []string{
				"path1",
				regexPrefix + "[_-]",
				globPrefix + "nonexistent",
				regexPrefix + "no",
				globPrefix + "*/*/*/abe/*",
			},
			excludePaths: []string{
				regexPrefix + "nonexistent",
				"path1",
				"path1",
				globPrefix + "*.tpl",
				globPrefix + "*/*.tpl",
				globPrefix + "*.tpl",
				regexpPrefix + ".*q",
			},
			diffs: []string{
				"path1/values.yaml",
				"path2/_helpers.tpl",
				"abc",
				"abb",
				"aba",
				"abz",
				"aby",
				"abx",
				"abw",
				"abv",
				"abu",
				"path3/abc/abd/abe/deployment.yaml",
				"path4.txt",
			},
			assertions: func(t *testing.T, matchFound bool, err error) {
				require.NoError(t, err)
				require.Equal(t, true, matchFound)
			},
		},
		{
			name: "success matching mix3; no includePaths",
			excludePaths: []string{
				regexpPrefix + "ab[cbazxwvu]",
				"helpers.tpl",
				globPrefix + "path*/*"},
			diffs: []string{
				"path1/values.yaml",
				"path2/_helpers.tpl",
				"abc",
				"abb",
				"aba",
				"abz",
				"aby",
				"abx",
				"abw",
				"abv",
				"abu",
			},
			assertions: func(t *testing.T, matchFound bool, err error) {
				require.NoError(t, err)
				require.Equal(t, true, matchFound)
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			matchFound, err := matchesPathsFilters(testCase.includePaths, testCase.excludePaths, testCase.diffs)
			testCase.assertions(t, matchFound, err)
		})
	}
}
