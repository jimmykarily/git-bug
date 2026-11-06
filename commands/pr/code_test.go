package prcmd

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/git-bug/git-bug/repository"
)

func TestReadLine(t *testing.T) {
	env, _, feature := newTestEnvWithBranches(t)

	line, err := readLine(t.Context(), env.Repo, feature, "main.go", 3)
	require.NoError(t, err)
	require.Equal(t, "line three", line)

	_, err = readLine(t.Context(), env.Repo, feature, "main.go", 4)
	require.ErrorIs(t, err, errNoSuchLine)

	_, err = readLine(t.Context(), env.Repo, feature, "other.go", 1)
	require.ErrorIs(t, err, errNoSuchLine)

	_, err = readLine(t.Context(), env.Repo, repository.Hash("1234567890123456789012345678901234567890"), "main.go", 1)
	require.ErrorIs(t, err, errCommitMissing)
}
