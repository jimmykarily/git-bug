package prcmd

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/git-bug/git-bug/entities/pullrequest"
)

func TestPrReview(t *testing.T) {
	env, _, feature := newTestEnvWithBranches(t)
	pr, _, err := env.Backend.PullRequests().New(t.Context(), "my pr", "msg", "feature", "trunk", feature)
	require.NoError(t, err)
	require.NoError(t, pr.Commit(t.Context()))
	args := []string{pr.Id().String()}

	t.Run("approval with line comments", func(t *testing.T) {
		err := runPrReview(env, prReviewOptions{
			approve: true,
			message: "looks good",
			at:      []string{"main.go:1:first note", "main.go:3:use a colon: here"},
		}, args)
		require.NoError(t, err)
		require.Equal(t, "review added to "+pr.Id().Human()+"\n", env.Out.String())

		snap := pr.Snapshot()
		review, ok := snap.Timeline[len(snap.Timeline)-1].(*pullrequest.ReviewTimelineItem)
		require.True(t, ok)
		require.Equal(t, pullrequest.ApproveVerdict, review.Verdict)
		require.Equal(t, "looks good", review.Message)
		require.Equal(t, []pullrequest.LineComment{
			{Commit: feature, Path: "main.go", Line: 1, Message: "first note"},
			{Commit: feature, Path: "main.go", Line: 3, Message: "use a colon: here"},
		}, review.Comments)
	})

	for name, at := range map[string]string{
		"line out of range": "main.go:9:nope",
		"missing file":      "other.go:1:nope",
		"malformed":         "main.go",
		"bad line number":   "main.go:x:nope",
	} {
		t.Run(name, func(t *testing.T) {
			before := len(pr.Snapshot().Timeline)
			err := runPrReview(env, prReviewOptions{at: []string{"main.go:1:fine", at}}, args)
			require.ErrorContains(t, err, `"`+at+`"`)
			require.Len(t, pr.Snapshot().Timeline, before)
		})
	}
}
