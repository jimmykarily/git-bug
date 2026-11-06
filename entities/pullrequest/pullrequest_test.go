package pullrequest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/git-bug/git-bug/entities/identity"
	"github.com/git-bug/git-bug/repository"
	"github.com/git-bug/git-bug/repository/repotest"
)

func TestPullRequestCommitLoad(t *testing.T) {
	repo := repotest.NewGoGitRepo(t, false)

	rene, err := identity.NewIdentity(repo, "René Descartes", "rene@descartes.fr")
	require.NoError(t, err)
	require.NoError(t, rene.Commit(t.Context(), repo))

	pr, _, err := Create(rene, time.Now().Unix(), "title", "message", "feature", "trunk", testHead)
	require.NoError(t, err)
	require.NoError(t, pr.Commit(t.Context(), repo))

	read, err := Read(t.Context(), repo, pr.Id())
	require.NoError(t, err)
	require.NoError(t, read.Validate())

	snap := read.Compile()
	require.Equal(t, pr.Id(), snap.Id())
	require.Equal(t, "title", snap.Title)
	require.Equal(t, "message", snap.Message)
	require.Equal(t, "feature", snap.Source)
	require.Equal(t, "trunk", snap.Target)
	require.Equal(t, testHead, snap.Head)
	require.Equal(t, OpenStatus, snap.Status)
	require.Equal(t, rene.Id(), snap.Author.Id())
	require.Empty(t, snap.Timeline)
	require.Len(t, snap.Operations, 1)
}

func TestSnapshotClone(t *testing.T) {
	repo := repository.NewMockRepo()
	rene, err := identity.NewIdentity(repo, "René Descartes", "rene@descartes.fr")
	require.NoError(t, err)

	pr, op, err := Create(rene, time.Now().Unix(), "title", "message", "feature", "trunk", testHead)
	require.NoError(t, err)

	snap := pr.Compile()
	clone := snap.Clone()
	require.Equal(t, snap, clone)

	clone.AppendOperation(op)
	require.Len(t, snap.Operations, 1)
	require.Len(t, clone.Operations, 2)
}
