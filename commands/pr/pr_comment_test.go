package prcmd

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/git-bug/git-bug/entities/pullrequest"
)

func TestPrComment(t *testing.T) {
	env, _, feature := newTestEnvWithBranches(t)
	pr, _, err := env.Backend.PullRequests().New(t.Context(), "my pr", "msg", "feature", "trunk", feature)
	require.NoError(t, err)
	require.NoError(t, pr.Commit(t.Context()))

	err = runPrComment(env, prCommentOptions{nonInteractive: true, message: "hello there"}, []string{pr.Id().String()})
	require.NoError(t, err)

	snap := pr.Snapshot()
	require.Len(t, snap.Timeline, 1)
	comment, ok := snap.Timeline[0].(*pullrequest.CommentTimelineItem)
	require.True(t, ok)
	require.Equal(t, "hello there", comment.Message)
}
