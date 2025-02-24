package helpers

import (
	"context"
	"crypto/sha1" // nolint: gosec
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	"github.com/akuity/kargo/internal/git"
	"github.com/akuity/kargo/internal/helm"
)

// GenerateID deterministically calculates a piece of Freight's ID based on its
// contents and returns it.
func GenerateID(f *kargoapi.Freight) string {
	size := len(f.Commits) + len(f.Images) + len(f.Charts)
	artifacts := make([]string, 0, size)
	for _, commit := range f.Commits {
		if commit.Tag != "" {
			// If we have a tag, incorporate it into the canonical representation of a
			// commit used when calculating Freight ID. This is necessary because one
			// commit could have multiple tags. Suppose we have already detected a
			// commit with a tag v1.0.0-rc.1 and produced the corresponding Freight.
			// Later, that same commit is tagged as v1.0.0. If we don't incorporate
			// the tag into the ID, we will never produce a new/distinct piece of
			// Freight for the new tag.
			artifacts = append(
				artifacts,
				fmt.Sprintf("%s:%s:%s", git.NormalizeURL(commit.RepoURL), commit.Tag, commit.ID),
			)
		} else {
			artifacts = append(
				artifacts,
				fmt.Sprintf("%s:%s", git.NormalizeURL(commit.RepoURL), commit.ID),
			)
		}
	}
	for _, image := range f.Images {
		artifacts = append(
			artifacts,
			// Note: This isn't the usual image representation using EITHER :<tag> OR @<digest>.
			// It is possible to have found an image with a tag that is already known, but with a
			// new digest -- as in the case of "mutable" tags like "latest". It is equally possible to
			// have found an image with a digest that is already known, but has been re-tagged.
			// To cover both cases, we incorporate BOTH tag and digest into the canonical
			// representation of an image used when calculating Freight ID.
			fmt.Sprintf("%s:%s@%s", image.RepoURL, image.Tag, image.Digest),
		)
	}
	for _, chart := range f.Charts {
		artifacts = append(
			artifacts,
			fmt.Sprintf(
				"%s:%s",
				// path.Join accounts for the possibility that chart.Name is empty
				path.Join(helm.NormalizeChartRepositoryURL(chart.RepoURL), chart.Name),
				chart.Version,
			),
		)
	}
	slices.Sort(artifacts)
	return fmt.Sprintf(
		"%x",
		sha1.Sum([]byte( // nolint: gosec
			fmt.Sprintf("%s:%s", f.Origin.String(), strings.Join(artifacts, "|")),
		)),
	)
}

// GetFreightByNameOrAlias returns a pointer to the Freight resource specified
// by the project, and name OR alias arguments. If no such resource is found,
// nil is returned instead.
func GetFreightByNameOrAlias(
	ctx context.Context,
	c client.Client,
	project string,
	name string,
	alias string,
) (*kargoapi.Freight, error) {
	if name != "" {
		return GetFreight(
			ctx,
			c,
			types.NamespacedName{
				Namespace: project,
				Name:      name,
			},
		)
	}
	return GetFreightByAlias(ctx, c, project, alias)
}

// GetFreight returns a pointer to the Freight resource specified by the
// namespacedName argument. If no such resource is found, nil is returned
// instead.
func GetFreight(
	ctx context.Context,
	c client.Client,
	namespacedName types.NamespacedName,
) (*kargoapi.Freight, error) {
	freight := kargoapi.Freight{}
	if err := c.Get(ctx, namespacedName, &freight); err != nil {
		if err = client.IgnoreNotFound(err); err == nil {
			return nil, nil
		}
		return nil, fmt.Errorf(
			"error getting Freight %q in namespace %q: %w",
			namespacedName.Name,
			namespacedName.Namespace,
			err,
		)
	}
	return &freight, nil
}

// GetFreightByAlias returns a pointer to the Freight resource specified by the
// project and alias arguments. If no such resource is found, nil is returned
// instead.
func GetFreightByAlias(
	ctx context.Context,
	c client.Client,
	project string,
	alias string,
) (*kargoapi.Freight, error) {
	freightList := kargoapi.FreightList{}
	if err := c.List(
		ctx,
		&freightList,
		client.InNamespace(project),
		client.MatchingLabels{
			kargoapi.AliasLabelKey: alias,
		},
	); err != nil {
		return nil, fmt.Errorf(
			"error listing Freight with alias %q in namespace %q: %w",
			alias,
			project,
			err,
		)
	}
	if len(freightList.Items) == 0 {
		return nil, nil
	}
	return &freightList.Items[0], nil
}

// ListFreightByCurrentStage returns a list of Freight resources that think
// they're currently in use by the Stage specified.
func ListFreightByCurrentStage(
	ctx context.Context,
	c client.Client,
	stage *kargoapi.Stage,
) ([]kargoapi.Freight, error) {
	freightList := kargoapi.FreightList{}
	if err := c.List(
		ctx,
		&freightList,
		client.InNamespace(stage.Namespace),
		client.MatchingFields{"currentlyIn": stage.Name},
	); err != nil {
		return nil, fmt.Errorf(
			"error listing Freight in namespace %q with current stage %q: %w",
			stage.Namespace,
			stage.Name,
			err,
		)
	}
	return freightList.Items, nil
}

// IsCurrentlyIn returns whether the Freight is currently in the specified
// Stage.
func IsCurrentlyIn(f *kargoapi.Freight, stage string) bool {
	// NB: This method exists for convenience. It doesn't require the caller to
	// know anything about the Freight status' internal data structure.
	_, in := f.Status.CurrentlyIn[stage]
	return in
}

// IsVerifiedIn returns whether the Freight has been verified in the specified
// Stage.
func IsVerifiedIn(f *kargoapi.Freight, stage string) bool {
	// NB: This method exists for convenience. It doesn't require the caller to
	// know anything about the Freight status' internal data structure.
	_, verified := f.Status.VerifiedIn[stage]
	return verified
}

// IsApprovedFor returns whether the Freight has been approved for the specified
// Stage.
func IsApprovedFor(f *kargoapi.Freight, stage string) bool {
	// NB: This method exists for convenience. It doesn't require the caller to
	// know anything about the Freight status' internal data structure.
	_, approved := f.Status.ApprovedFor[stage]
	return approved
}

// GetLongestSoak returns the longest soak time for the Freight in the specified
// Stage if it's been verified in that Stage. If it has not, zero will be
// returned instead. If the Freight is currently in use by the specified Stage,
// the current soak time is calculated and compared to the longest completed
// soak time on record.
func GetLongestSoak(f *kargoapi.Freight, stage string) time.Duration {
	if _, verified := f.Status.VerifiedIn[stage]; !verified {
		return 0
	}
	var longestCompleted time.Duration
	if record, isVerified := f.Status.VerifiedIn[stage]; isVerified && record.LongestCompletedSoak != nil {
		longestCompleted = record.LongestCompletedSoak.Duration
	}
	var current time.Duration
	if record, isCurrent := f.Status.CurrentlyIn[stage]; isCurrent {
		current = time.Since(record.Since.Time)
	}
	return time.Duration(max(longestCompleted.Nanoseconds(), current.Nanoseconds()))
}

// AddCurrentStage updates the Freight status to reflect that the Freight is
// currently in the specified Stage.
func AddCurrentStage(f *kargoapi.FreightStatus, stage string, since time.Time) {
	if _, alreadyIn := f.CurrentlyIn[stage]; !alreadyIn {
		if f.CurrentlyIn == nil {
			f.CurrentlyIn = make(map[string]kargoapi.CurrentStage)
		}
		f.CurrentlyIn[stage] = kargoapi.CurrentStage{
			Since: &metav1.Time{Time: since},
		}
	}
}

// RemoveCurrentStage updates the Freight status to reflect that the Freight is
// no longer in the specified Stage. If the Freight was verified in the
// specified Stage, the longest completed soak time will be updated if
// necessary.
func RemoveCurrentStage(f *kargoapi.FreightStatus, stage string) {
	if record, in := f.CurrentlyIn[stage]; in {
		if _, verified := f.VerifiedIn[stage]; verified {
			soak := time.Since(record.Since.Time)
			if soak > f.VerifiedIn[stage].LongestCompletedSoak.Duration {
				f.VerifiedIn[stage] = kargoapi.VerifiedStage{
					LongestCompletedSoak: &metav1.Duration{Duration: soak},
				}
			}
		}
		delete(f.CurrentlyIn, stage)
	}
}

// AddVerifiedStage updates the Freight status to reflect that the Freight has
// been verified in the specified Stage.
func AddVerifiedStage(f *kargoapi.FreightStatus, stage string, verifiedAt time.Time) {
	if _, verified := f.VerifiedIn[stage]; !verified {
		record := kargoapi.VerifiedStage{VerifiedAt: &metav1.Time{Time: verifiedAt}}
		if f.VerifiedIn == nil {
			f.VerifiedIn = map[string]kargoapi.VerifiedStage{stage: record}
		}
		f.VerifiedIn[stage] = record
	}
}

// AddApprovedStage updates the Freight status to reflect that the Freight has
// been approved for the specified Stage.
func AddApprovedStage(f *kargoapi.FreightStatus, stage string, approvedAt time.Time) {
	if _, approved := f.ApprovedFor[stage]; !approved {
		record := kargoapi.ApprovedStage{ApprovedAt: &metav1.Time{Time: approvedAt}}
		if f.ApprovedFor == nil {
			f.ApprovedFor = map[string]kargoapi.ApprovedStage{stage: record}
		}
		f.ApprovedFor[stage] = record
	}
}
