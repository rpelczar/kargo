package directives

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/akuity/kargo/internal/credentials"
)

type kargoClientContextKey struct{}

func contextWithKargoClient(
	ctx context.Context,
	kargoClient client.Client,
) context.Context {
	return context.WithValue(ctx, kargoClientContextKey{}, kargoClient)
}

func kargoClientFromContext(ctx context.Context) client.Client {
	c := ctx.Value(kargoClientContextKey{})
	if c == nil {
		return nil
	}
	return c.(client.Client) // nolint: forcetypeassert
}

type argoCDClientContextKey struct{}

func contextWithArgocdClient(
	ctx context.Context,
	argoCDClient client.Client,
) context.Context {
	return context.WithValue(ctx, argoCDClientContextKey{}, argoCDClient)
}

func argoCDClientFromContext(ctx context.Context) client.Client {
	c := ctx.Value(argoCDClientContextKey{})
	if c == nil {
		return nil
	}
	return c.(client.Client) // nolint: forcetypeassert
}

type credentialsDBContextKey struct{}

func contextWithCredentialsDB(
	ctx context.Context,
	credentialsDB credentials.Database,
) context.Context {
	return context.WithValue(ctx, credentialsDBContextKey{}, credentialsDB)
}

func credentialsDBFromContext(ctx context.Context) credentials.Database {
	c := ctx.Value(credentialsDBContextKey{})
	if c == nil {
		return nil
	}
	return c.(credentials.Database) // nolint: forcetypeassert
}
