package prcmd

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/git-bug/git-bug/commands/bug/testenv"
	"github.com/git-bug/git-bug/commands/execenv"
	"github.com/git-bug/git-bug/repository"
	"github.com/git-bug/git-bug/repository/repotest"
)

// newTestEnvWithBranches returns an environment holding a trunk branch with a
// main.go file of two lines, and a feature branch that adds a third line.
// It returns the trunk commit and the feature commit.
func newTestEnvWithBranches(t *testing.T) (env *execenv.Env, trunk, feature repository.Hash) {
	t.Helper()

	env, _ = testenv.NewTestEnvAndUser(t)
	trunk = commitMain(t, env, "line one\nline two\n")
	feature = commitMain(t, env, "line one\nline two\nline three\n", trunk)

	repo := env.Repo.(*repotest.GoGitRepo)
	require.NoError(t, repo.SetBranch("trunk", trunk))
	require.NoError(t, repo.SetBranch("feature", feature))

	return env, trunk, feature
}

// commitMain stores a commit whose tree holds a main.go file with the content.
func commitMain(t *testing.T, env *execenv.Env, content string, parents ...repository.Hash) repository.Hash {
	t.Helper()

	blob, err := env.Repo.StoreData(t.Context(), []byte(content))
	require.NoError(t, err)
	tree, err := env.Repo.StoreTree(t.Context(), []repository.TreeEntry{
		{ObjectType: repository.Blob, Hash: blob, Name: "main.go"},
	})
	require.NoError(t, err)
	commit, err := env.Repo.StoreCommit(t.Context(), tree, parents...)
	require.NoError(t, err)
	return commit
}
