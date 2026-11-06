package prcmd

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/git-bug/git-bug/repository/repotest"
)

func TestPrHead(t *testing.T) {
	env, trunk, feature := newTestEnvWithBranches(t)
	pr, _, err := env.Backend.PullRequests().New(t.Context(), "my pr", "msg", "feature", "trunk", feature)
	require.NoError(t, err)
	require.NoError(t, pr.Commit(t.Context()))
	args := []string{pr.Id().String()}

	// unchanged
	require.NoError(t, runPrHead(env, prHeadOptions{}, args))
	require.Equal(t, "head unchanged\n", env.Out.String())
	require.Len(t, pr.Snapshot().Timeline, 0)
	env.Out.Reset()

	// moved
	moved := commitMain(t, env, "line one\nline two\nline three\nline four\n", feature)
	require.NoError(t, env.Repo.(*repotest.GoGitRepo).SetBranch("feature", moved))
	require.NoError(t, runPrHead(env, prHeadOptions{}, args))
	require.Equal(t, moved, pr.Snapshot().Head)
	require.Len(t, pr.Snapshot().Timeline, 1)
	require.Equal(t, fmt.Sprintf("head set to %s (was %s)\n", moved.String()[:7], feature.String()[:7]), env.Out.String())
	env.Out.Reset()

	require.NoError(t, runPrHead(env, prHeadOptions{}, args))
	require.Equal(t, "head unchanged\n", env.Out.String())
	require.Len(t, pr.Snapshot().Timeline, 1)

	// explicit commit
	require.NoError(t, runPrHead(env, prHeadOptions{commit: trunk.String()}, args))
	require.Equal(t, trunk, pr.Snapshot().Head)

	// invalid commit
	err = runPrHead(env, prHeadOptions{commit: "abc"}, args)
	require.ErrorContains(t, err, `"abc" is not a full commit ID`)

	// well-formed but absent commit
	err = runPrHead(env, prHeadOptions{commit: "0123456789012345678901234567890123456789"}, args)
	require.ErrorContains(t, err, "not in this clone")
	require.Equal(t, trunk, pr.Snapshot().Head)
}
