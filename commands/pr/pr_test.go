package prcmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrList(t *testing.T) {
	env, _, _ := newTestEnvWithBranches(t)

	for _, title := range []string{"first pr", "second pr"} {
		require.NoError(t, runPrNew(env, prNewOptions{
			nonInteractive: true,
			title:          title,
			source:         "feature",
			target:         "trunk",
		}))
	}
	env.Out.Reset()

	require.NoError(t, runPr(env))

	lines := strings.Split(strings.TrimSuffix(env.Out.String(), "\n"), "\n")
	require.Len(t, lines, 2)
	require.Regexp(t, "^[0-9a-f]{7}\topen\tfirst pr\tfeature -> trunk\tJohn Doe$", lines[0])
	require.Regexp(t, "^[0-9a-f]{7}\topen\tsecond pr\tfeature -> trunk\tJohn Doe$", lines[1])
}
