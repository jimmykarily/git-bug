package cache

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/git-bug/git-bug/entities/identity"
	"github.com/git-bug/git-bug/entities/pullrequest"
	"github.com/git-bug/git-bug/entity"
	"github.com/git-bug/git-bug/repository"
)

// sharedPullRequest is the single loaded copy of a pull request, see sharedEntity.
type sharedPullRequest = sharedEntity[*pullrequest.Snapshot, pullrequest.Operation]

func newSharedPullRequest(p *pullrequest.PullRequest) *sharedPullRequest {
	return &sharedPullRequest{Tracked: p}
}

type RepoCachePullRequest struct {
	*SubCache[*sharedPullRequest, *PullRequestExcerpt, *PullRequestCache]
}

func NewRepoCachePullRequest(repo repository.Repo,
	resolvers func() entity.Resolvers,
	getUserIdentity getUserIdentityFunc) *RepoCachePullRequest {

	makeView := func(shared *sharedPullRequest, onCommit func()) *PullRequestCache {
		return newPullRequestCache(shared, repo, resolvers, getUserIdentity, onCommit)
	}

	makeIndexData := func(p *sharedPullRequest) []string {
		snap := p.Compile()
		res := []string{snap.Title, snap.Message}
		for _, item := range snap.Timeline {
			switch item := item.(type) {
			case *pullrequest.CommentTimelineItem:
				res = append(res, item.Message)
			case *pullrequest.ReviewTimelineItem:
				res = append(res, item.Message)
				for _, c := range item.Comments {
					res = append(res, c.Message)
				}
			}
		}
		return res
	}

	actions := Actions[*sharedPullRequest]{
		ReadWithResolver: func(ctx context.Context, repo repository.Repo, resolvers entity.Resolvers, id entity.Id) (*sharedPullRequest, error) {
			p, err := pullrequest.ReadWithResolver(ctx, repo, resolvers, id)
			if err != nil {
				return nil, err
			}
			return newSharedPullRequest(p), nil
		},
		Refresh: func(ctx context.Context, repo repository.Repo, resolvers entity.Resolvers, shared *sharedPullRequest) (*sharedPullRequest, error) {
			return shared, shared.Repair(ctx, repo, resolvers)
		},
		Remove:       pullrequest.Remove,
		RemoveAll:    pullrequest.RemoveAll,
		MergeAll:     pullrequest.MergeAll,
		EnsureClocks: pullrequest.EnsureClocks,
	}

	sc := NewSubCache[*sharedPullRequest, *PullRequestExcerpt, *PullRequestCache](
		repo, resolvers, getUserIdentity,
		makeView, newPullRequestExcerpt, makeIndexData, actions,
		pullrequest.Typename, pullrequest.Namespace,
		formatVersion, defaultMaxLoadedBugs,
	)

	return &RepoCachePullRequest{SubCache: sc}
}

// New creates a new pull request.
// The creation is staged, like any operation: Commit writes the new pull
// request in the repository.
func (c *RepoCachePullRequest) New(ctx context.Context, title, message, source, target string, head repository.Hash) (*PullRequestCache, *pullrequest.CreateOperation, error) {
	author, err := c.getUserIdentity(ctx)
	if err != nil {
		return nil, nil, err
	}

	return c.NewRaw(author, time.Now().Unix(), title, message, source, target, head)
}

// NewRaw creates a new pull request with the given author and time.
// The creation is staged, like any operation: Commit writes the new pull
// request in the repository. Until then, the pull request is only known to the
// returned view.
func (c *RepoCachePullRequest) NewRaw(author identity.Interface, unixTime int64, title, message, source, target string, head repository.Hash) (*PullRequestCache, *pullrequest.CreateOperation, error) {
	op := pullrequest.NewCreateOp(author, unixTime, title, message, source, target, head)
	if err := op.Validate(); err != nil {
		return nil, nil, err
	}

	cached := c.newView(newSharedPullRequest(pullrequest.New()))
	cached.Append(op)
	return cached, op, nil
}

// ResolveBranch returns the commit a branch points to. The local branch is
// used if it exists, otherwise the branch of the remotes. It fails if the
// branch is nowhere, or if the remotes disagree.
func (c *RepoCachePullRequest) ResolveBranch(ctx context.Context, name string) (repository.Hash, error) {
	hash, err := c.repo.ResolveRef(ctx, "heads", name)
	if err == nil {
		return hash, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return "", err
	}

	remotes, err := c.repo.GetRemotes()
	if err != nil {
		return "", err
	}
	names := make([]string, 0, len(remotes))
	for remote := range remotes {
		names = append(names, remote)
	}
	slices.Sort(names)

	var found repository.Hash
	var foundOn []string
	disagree := false
	for _, remote := range names {
		hash, err := c.repo.ResolveRef(ctx, "remotes/"+remote, name)
		if errors.Is(err, repository.ErrNotFound) {
			continue
		}
		if err != nil {
			return "", err
		}
		if found != "" && found != hash {
			disagree = true
		}
		found = hash
		foundOn = append(foundOn, remote)
	}

	if disagree {
		return "", fmt.Errorf("branch %q points to different commits on remotes %s", name, strings.Join(foundOn, ", "))
	}
	if found == "" {
		return "", fmt.Errorf("branch %q not found locally or on any remote", name)
	}
	return found, nil
}
