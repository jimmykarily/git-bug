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
		{Commit: feature, Path: "main.go", Line: 3, Message: "nice line"},
	})
	require.NoError(t, err)
	require.NoError(t, pr.Commit(t.Context()))

	require.NoError(t, runPrShow(env, []string{pr.Id().String()}))

	d := testDate()
	expected := fmt.Sprintf(`my pr [open]
feature -> trunk, head %[1]s
opened by John Doe on %[2]s

my message

View the changes: git diff %[3]s...%[4]s

--- John Doe commented on %[2]s
a comment

--- John Doe approved on %[2]s
looks good
  main.go:3 @ %[1]s
  > line three
  nice line
`, feature.String()[:7], d, trunk, feature)
	require.Equal(t, expected, env.Out.String())
}

func TestPrShowOutdatedAndMissing(t *testing.T) {
	env, _, feature := newTestEnvWithBranches(t)
	author, err := env.Backend.GetUserIdentity(t.Context())
	require.NoError(t, err)

	missing := repository.Hash("1234567890123456789012345678901234567890")

	pr, _, err := env.Backend.PullRequests().NewRaw(author, testTime, "my pr", "my message", "feature", "trunk", feature)
	require.NoError(t, err)
	_, err = pr.AddReviewRaw(author, testTime, pullrequest.RequestChangesVerdict, "", []pullrequest.LineComment{
		{Commit: feature, Path: "main.go", Line: 3, Message: "old comment"},
		{Commit: missing, Path: "main.go", Line: 1, Message: "lost comment"},
		{Commit: feature, Path: "main.go", Line: 9, Message: "beyond the end"},
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
	env, trunk, feature := newTestEnvWithBranches(t)
	author, err := env.Backend.GetUserIdentity(t.Context())
	require.NoError(t, err)

	pr, _, err := env.Backend.PullRequests().NewRaw(author, testTime, "my pr", "", "feature", "trunk", feature)
	require.NoError(t, err)
	require.NoError(t, pr.Commit(t.Context()))

	require.NoError(t, runPrShow(env, []string{pr.Id().String()}))

	expected := fmt.Sprintf(`my pr [open]
feature -> trunk, head %[1]s
opened by John Doe on %[2]s

View the changes: git diff %[3]s...%[4]s
`, feature.String()[:7], testDate(), trunk, feature)
	require.Equal(t, expected, env.Out.String())
}
