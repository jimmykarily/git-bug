package prcmd

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/git-bug/git-bug/entities/pullrequest"
)

func TestPrStatus(t *testing.T) {
	env, _, feature := newTestEnvWithBranches(t)
	pr, _, err := env.Backend.PullRequests().New(t.Context(), "my pr", "msg", "feature", "trunk", feature)
	require.NoError(t, err)
	require.NoError(t, pr.Commit(t.Context()))
	id := pr.Id().String()

	require.NoError(t, runPrStatus(env, []string{id, "merged"}))
	require.Equal(t, pullrequest.MergedStatus, pr.Snapshot().Status)

	require.NoError(t, runPrStatus(env, []string{id, "open"}))
	require.Equal(t, pullrequest.OpenStatus, pr.Snapshot().Status)

	err = runPrStatus(env, []string{id, "nope"})
	require.ErrorContains(t, err, `unknown status "nope"`)
	require.Equal(t, pullrequest.OpenStatus, pr.Snapshot().Status)
}
