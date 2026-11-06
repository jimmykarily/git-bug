package prcmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/git-bug/git-bug/entities/pullrequest"
)

func TestPrReview(t *testing.T) {
	env, trunk, feature := newTestEnvWithBranches(t)
	pr, _, err := env.Backend.PullRequests().New(t.Context(), "my pr", "msg", "feature", "trunk", feature)
	require.NoError(t, err)
	require.NoError(t, pr.Commit(t.Context()))
	args := []string{pr.Id().String()}

	t.Run("approval with line comments", func(t *testing.T) {
		err := runPrReview(env, prReviewOptions{
			approve: true,
			message: "looks good",
			at:      []string{"main.go:1:first note", "main.go:3:use a colon: here"},
		}, args)
		require.NoError(t, err)
		require.Equal(t, "review added to "+pr.Id().Human()+"\n", env.Out.String())

		snap := pr.Snapshot()
		review, ok := snap.Timeline[len(snap.Timeline)-1].(*pullrequest.ReviewTimelineItem)
		require.True(t, ok)
		require.Equal(t, pullrequest.ApproveVerdict, review.Verdict)
		require.Equal(t, "looks good", review.Message)
		require.Equal(t, []pullrequest.LineComment{
			{Commit: feature, Base: trunk, Path: "main.go", OldLine: 1, NewLine: 1, Message: "first note"},
			{Commit: feature, Base: trunk, Path: "main.go", NewLine: 3, Message: "use a colon: here"},
		}, review.Comments)
	})

	for name, at := range map[string]string{
		"line out of range": "main.go:9:nope",
		"missing file":      "other.go:1:nope",
		"malformed":         "main.go",
		"bad line number":   "main.go:x:nope",
		"empty path":        ":1:nope",
		"empty message":     "main.go:1:",
	} {
		t.Run(name, func(t *testing.T) {
			before := len(pr.Snapshot().Timeline)
			err := runPrReview(env, prReviewOptions{at: []string{"main.go:1:fine", at}}, args)
			require.ErrorContains(t, err, `"`+at+`"`)
			require.Len(t, pr.Snapshot().Timeline, before)
		})
	}
}

// editorScript returns a GIT_EDITOR that runs the shell commands with the
// edited file as $1.
func editorScript(t *testing.T, commands string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "editor.sh")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"+commands+"\n"), 0o755))
	return path
}

func TestPrReviewEditor(t *testing.T) {
	env, trunk, feature := newTestEnvWithBranches(t)
	pr, _, err := env.Backend.PullRequests().New(t.Context(), "my pr", "msg", "feature", "trunk", feature)
	require.NoError(t, err)
	require.NoError(t, pr.Commit(t.Context()))
	args := []string{pr.Id().String()}

	t.Run("comment under the added line", func(t *testing.T) {
		// The added line is the last line of the template.
		t.Setenv("GIT_EDITOR", editorScript(t, `echo "This should be a constant." >> "$1"`))

		err := runPrReview(env, prReviewOptions{requestChanges: true}, args)
		require.NoError(t, err)
		require.Equal(t, "review added to "+pr.Id().Human()+"\n", env.Out.String())

		snap := pr.Snapshot()
		review, ok := snap.Timeline[len(snap.Timeline)-1].(*pullrequest.ReviewTimelineItem)
		require.True(t, ok)
		require.Equal(t, pullrequest.RequestChangesVerdict, review.Verdict)
		require.Empty(t, review.Message)
		require.Equal(t, []pullrequest.LineComment{
			{Commit: feature, Base: trunk, Path: "main.go", NewLine: 3, Message: "This should be a constant."},
		}, review.Comments)
	})

	for name, tc := range map[string]struct {
		opts     prReviewOptions
		commands string
	}{
		"unchanged template":         {commands: "true"},
		"only the verdict":           {commands: `echo comment > "$1"`},
		"approve changed to comment": {opts: prReviewOptions{approve: true}, commands: `echo comment > "$1"`},
	} {
		t.Run(name, func(t *testing.T) {
			env.Out.Reset()
			t.Setenv("GIT_EDITOR", editorScript(t, tc.commands))
			before := len(pr.Snapshot().Timeline)

			err := runPrReview(env, tc.opts, args)
			require.NoError(t, err)
			require.Equal(t, "Empty review, aborting.\n", env.Out.String())
			require.Len(t, pr.Snapshot().Timeline, before)
		})
	}

	for name, tc := range map[string]struct {
		opts     prReviewOptions
		commands string
		verdict  pullrequest.Verdict
	}{
		"unchanged approve template":         {opts: prReviewOptions{approve: true}, commands: "true", verdict: pullrequest.ApproveVerdict},
		"unchanged request-changes template": {opts: prReviewOptions{requestChanges: true}, commands: "true", verdict: pullrequest.RequestChangesVerdict},
		"whitespace stripped":                {opts: prReviewOptions{approve: true}, commands: `c=$(sed 's/[ \t]*$//' "$1"); printf '%s' "$c" > "$1"`, verdict: pullrequest.ApproveVerdict},
	} {
		t.Run(name, func(t *testing.T) {
			env.Out.Reset()
			t.Setenv("GIT_EDITOR", editorScript(t, tc.commands))
			before := len(pr.Snapshot().Timeline)

			err := runPrReview(env, tc.opts, args)
			require.NoError(t, err)
			require.Equal(t, "review added to "+pr.Id().Human()+"\n", env.Out.String())

			snap := pr.Snapshot()
			require.Len(t, snap.Timeline, before+1)
			review, ok := snap.Timeline[before].(*pullrequest.ReviewTimelineItem)
			require.True(t, ok)
			require.Equal(t, tc.verdict, review.Verdict)
			require.Empty(t, review.Message)
			require.Empty(t, review.Comments)
		})
	}

	t.Run("non-interactive approval", func(t *testing.T) {
		env.Out.Reset()
		t.Setenv("GIT_EDITOR", "false")

		err := runPrReview(env, prReviewOptions{approve: true, nonInteractive: true}, args)
		require.NoError(t, err)
		require.Equal(t, "review added to "+pr.Id().Human()+"\n", env.Out.String())

		snap := pr.Snapshot()
		review, ok := snap.Timeline[len(snap.Timeline)-1].(*pullrequest.ReviewTimelineItem)
		require.True(t, ok)
		require.Equal(t, pullrequest.ApproveVerdict, review.Verdict)
		require.Empty(t, review.Message)
		require.Empty(t, review.Comments)
	})

	t.Run("invalid review", func(t *testing.T) {
		t.Setenv("GIT_EDITOR", editorScript(t, `echo lgtm > "$1"`))
		before := len(pr.Snapshot().Timeline)

		err := runPrReview(env, prReviewOptions{}, args)
		require.ErrorContains(t, err, `unknown verdict "lgtm"`)
		require.Len(t, pr.Snapshot().Timeline, before)
	})
}
