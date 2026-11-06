package cache

import (
	"context"
	"time"

	"github.com/git-bug/git-bug/entities/identity"
	"github.com/git-bug/git-bug/entities/pullrequest"
	"github.com/git-bug/git-bug/entity"
	"github.com/git-bug/git-bug/repository"
)

var _ pullrequest.Mutable = &PullRequestCache{}

// PullRequestCache is a wrapper around a PullRequest. It provides a higher
// level API than the raw one from PullRequest, keeps an up-to-date Snapshot
// available and deals with concurrency.
//
// It is a view of the pull request loaded in the cache, see CachedEntityBase:
// what it stages is its own until committed.
type PullRequestCache struct {
	CachedEntityBase[*pullrequest.Snapshot, pullrequest.Operation]
}

func newPullRequestCache(shared *sharedPullRequest, repo repository.Repo, resolvers func() entity.Resolvers, getUserIdentity getUserIdentityFunc, onCommit func()) *PullRequestCache {
	return &PullRequestCache{
		CachedEntityBase: CachedEntityBase[*pullrequest.Snapshot, pullrequest.Operation]{
			repo:            repo,
			resolvers:       resolvers,
			onCommit:        onCommit,
			getUserIdentity: getUserIdentity,
			shared:          shared,
		},
	}
}

func (c *PullRequestCache) AddComment(ctx context.Context, message string) (*pullrequest.AddCommentOperation, error) {
	author, err := c.getUserIdentity(ctx)
	if err != nil {
		return nil, err
	}

	return c.AddCommentRaw(author, time.Now().Unix(), message)
}

func (c *PullRequestCache) AddCommentRaw(author identity.Interface, unixTime int64, message string) (*pullrequest.AddCommentOperation, error) {
	return pullrequest.AddComment(c, author, unixTime, message)
}

func (c *PullRequestCache) AddReview(ctx context.Context, verdict pullrequest.Verdict, message string, comments []pullrequest.LineComment) (*pullrequest.AddReviewOperation, error) {
	author, err := c.getUserIdentity(ctx)
	if err != nil {
		return nil, err
	}

	return c.AddReviewRaw(author, time.Now().Unix(), verdict, message, comments)
}

func (c *PullRequestCache) AddReviewRaw(author identity.Interface, unixTime int64, verdict pullrequest.Verdict, message string, comments []pullrequest.LineComment) (*pullrequest.AddReviewOperation, error) {
	return pullrequest.AddReview(c, author, unixTime, verdict, message, comments)
}

func (c *PullRequestCache) SetHead(ctx context.Context, head repository.Hash) (*pullrequest.SetHeadOperation, error) {
	author, err := c.getUserIdentity(ctx)
	if err != nil {
		return nil, err
	}

	return c.SetHeadRaw(author, time.Now().Unix(), head)
}

func (c *PullRequestCache) SetHeadRaw(author identity.Interface, unixTime int64, head repository.Hash) (*pullrequest.SetHeadOperation, error) {
	return pullrequest.SetHead(c, author, unixTime, head)
}

func (c *PullRequestCache) SetStatus(ctx context.Context, status pullrequest.Status) (*pullrequest.SetStatusOperation, error) {
	author, err := c.getUserIdentity(ctx)
	if err != nil {
		return nil, err
	}

	return c.SetStatusRaw(author, time.Now().Unix(), status)
}

func (c *PullRequestCache) SetStatusRaw(author identity.Interface, unixTime int64, status pullrequest.Status) (*pullrequest.SetStatusOperation, error) {
	return pullrequest.SetStatus(c, author, unixTime, status)
}
