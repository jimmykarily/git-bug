package prcmd

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/git-bug/git-bug/repository/repotest"
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
		require.Equal(t, `warning: branch "feature" is not on any remote yet; push it so reviewers can see the code`+"\n", env.Err.String())

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

	t.Run("nothing to review", func(t *testing.T) {
		env, trunk, feature := newTestEnvWithBranches(t)
		repo := env.Repo.(*repotest.GoGitRepo)
		require.NoError(t, repo.SetBranch("trunk-copy", trunk))
		require.NoError(t, repo.SetBranch("merged", feature))
		require.NoError(t, repo.SetBranch("trunk-ahead", commitMain(t, env, "later\n", feature)))

		cases := []struct {
			name, source, target, err string
		}{
			{"same branch", "feature", "feature", `source and target are the same branch "feature"`},
			{"same commit", "trunk-copy", "trunk", `nothing to review: "trunk-copy" has no commits that "trunk" does not have`},
			{"already merged", "merged", "trunk-ahead", `nothing to review: "merged" has no commits that "trunk-ahead" does not have`},
			{"unknown target", "feature", "nope", `branch "nope" not found locally or on any remote`},
		}
		for _, c := range cases {
			err := runPrNew(env, prNewOptions{
				nonInteractive: true,
				title:          "my pr",
				source:         c.source,
				target:         c.target,
			})
			require.EqualError(t, err, c.err, c.name)
		}
		require.Empty(t, env.Backend.PullRequests().AllIds())
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
