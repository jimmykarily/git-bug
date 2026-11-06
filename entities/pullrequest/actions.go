package pullrequest

import (
	"context"

	"github.com/git-bug/git-bug/entities/identity"
	"github.com/git-bug/git-bug/entity"
	"github.com/git-bug/git-bug/entity/dag"
	"github.com/git-bug/git-bug/repository"
)

// MergeAll will merge all the available remote pull requests
// Note: an author is necessary for the case where a merge commit is created, as this commit will
// have an author and may be signed if a signing key is available.
func MergeAll(ctx context.Context, repo repository.Repo, resolvers entity.Resolvers, remote string, mergeAuthor identity.Interface) <-chan entity.MergeResult {
	return dag.MergeAll(ctx, def, wrapper, repo, resolvers, remote, mergeAuthor)
}

// Remove will remove a local pull request from its entity.Id
func Remove(ctx context.Context, repo repository.Repo, id entity.Id) error {
	return dag.Remove(ctx, def, repo, id)
}

// RemoveAll will remove all local pull requests.
// RemoveAll is idempotent.
func RemoveAll(ctx context.Context, repo repository.Repo) error {
	return dag.RemoveAll(ctx, def, repo)
}

// EnsureClocks makes sure that the clocks ordering pull requests are usable, and
// rebuilds them from the pull requests if not. See dag.EnsureClocks.
func EnsureClocks(ctx context.Context, repo repository.Repo) error {
	return dag.EnsureClocks(ctx, def, repo)
}
