package prcmd

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/git-bug/git-bug/cache"
	"github.com/git-bug/git-bug/commands/execenv"
	"github.com/git-bug/git-bug/entities/identity"
	"github.com/git-bug/git-bug/entities/pullrequest"
	"github.com/git-bug/git-bug/repository"
)

func TestPrDiff(t *testing.T) {
	env, trunk, feature := newTestEnvWithBranches(t)
	pr, _, err := env.Backend.PullRequests().New(t.Context(), "my pr", "", "feature", "trunk", feature)
	require.NoError(t, err)
	require.NoError(t, pr.Commit(t.Context()))

	require.NoError(t, runPrDiff(env, prDiffOptions{}, []string{pr.Id().String()}))

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
	pure := "pure.go"
	files := []repository.FileDiff{
		{
			Path:  "new.go",
			IsNew: true,
			Hunks: []repository.DiffHunk{{OldStart: 0, OldLines: 0, NewStart: 1, NewLines: 1, Lines: []repository.DiffLine{
				{Type: repository.DiffLineAdded, Content: "hello", NewLine: 1},
			}}},
		},
		{Path: "empty.go", IsNew: true},
		{Path: "moved.go", OldPath: &pure},
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
	writeUnifiedDiff(&buf, files, "", nil)
	require.Equal(t, `diff --git a/new.go b/new.go
new file mode 100644
--- /dev/null
+++ b/new.go
@@ -0,0 +1,1 @@
+hello
diff --git a/empty.go b/empty.go
new file mode 100644
diff --git a/pure.go b/moved.go
similarity index 100%
rename from pure.go
rename to moved.go
diff --git a/gone.go b/gone.go
deleted file mode 100644
--- a/gone.go
+++ /dev/null
@@ -1,1 +0,0 @@
-bye
diff --git a/old.go b/renamed.go
rename from old.go
rename to renamed.go
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
	writeUnifiedDiff(&buf, files[:1], "> ", nil)
	require.Equal(t, `> diff --git a/new.go b/new.go
> new file mode 100644
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

func TestWriteUnifiedDiffAfterCallback(t *testing.T) {
	files := []repository.FileDiff{{
		Path: "a.go",
		Hunks: []repository.DiffHunk{{OldStart: 1, OldLines: 1, NewStart: 1, NewLines: 2, Lines: []repository.DiffLine{
			{Type: repository.DiffLineContext, Content: "one", OldLine: 1, NewLine: 1},
			{Type: repository.DiffLineAdded, Content: "two", NewLine: 2},
		}}},
	}}

	var buf bytes.Buffer
	writeUnifiedDiff(&buf, files, "", func(f repository.FileDiff, l repository.DiffLine) {
		buf.WriteString("after " + f.Path + " " + l.Content + "\n")
	})
	require.Equal(t, `diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -1,1 +1,2 @@
 one
after a.go one
+two
after a.go two
`, buf.String())
}

// newDiffCommentsPr returns a pull request whose head removes the last line
// of main.go and adds a line of other.go, and the trunk and head commits.
func newDiffCommentsPr(t *testing.T) (env *execenv.Env, pr *cache.PullRequestCache, trunk, head repository.Hash, author identity.Interface) {
	t.Helper()

	env, trunk, _ = newTestEnvWithBranches(t)
	author, err := env.Backend.GetUserIdentity(t.Context())
	require.NoError(t, err)

	head = commitMain(t, env, "line one\nline 2\n", trunk)
	pr, _, err = env.Backend.PullRequests().NewRaw(author, testTime, "my pr", "", "feature", "trunk", head)
	require.NoError(t, err)
	return env, pr, trunk, head, author
}

func TestPrDiffComments(t *testing.T) {
	env, pr, trunk, head, author := newDiffCommentsPr(t)
	_, err := pr.AddReviewRaw(author, testTime, pullrequest.RequestChangesVerdict, "", []pullrequest.LineComment{
		{Commit: head, Base: trunk, Path: "main.go", NewLine: 2, Message: "why the change?"},
		{Commit: head, Base: trunk, Path: "main.go", OldLine: 2, Message: "first paragraph\n\nsecond paragraph"},
	})
	require.NoError(t, err)
	require.NoError(t, pr.Commit(t.Context()))

	require.NoError(t, runPrDiff(env, prDiffOptions{}, []string{pr.Id().String()}))

	header := fmt.Sprintf("    ┃ John Doe, %s (requested changes)\n", time.Unix(testTime, 0).Format("Mon Jan 2 15:04 2006"))
	expected := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,2 +1,2 @@
 line one
-line two
` + header + `    ┃ first paragraph
    ┃
    ┃ second paragraph
+line 2
` + header + `    ┃ why the change?
`
	require.Equal(t, expected, env.Out.String())
}

func TestPrDiffSeveralCommentsOnOneLine(t *testing.T) {
	env, pr, trunk, head, author := newDiffCommentsPr(t)
	_, err := pr.AddReviewRaw(author, testTime, pullrequest.CommentVerdict, "", []pullrequest.LineComment{
		{Commit: head, Base: trunk, Path: "main.go", NewLine: 1, Message: "first"},
	})
	require.NoError(t, err)
	_, err = pr.AddReviewRaw(author, testTime, pullrequest.ApproveVerdict, "", []pullrequest.LineComment{
		{Commit: head, Base: trunk, Path: "main.go", NewLine: 1, Message: "second"},
	})
	require.NoError(t, err)
	require.NoError(t, pr.Commit(t.Context()))

	require.NoError(t, runPrDiff(env, prDiffOptions{}, []string{pr.Id().String()}))

	date := time.Unix(testTime, 0).Format("Mon Jan 2 15:04 2006")
	expected := fmt.Sprintf(`diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,2 +1,2 @@
 line one
    ┃ John Doe, %[1]s (reviewed)
    ┃ first
    ┃ John Doe, %[1]s (approved)
    ┃ second
-line two
+line 2
`, date)
	require.Equal(t, expected, env.Out.String())
}

func TestPrDiffCommentOnAnotherBase(t *testing.T) {
	env, pr, _, head, author := newDiffCommentsPr(t)
	otherBase := commitMain(t, env, "something else\n")
	_, err := pr.AddReviewRaw(author, testTime, pullrequest.CommentVerdict, "", []pullrequest.LineComment{
		{Commit: head, Base: otherBase, Path: "main.go", NewLine: 2, Message: "old base"},
	})
	require.NoError(t, err)
	require.NoError(t, pr.Commit(t.Context()))

	require.NoError(t, runPrDiff(env, prDiffOptions{}, []string{pr.Id().String()}))

	out := env.Out.String()
	require.Contains(t, out, "+line 2\n\nComments not on a line of this diff:\n")
	require.Contains(t, out, "(reviewed)\n  main.go:2 @ "+head.String()[:7]+"\n  > line 2\n  old base\n")
}

func TestDiffCommentOnLineRenamedFile(t *testing.T) {
	old := "old.go"
	f := repository.FileDiff{Path: "new.go", OldPath: &old}
	deleted := repository.DiffLine{Type: repository.DiffLineDeleted, Content: "a", OldLine: 2}
	head, base := repository.Hash("h"), repository.Hash("b")

	c := diffComment{comment: pullrequest.LineComment{Commit: head, Base: base, Path: "new.go", OldPath: "old.go", OldLine: 2}}
	require.True(t, c.onLine(head, base, f, deleted))

	c.comment.OldPath = "other.go"
	require.False(t, c.onLine(head, base, f, deleted))
}

func TestPrDiffCommentsOutsideTheDiff(t *testing.T) {
	env, pr, trunk, head, author := newDiffCommentsPr(t)
	older := commitMain(t, env, "line one\nline two\nline 3\n", trunk)
	_, err := pr.AddReviewRaw(author, testTime, pullrequest.CommentVerdict, "", []pullrequest.LineComment{
		{Commit: older, Base: trunk, Path: "main.go", NewLine: 2, Message: "on an older head"},
		{Commit: head, Base: trunk, Path: "main.go", NewLine: 9, Message: "far away"},
	})
	require.NoError(t, err)
	require.NoError(t, pr.Commit(t.Context()))

	require.NoError(t, runPrDiff(env, prDiffOptions{}, []string{pr.Id().String()}))

	header := fmt.Sprintf("    ┃ John Doe, %s (reviewed)\n", time.Unix(testTime, 0).Format("Mon Jan 2 15:04 2006"))
	expected := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,2 +1,2 @@
 line one
-line two
+line 2

Comments not on a line of this diff:
` + header + fmt.Sprintf("  main.go:2 @ %s [outdated]\n  > line two\n  on an older head\n", older.String()[:7]) +
		header + fmt.Sprintf("  main.go:9 @ %s\n  > code not available (no line 9 in main.go)\n  far away\n", head.String()[:7])
	require.Equal(t, expected, env.Out.String())
}

func TestPrDiffNoComments(t *testing.T) {
	env, pr, trunk, head, author := newDiffCommentsPr(t)
	_, err := pr.AddReviewRaw(author, testTime, pullrequest.CommentVerdict, "", []pullrequest.LineComment{
		{Commit: head, Base: trunk, Path: "main.go", NewLine: 2, Message: "inline"},
		{Commit: head, Base: trunk, Path: "main.go", NewLine: 9, Message: "listed"},
	})
	require.NoError(t, err)
	require.NoError(t, pr.Commit(t.Context()))

	require.NoError(t, runPrDiff(env, prDiffOptions{noComments: true}, []string{pr.Id().String()}))

	require.Equal(t, `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,2 +1,2 @@
 line one
-line two
+line 2
`, env.Out.String())
}
