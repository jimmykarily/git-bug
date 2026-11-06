package prcmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/git-bug/git-bug/repository"
)

func TestPrDiff(t *testing.T) {
	env, trunk, feature := newTestEnvWithBranches(t)
	pr, _, err := env.Backend.PullRequests().New(t.Context(), "my pr", "", "feature", "trunk", feature)
	require.NoError(t, err)
	require.NoError(t, pr.Commit(t.Context()))

	require.NoError(t, runPrDiff(env, []string{pr.Id().String()}))

	expected := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,2 +1,3 @@
 line one
 line two
+line three
`
	require.Equal(t, expected, env.Out.String())

	base, _, err := prDiff(env, pr.Snapshot())
	require.NoError(t, err)
	require.Equal(t, trunk, base)
}

func TestPrDiffNoMergeBase(t *testing.T) {
	env, _, feature := newTestEnvWithBranches(t)
	author, err := env.Backend.GetUserIdentity(t.Context())
	require.NoError(t, err)
	missing := repository.Hash("0123456789012345678901234567890123456789")
	pr, _, err := env.Backend.PullRequests().NewRaw(author, testTime, "my pr", "", "feature", "trunk", feature)
	require.NoError(t, err)
	_, err = pr.SetHeadRaw(author, testTime, missing)
	require.NoError(t, err)
	require.NoError(t, pr.Commit(t.Context()))

	_, _, err = prDiff(env, pr.Snapshot())
	require.ErrorContains(t, err, `no merge base between "trunk" and the head 0123456 in this clone`)
}

func TestWriteUnifiedDiff(t *testing.T) {
	old := "old.go"
	files := []repository.FileDiff{
		{
			Path:  "new.go",
			IsNew: true,
			Hunks: []repository.DiffHunk{{OldStart: 0, OldLines: 0, NewStart: 1, NewLines: 1, Lines: []repository.DiffLine{
				{Type: repository.DiffLineAdded, Content: "hello", NewLine: 1},
			}}},
		},
		{
			Path:     "gone.go",
			IsDelete: true,
			Hunks: []repository.DiffHunk{{OldStart: 1, OldLines: 1, NewStart: 0, NewLines: 0, Lines: []repository.DiffLine{
				{Type: repository.DiffLineDeleted, Content: "bye", OldLine: 1},
			}}},
		},
		{
			Path:    "renamed.go",
			OldPath: &old,
			Hunks: []repository.DiffHunk{{OldStart: 1, OldLines: 2, NewStart: 1, NewLines: 2, Lines: []repository.DiffLine{
				{Type: repository.DiffLineContext, Content: "same", OldLine: 1, NewLine: 1},
				{Type: repository.DiffLineDeleted, Content: "a", OldLine: 2},
				{Type: repository.DiffLineAdded, Content: "b", NewLine: 2},
			}}},
		},
		{Path: "image.png", IsBinary: true},
	}

	var buf bytes.Buffer
	writeUnifiedDiff(&buf, files, "")
	require.Equal(t, `diff --git a/new.go b/new.go
--- /dev/null
+++ b/new.go
@@ -0,0 +1,1 @@
+hello
diff --git a/gone.go b/gone.go
--- a/gone.go
+++ /dev/null
@@ -1,1 +0,0 @@
-bye
diff --git a/old.go b/renamed.go
--- a/old.go
+++ b/renamed.go
@@ -1,2 +1,2 @@
 same
-a
+b
diff --git a/image.png b/image.png
Binary files differ
`, buf.String())

	buf.Reset()
	writeUnifiedDiff(&buf, files[:1], "> ")
	require.Equal(t, `> diff --git a/new.go b/new.go
> --- /dev/null
> +++ b/new.go
> @@ -0,0 +1,1 @@
> +hello
`, buf.String())
}

func TestUnchangedLineRename(t *testing.T) {
	old := "old.go"
	files := []repository.FileDiff{
		{Path: "other.go"},
		{
			Path:    "renamed.go",
			OldPath: &old,
			Hunks: []repository.DiffHunk{{Lines: []repository.DiffLine{
				{Type: repository.DiffLineContext, Content: "same", OldLine: 4, NewLine: 5},
				{Type: repository.DiffLineAdded, Content: "b", NewLine: 6},
			}}},
		},
	}

	oldLine, oldPath := unchangedLine(files, "renamed.go", 5)
	require.Equal(t, 4, oldLine)
	require.NotNil(t, oldPath)
	require.Equal(t, "old.go", *oldPath)

	oldLine, oldPath = unchangedLine(files, "renamed.go", 6)
	require.Equal(t, 0, oldLine)
	require.Nil(t, oldPath)

	oldLine, oldPath = unchangedLine(files, "missing.go", 1)
	require.Equal(t, 0, oldLine)
	require.Nil(t, oldPath)
}
