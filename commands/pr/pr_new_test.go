package prcmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrNew(t *testing.T) {
	t.Run("source and target", func(t *testing.T) {
		env, _, feature := newTestEnvWithBranches(t)

		err := runPrNew(env, prNewOptions{
			nonInteractive: true,
			title:          "my pr",
			message:        "my message",
			source:         "feature",
			target:         "trunk",
		})
		require.NoError(t, err)
		require.Regexp(t, "^[0-9a-f]{7} created\n$", env.Out.String())

		ids := env.Backend.PullRequests().AllIds()
		require.Len(t, ids, 1)
		pr, err := env.Backend.PullRequests().Resolve(t.Context(), ids[0])
		require.NoError(t, err)
		snap := pr.Snapshot()
		require.Equal(t, "my pr", snap.Title)
		require.Equal(t, "my message", snap.Message)
		require.Equal(t, "feature", snap.Source)
		require.Equal(t, "trunk", snap.Target)
		require.Equal(t, feature, snap.Head)
	})

	t.Run("unknown source", func(t *testing.T) {
		env, _, _ := newTestEnvWithBranches(t)

		err := runPrNew(env, prNewOptions{
			nonInteractive: true,
			title:          "my pr",
			source:         "nope",
			target:         "trunk",
		})
		require.ErrorContains(t, err, `branch "nope" not found locally or on any remote`)
		require.Empty(t, env.Backend.PullRequests().AllIds())
	})
}
