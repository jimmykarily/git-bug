package prcmd

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/git-bug/git-bug/entities/pullrequest"
	"github.com/git-bug/git-bug/repository"
)

const testTime = int64(1700000000)

func testDate() string {
	return time.Unix(testTime, 0).Format("Mon Jan 2 15:04:05 2006 -0700")
}

func TestPrShow(t *testing.T) {
	env, trunk, feature := newTestEnvWithBranches(t)
	author, err := env.Backend.GetUserIdentity(t.Context())
	require.NoError(t, err)

	pr, _, err := env.Backend.PullRequests().NewRaw(author, testTime, "my pr", "my message", "feature", "trunk", feature)
	require.NoError(t, err)
	_, err = pr.AddCommentRaw(author, testTime, "a comment")
	require.NoError(t, err)
	_, err = pr.AddReviewRaw(author, testTime, pullrequest.ApproveVerdict, "looks good", []pullrequest.LineComment{
		{Commit: feature, Base: trunk, Path: "main.go", NewLine: 3, Message: "nice line"},
	})
	require.NoError(t, err)
	require.NoError(t, pr.Commit(t.Context()))

	require.NoError(t, runPrShow(env, []string{pr.Id().String()}))

	d := testDate()
	expected := fmt.Sprintf(`my pr [open]
feature -> trunk, head %[1]s
opened by John Doe on %[2]s

my message

View the changes: git bug pr diff %[3]s

--- John Doe commented on %[2]s
a comment

--- John Doe approved on %[2]s
looks good
  main.go:3 @ %[1]s
  > line three
  nice line
`, feature.String()[:7], d, pr.Id().Human())
	require.Equal(t, expected, env.Out.String())
}

func TestPrShowOutdatedAndMissing(t *testing.T) {
	env, trunk, feature := newTestEnvWithBranches(t)
	author, err := env.Backend.GetUserIdentity(t.Context())
	require.NoError(t, err)

	missing := repository.Hash("1234567890123456789012345678901234567890")

	pr, _, err := env.Backend.PullRequests().NewRaw(author, testTime, "my pr", "my message", "feature", "trunk", feature)
	require.NoError(t, err)
	_, err = pr.AddReviewRaw(author, testTime, pullrequest.RequestChangesVerdict, "", []pullrequest.LineComment{
		{Commit: feature, Base: trunk, Path: "main.go", NewLine: 3, Message: "old comment"},
		{Commit: missing, Base: trunk, Path: "main.go", NewLine: 1, Message: "lost comment"},
		{Commit: feature, Base: trunk, Path: "main.go", NewLine: 9, Message: "beyond the end"},
	})
	require.NoError(t, err)
	newHead := commitMain(t, env, "line one\nline two\nline three\nline four\n", feature)
	_, err = pr.SetHeadRaw(author, testTime, newHead)
	require.NoError(t, err)
	_, err = pr.SetStatusRaw(author, testTime, pullrequest.MergedStatus)
	require.NoError(t, err)
	require.NoError(t, pr.Commit(t.Context()))

	require.NoError(t, runPrShow(env, []string{pr.Id().String()}))

	out := env.Out.String()
	d := testDate()
	require.Contains(t, out, "my pr [merged]\n")
	require.Contains(t, out, fmt.Sprintf("--- John Doe requested changes on %s\n  main.go:3 @ %s [outdated]\n  > line three\n  old comment\n", d, feature.String()[:7]))
	require.Contains(t, out, "  main.go:1 @ 1234567 [outdated]\n  > code not available (commit 1234567 not in this clone)\n  lost comment\n")
	require.Contains(t, out, "  main.go:9 @ "+feature.String()[:7]+" [outdated]\n  > code not available (no line 9 in main.go)\n  beyond the end\n")
	require.Contains(t, out, fmt.Sprintf("--- John Doe set the head to %s on %s\n", newHead.String()[:7], d))
	require.Contains(t, out, fmt.Sprintf("--- John Doe set the status to merged on %s\n", d))
}

func TestPrShowEmptyMessage(t *testing.T) {
	env, _, feature := newTestEnvWithBranches(t)
	author, err := env.Backend.GetUserIdentity(t.Context())
	require.NoError(t, err)

	pr, _, err := env.Backend.PullRequests().NewRaw(author, testTime, "my pr", "", "feature", "trunk", feature)
	require.NoError(t, err)
	require.NoError(t, pr.Commit(t.Context()))

	require.NoError(t, runPrShow(env, []string{pr.Id().String()}))

	expected := fmt.Sprintf(`my pr [open]
feature -> trunk, head %[1]s
opened by John Doe on %[2]s

View the changes: git bug pr diff %[3]s
`, feature.String()[:7], testDate(), pr.Id().Human())
	require.Equal(t, expected, env.Out.String())
}

func TestPrShowRemovedLine(t *testing.T) {
	env, trunk, _ := newTestEnvWithBranches(t)
	author, err := env.Backend.GetUserIdentity(t.Context())
	require.NoError(t, err)

	head := commitMain(t, env, "line one\n", trunk)
	missingBase := repository.Hash("1234567890123456789012345678901234567890")

	pr, _, err := env.Backend.PullRequests().NewRaw(author, testTime, "my pr", "", "feature", "trunk", head)
	require.NoError(t, err)
	_, err = pr.AddReviewRaw(author, testTime, pullrequest.CommentVerdict, "", []pullrequest.LineComment{
		{Commit: head, Base: trunk, Path: "main.go", OldLine: 2, Message: "why drop this?"},
		{Commit: head, Base: trunk, Path: "new.go", OldPath: "main.go", OldLine: 1, Message: "renamed"},
		{Commit: head, Base: missingBase, Path: "main.go", OldLine: 2, Message: "base is gone"},
		{Commit: head, Base: trunk, Path: "main.go", OldLine: 7, Message: "beyond the end"},
	})
	require.NoError(t, err)
	require.NoError(t, pr.Commit(t.Context()))

	require.NoError(t, runPrShow(env, []string{pr.Id().String()}))

	out := env.Out.String()
	h := head.String()[:7]
	require.Contains(t, out, "  main.go:-2 @ "+h+"\n  > line two\n  why drop this?\n")
	require.Contains(t, out, "  new.go:-1 @ "+h+"\n  > line one\n  renamed\n")
	require.Contains(t, out, "  main.go:-2 @ "+h+"\n  > code not available (commit 1234567 not in this clone)\n  base is gone\n")
	require.Contains(t, out, "  main.go:-7 @ "+h+"\n  > code not available (no line 7 in main.go)\n  beyond the end\n")
}

func TestPrShowMultiLineLineComment(t *testing.T) {
	env, trunk, feature := newTestEnvWithBranches(t)
	author, err := env.Backend.GetUserIdentity(t.Context())
	require.NoError(t, err)

	pr, _, err := env.Backend.PullRequests().NewRaw(author, testTime, "my pr", "", "feature", "trunk", feature)
	require.NoError(t, err)
	_, err = pr.AddReviewRaw(author, testTime, pullrequest.CommentVerdict, "", []pullrequest.LineComment{
		{Commit: feature, Base: trunk, Path: "main.go", NewLine: 3, Message: "first paragraph\n\nsecond paragraph"},
	})
	require.NoError(t, err)
	require.NoError(t, pr.Commit(t.Context()))

	require.NoError(t, runPrShow(env, []string{pr.Id().String()}))

	require.Contains(t, env.Out.String(),
		"  > line three\n  first paragraph\n\n  second paragraph\n")
}
