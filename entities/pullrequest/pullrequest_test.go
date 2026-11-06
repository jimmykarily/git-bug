package pullrequest

import (
	"strings"
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

func TestSnapshotTimeline(t *testing.T) {
	repo := repository.NewMockRepo()
	rene, err := identity.NewIdentity(repo, "René Descartes", "rene@descartes.fr")
	require.NoError(t, err)
	now := time.Now().Unix()

	newHead := repository.Hash(strings.Repeat("b", 40))

	pr, _, err := Create(rene, now, "title", "message", "feature", "trunk", testHead)
	require.NoError(t, err)

	_, err = AddComment(pr, rene, now, "a comment")
	require.NoError(t, err)
	_, err = AddReview(pr, rene, now, RequestChangesVerdict, "please fix", []LineComment{
		{Commit: testHead, Path: "main.go", Line: 4, Message: "here"},
	})
	require.NoError(t, err)
	_, err = SetHead(pr, rene, now, newHead)
	require.NoError(t, err)
	_, err = SetStatus(pr, rene, now, MergedStatus)
	require.NoError(t, err)
	_, err = SetStatus(pr, rene, now, OpenStatus)
	require.NoError(t, err)

	snap := pr.Compile()
	require.Equal(t, newHead, snap.Head)
	require.Equal(t, OpenStatus, snap.Status)
	require.Len(t, snap.Timeline, 5)

	require.IsType(t, &CommentTimelineItem{}, snap.Timeline[0])
	require.IsType(t, &ReviewTimelineItem{}, snap.Timeline[1])
	require.IsType(t, &HeadTimelineItem{}, snap.Timeline[2])
	require.IsType(t, &StatusTimelineItem{}, snap.Timeline[3])
	require.IsType(t, &StatusTimelineItem{}, snap.Timeline[4])

	require.Equal(t, MergedStatus, snap.Timeline[3].(*StatusTimelineItem).Status)
	require.Equal(t, OpenStatus, snap.Timeline[4].(*StatusTimelineItem).Status)

	review := snap.Timeline[1].(*ReviewTimelineItem)
	require.Equal(t, RequestChangesVerdict, review.Verdict)
	require.Len(t, review.Comments, 1)
	require.Equal(t, testHead, review.Comments[0].Commit)

	for _, item := range snap.Timeline {
		require.NotEmpty(t, item.CombinedId())
	}
}
