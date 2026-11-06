package prcmd

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/require"

	"github.com/git-bug/git-bug/entities/pullrequest"
	"github.com/git-bug/git-bug/repository"
	"github.com/git-bug/git-bug/repository/repotest"
)

// forceColors turns colors on for the test.
func forceColors(t *testing.T) {
	t.Helper()
	old := color.NoColor
	color.NoColor = false
	t.Cleanup(func() { color.NoColor = old })
}

func TestWriteDiffColors(t *testing.T) {
	forceColors(t)
	files := []repository.FileDiff{{
		Path: "a.go",
		Hunks: []repository.DiffHunk{{OldStart: 1, OldLines: 2, NewStart: 1, NewLines: 2, Lines: []repository.DiffLine{
			{Type: repository.DiffLineContext, Content: "one", OldLine: 1, NewLine: 1},
			{Type: repository.DiffLineDeleted, Content: "two", OldLine: 2},
			{Type: repository.DiffLineAdded, Content: "2", NewLine: 2},
		}}},
	}}

	var buf bytes.Buffer
	writeDiff(&buf, files, "", true, nil)
	require.Equal(t, "\x1b[1mdiff --git a/a.go b/a.go\x1b[22m\n"+
		"\x1b[1m--- a/a.go\x1b[22m\n"+
		"\x1b[1m+++ b/a.go\x1b[22m\n"+
		"\x1b[36m@@ -1,2 +1,2 @@\x1b[0m\n"+
		" one\n"+
		"\x1b[31m-two\x1b[0m\n"+
		"\x1b[32m+2\x1b[0m\n", buf.String())

	buf.Reset()
	writeUnifiedDiff(&buf, files, "> ", nil)
	require.NotContains(t, buf.String(), "\x1b")
}

func TestPrListColors(t *testing.T) {
	env, _, _ := newTestEnvWithBranches(t)
	require.NoError(t, runPrNew(env, prNewOptions{nonInteractive: true, title: "first pr", source: "feature", target: "trunk"}))
	env.Out.Reset()
	forceColors(t)

	require.NoError(t, runPr(env))

	require.Regexp(t, "^\x1b\\[36m[0-9a-f]{7}\x1b\\[0m\t\x1b\\[33mopen\x1b\\[0m\tfirst pr\tfeature -> trunk\t\x1b\\[35mJohn Doe\x1b\\[0m\n$", env.Out.String())
}

func TestPrShowColors(t *testing.T) {
	env, trunk, feature := newTestEnvWithBranches(t)
	author, err := env.Backend.GetUserIdentity(t.Context())
	require.NoError(t, err)

	head := commitMain(t, env, "line one\nline 2\n", trunk)
	pr, _, err := env.Backend.PullRequests().NewRaw(author, testTime, "my pr", "", "feature", "trunk", head)
	require.NoError(t, err)
	_, err = pr.AddReviewRaw(author, testTime, pullrequest.ApproveVerdict, "", []pullrequest.LineComment{
		{Commit: head, Base: trunk, Path: "main.go", NewLine: 2, Message: "added"},
		{Commit: head, Base: trunk, Path: "main.go", OldLine: 2, Message: "removed"},
		{Commit: head, Base: trunk, Path: "main.go", NewLine: 1, Message: "unchanged"},
		{Commit: feature, Base: trunk, Path: "main.go", NewLine: 3, Message: "outdated"},
	})
	require.NoError(t, err)
	_, err = pr.AddReviewRaw(author, testTime, pullrequest.RequestChangesVerdict, "", nil)
	require.NoError(t, err)
	require.NoError(t, pr.Commit(t.Context()))
	forceColors(t)

	require.NoError(t, runPrShow(env, []string{pr.Id().String()}))

	out := env.Out.String()
	require.Contains(t, out, "\x1b[1mmy pr\x1b[22m [\x1b[33mopen\x1b[0m]\n")
	require.Contains(t, out, "opened by \x1b[35mJohn Doe\x1b[0m on ")
	require.Contains(t, out, "git bug pr diff \x1b[36m"+pr.Id().Human()+"\x1b[0m\n")
	require.Contains(t, out, "\x1b[35mJohn Doe\x1b[0m \x1b[32mapproved\x1b[0m on ")
	require.Contains(t, out, "\x1b[35mJohn Doe\x1b[0m \x1b[31mrequested changes\x1b[0m on ")
	require.Contains(t, out, "  \x1b[36mmain.go:2 @ "+head.String()[:7]+"\x1b[0m\n  > \x1b[32mline 2\x1b[0m\n")
	require.Contains(t, out, "  \x1b[36mmain.go:-2 @ "+head.String()[:7]+"\x1b[0m\n  > \x1b[31mline two\x1b[0m\n")
	require.Contains(t, out, "  \x1b[36mmain.go:1 @ "+head.String()[:7]+"\x1b[0m\n  > line one\n")
	require.Contains(t, out, "\x1b[2m[outdated]\x1b[22m\n")
}

func TestPrShowColorsNotAvailable(t *testing.T) {
	env, trunk, feature := newTestEnvWithBranches(t)
	author, err := env.Backend.GetUserIdentity(t.Context())
	require.NoError(t, err)
	missing := repository.Hash("1234567890123456789012345678901234567890")
	pr, _, err := env.Backend.PullRequests().NewRaw(author, testTime, "my pr", "", "feature", "trunk", feature)
	require.NoError(t, err)
	_, err = pr.AddReviewRaw(author, testTime, pullrequest.CommentVerdict, "", []pullrequest.LineComment{
		{Commit: missing, Base: trunk, Path: "main.go", NewLine: 1, Message: "lost"},
	})
	require.NoError(t, err)
	require.NoError(t, pr.Commit(t.Context()))
	forceColors(t)

	require.NoError(t, runPrShow(env, []string{pr.Id().String()}))

	require.Contains(t, env.Out.String(), "  > \x1b[2mcode not available (commit 1234567 not in this clone)\x1b[22m\n")
}

func TestPrDiffColors(t *testing.T) {
	env, pr, trunk, head, author := newDiffCommentsPr(t)
	_, err := pr.AddReviewRaw(author, testTime, pullrequest.ApproveVerdict, "", []pullrequest.LineComment{
		{Commit: head, Base: trunk, Path: "main.go", NewLine: 2, Message: "why the change?"},
	})
	require.NoError(t, err)
	require.NoError(t, pr.Commit(t.Context()))
	forceColors(t)

	require.NoError(t, runPrDiff(env, prDiffOptions{}, []string{pr.Id().String()}))

	out := env.Out.String()
	require.Contains(t, out, "\x1b[31m-line two\x1b[0m\n")
	require.Contains(t, out, "\x1b[32m+line 2\x1b[0m\n")
	require.Contains(t, out, "\x1b[33;1m    ┃\x1b[0;22m \x1b[35mJohn Doe\x1b[0m\x1b[2m, ")
	require.Contains(t, out, "\x1b[32mapproved\x1b[0m")
	require.Contains(t, out, "\x1b[33;1m    ┃\x1b[0;22m \x1b[1mwhy the change?\x1b[22m\n")

	env.Out.Reset()
	require.NoError(t, runPrDiff(env, prDiffOptions{noComments: true}, []string{pr.Id().String()}))
	require.Contains(t, env.Out.String(), "\x1b[32m+line 2\x1b[0m\n")
}

func TestNoColorsFlag(t *testing.T) {
	env, _, feature := newTestEnvWithBranches(t)
	author, err := env.Backend.GetUserIdentity(t.Context())
	require.NoError(t, err)
	pr, _, err := env.Backend.PullRequests().NewRaw(author, testTime, "my pr", "", "feature", "trunk", feature)
	require.NoError(t, err)
	require.NoError(t, pr.Commit(t.Context()))
	// The command opens the repository of the working directory and loads
	// its own backend.
	require.NoError(t, env.Backend.Close())
	t.Chdir(filepath.Dir(env.Repo.(*repotest.GoGitRepo).GitDir))

	run := func(args ...string) string {
		forceColors(t)
		env.Out.Reset()
		cmd := NewPullRequestCommand(env)
		cmd.SetArgs(args)
		require.NoError(t, cmd.Execute())
		return env.Out.String()
	}

	require.Contains(t, run("show", pr.Id().String()), "\x1b[")
	require.NotContains(t, run("show", "--no-colors", pr.Id().String()), "\x1b")
	require.NotContains(t, run("--no-colors"), "\x1b")
}

func TestWriteDiffBinaryPlain(t *testing.T) {
	forceColors(t)
	var buf bytes.Buffer
	writeDiff(&buf, []repository.FileDiff{{Path: "a.bin", IsBinary: true}}, "", true, nil)
	require.Equal(t, "\x1b[1mdiff --git a/a.bin b/a.bin\x1b[22m\nBinary files differ\n", buf.String())
}
