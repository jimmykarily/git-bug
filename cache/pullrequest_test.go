package cache

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/git-bug/git-bug/entities/pullrequest"
	"github.com/git-bug/git-bug/repository"
	"github.com/git-bug/git-bug/repository/repotest"
)

func storeTestCommit(t testing.TB, repo repository.Repo, parents ...repository.Hash) repository.Hash {
	t.Helper()

	data, err := repo.StoreData(t.Context(), []byte("content"))
	require.NoError(t, err)
	tree, err := repo.StoreTree(t.Context(), []repository.TreeEntry{
		{ObjectType: repository.Blob, Hash: data, Name: "file"},
	})
	require.NoError(t, err)
	commit, err := repo.StoreCommit(t.Context(), tree, parents...)
	require.NoError(t, err)
	return commit
}

func TestPullRequestCacheReload(t *testing.T) {
	repo := repotest.NewGoGitRepo(t, false)
	head := storeTestCommit(t, repo)
	newHead := storeTestCommit(t, repo, head)

	cache, err := NewRepoCacheNoEvents(repo, nil)
	require.NoError(t, err)

	rene, err := cache.Identities().New(t.Context(), "René Descartes", "rene@descartes.fr")
	require.NoError(t, err)
	require.NoError(t, cache.SetUserIdentity(t.Context(), rene))

	pr, _, err := cache.PullRequests().New(t.Context(), "my pr", "message", "feature", "trunk", head)
	require.NoError(t, err)
	_, err = pr.AddComment(t.Context(), "a comment")
	require.NoError(t, err)
	_, err = pr.AddReview(t.Context(), pullrequest.ApproveVerdict, "looks good", []pullrequest.LineComment{
		{Commit: head, Path: "file", Line: 1, Message: "nice"},
	})
	require.NoError(t, err)
	_, err = pr.SetHead(t.Context(), newHead)
	require.NoError(t, err)
	_, err = pr.SetStatus(t.Context(), pullrequest.MergedStatus)
	require.NoError(t, err)
	require.NoError(t, pr.Commit(t.Context()))
	id := pr.Id()

	require.NoError(t, cache.Close())

	cache, err = NewRepoCacheNoEvents(repo, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cache.Close()) })

	reloaded, err := cache.PullRequests().Resolve(t.Context(), id)
	require.NoError(t, err)
	snap := reloaded.Snapshot()
	require.Equal(t, "my pr", snap.Title)
	require.Equal(t, newHead, snap.Head)
	require.Equal(t, pullrequest.MergedStatus, snap.Status)
	require.Len(t, snap.Timeline, 4)

	excerpt, err := cache.PullRequests().ResolveExcerpt(id)
	require.NoError(t, err)
	require.Equal(t, "my pr", excerpt.Title)
	require.Equal(t, "feature", excerpt.Source)
	require.Equal(t, "trunk", excerpt.Target)
	require.Equal(t, pullrequest.MergedStatus, excerpt.Status)
	require.Equal(t, rene.Id(), excerpt.AuthorId)
}

func TestPullRequestPushPull(t *testing.T) {
	repoA, repoB, _ := repotest.SetupGoGitReposAndRemote(t)
	head := storeTestCommit(t, repoA)

	cacheA := createTestRepoCacheNoEvents(t, repoA)
	cacheB := createTestRepoCacheNoEvents(t, repoB)

	reneA, err := cacheA.Identities().New(t.Context(), "René Descartes", "rene@descartes.fr")
	require.NoError(t, err)
	require.NoError(t, cacheA.SetUserIdentity(t.Context(), reneA))
	isaacB, err := cacheB.Identities().New(t.Context(), "Isaac Newton", "isaac@newton.uk")
	require.NoError(t, err)
	require.NoError(t, cacheB.SetUserIdentity(t.Context(), isaacB))

	// distribute the identity
	_, err = cacheA.Push(t.Context(), "origin")
	require.NoError(t, err)
	require.NoError(t, cacheB.Pull(t.Context(), "origin"))

	// A creates and reviews a pull request
	prA, _, err := cacheA.PullRequests().New(t.Context(), "my pr", "message", "feature", "trunk", head)
	require.NoError(t, err)
	_, err = prA.AddReview(t.Context(), pullrequest.RequestChangesVerdict, "please fix", nil)
	require.NoError(t, err)
	require.NoError(t, prA.Commit(t.Context()))

	_, err = cacheA.Push(t.Context(), "origin")
	require.NoError(t, err)
	require.NoError(t, cacheB.Pull(t.Context(), "origin"))

	require.Len(t, cacheB.PullRequests().AllIds(), 1)
	prB, err := cacheB.PullRequests().Resolve(t.Context(), prA.Id())
	require.NoError(t, err)
	snap := prB.Snapshot()
	require.Equal(t, "my pr", snap.Title)
	require.Len(t, snap.Timeline, 1)
	review, ok := snap.Timeline[0].(*pullrequest.ReviewTimelineItem)
	require.True(t, ok)
	require.Equal(t, "please fix", review.Message)

	// B comments, A pulls it
	_, err = prB.AddComment(t.Context(), "thanks")
	require.NoError(t, err)
	require.NoError(t, prB.Commit(t.Context()))

	_, err = cacheB.Push(t.Context(), "origin")
	require.NoError(t, err)
	require.NoError(t, cacheA.Pull(t.Context(), "origin"))

	require.Len(t, prA.Snapshot().Timeline, 2)
	comment, ok := prA.Snapshot().Timeline[1].(*pullrequest.CommentTimelineItem)
	require.True(t, ok)
	require.Equal(t, "thanks", comment.Message)
}

func TestResolveBranch(t *testing.T) {
	repo := repotest.NewGoGitRepo(t, false)
	repotest.AddRemote(t, repo, "origin", "/nowhere/origin")
	repotest.AddRemote(t, repo, "upstream", "/nowhere/upstream")

	first := storeTestCommit(t, repo)
	second := storeTestCommit(t, repo, first)

	cache := createTestRepoCacheNoEvents(t, repo)
	prs := cache.PullRequests()

	require.NoError(t, repo.SetBranch("local", first))
	got, err := prs.ResolveBranch(t.Context(), "local")
	require.NoError(t, err)
	require.Equal(t, first, got)

	// a remote at a different commit than the local branch is refused
	require.NoError(t, repo.UpdateRef(t.Context(), "remotes/origin", "local", "", second))
	_, err = prs.ResolveBranch(t.Context(), "local")
	require.EqualError(t, err, fmt.Sprintf("branch \"local\" is at %s locally but at %s on origin; push or update it first", first[:7], second[:7]))

	// a remote at the same commit as the local branch is fine
	require.NoError(t, repo.SetBranch("synced", second))
	require.NoError(t, repo.UpdateRef(t.Context(), "remotes/origin", "synced", "", second))
	got, err = prs.ResolveBranch(t.Context(), "synced")
	require.NoError(t, err)
	require.Equal(t, second, got)

	// a branch known only as a remote tracking ref
	require.NoError(t, repo.UpdateRef(t.Context(), "remotes/origin", "remote-only", "", second))
	got, err = prs.ResolveBranch(t.Context(), "remote-only")
	require.NoError(t, err)
	require.Equal(t, second, got)

	// remotes that agree
	require.NoError(t, repo.UpdateRef(t.Context(), "remotes/upstream", "remote-only", "", second))
	got, err = prs.ResolveBranch(t.Context(), "remote-only")
	require.NoError(t, err)
	require.Equal(t, second, got)

	_, err = prs.ResolveBranch(t.Context(), "missing")
	require.EqualError(t, err, `branch "missing" not found locally or on any remote`)

	// remotes that disagree
	require.NoError(t, repo.UpdateRef(t.Context(), "remotes/origin", "split", "", first))
	require.NoError(t, repo.UpdateRef(t.Context(), "remotes/upstream", "split", "", second))
	_, err = prs.ResolveBranch(t.Context(), "split")
	require.EqualError(t, err, `branch "split" points to different commits on remotes origin, upstream`)
}

func TestBranchOnRemote(t *testing.T) {
	repo := repotest.NewGoGitRepo(t, false)
	repotest.AddRemote(t, repo, "origin", "/nowhere/origin")
	first := storeTestCommit(t, repo)

	cache := createTestRepoCacheNoEvents(t, repo)
	prs := cache.PullRequests()

	require.NoError(t, repo.SetBranch("local", first))
	on, err := prs.BranchOnRemote(t.Context(), "local")
	require.NoError(t, err)
	require.False(t, on)

	require.NoError(t, repo.UpdateRef(t.Context(), "remotes/origin", "local", "", first))
	on, err = prs.BranchOnRemote(t.Context(), "local")
	require.NoError(t, err)
	require.True(t, on)
}
