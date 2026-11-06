package pullrequest

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/git-bug/git-bug/entities/identity"
	"github.com/git-bug/git-bug/entity"
	"github.com/git-bug/git-bug/entity/dag/dagtest"
	"github.com/git-bug/git-bug/repository"
)

func TestAddReviewSerialize(t *testing.T) {
	dagtest.SerializeRoundTripTest(t, operationUnmarshaler, func(author identity.Interface, unixTime int64) (*AddReviewOperation, entity.Resolvers) {
		return NewAddReviewOp(author, unixTime, RequestChangesVerdict, "summary", []LineComment{
			{Commit: testHead, Base: testBase, Path: "main.go", OldPath: "old.go", OldLine: 11, NewLine: 12, Message: "why?"},
			{Commit: repository.Hash(strings.Repeat("b", 40)), Base: testBase, Path: "util.go", OldLine: 4, Message: "typo"},
		}), nil
	})
	dagtest.SerializeRoundTripTest(t, operationUnmarshaler, func(author identity.Interface, unixTime int64) (*AddReviewOperation, entity.Resolvers) {
		return NewAddReviewOp(author, unixTime, ApproveVerdict, "", nil), nil
	})
}

func TestAddReviewValidate(t *testing.T) {
	repo := repository.NewMockRepo()
	rene, err := identity.NewIdentity(repo, "René Descartes", "rene@descartes.fr")
	require.NoError(t, err)

	good := LineComment{Commit: testHead, Base: testBase, Path: "main.go", NewLine: 3, Message: "hm"}

	require.NoError(t, NewAddReviewOp(rene, 1, ApproveVerdict, "", nil).Validate())
	require.NoError(t, NewAddReviewOp(rene, 1, CommentVerdict, "", []LineComment{good}).Validate())
	require.NoError(t, NewAddReviewOp(rene, 1, CommentVerdict, "msg", nil).Validate())

	// an empty review
	require.Error(t, NewAddReviewOp(rene, 1, CommentVerdict, "", nil).Validate())
	// an unknown verdict
	require.Error(t, NewAddReviewOp(rene, 1, Verdict(0), "msg", nil).Validate())

	// a line on the removed side only, and one on both sides
	require.NoError(t, LineComment{Commit: testHead, Base: testBase, Path: "main.go", OldLine: 2, Message: "hm"}.Validate())
	require.NoError(t, LineComment{Commit: testHead, Base: testBase, Path: "main.go", OldLine: 2, NewLine: 2, Message: "hm"}.Validate())

	bad := []LineComment{
		{Commit: testHead, Base: testBase, Path: "main.go", Message: "hm"},
		{Commit: testHead, Base: testBase, Path: "main.go", NewLine: -1, Message: "hm"},
		{Commit: testHead, Base: testBase, Path: "main.go", OldLine: -1, NewLine: 3, Message: "hm"},
		{Commit: testHead, Base: testBase, Path: "", NewLine: 3, Message: "hm"},
		{Commit: "abc", Base: testBase, Path: "main.go", NewLine: 3, Message: "hm"},
		{Commit: testHead, Base: "abc", Path: "main.go", NewLine: 3, Message: "hm"},
		{Commit: testHead, Path: "main.go", NewLine: 3, Message: "hm"},
		{Commit: testHead, Base: testBase, Path: "main.go", NewLine: 3, Message: ""},
	}
	for _, c := range bad {
		require.Error(t, c.Validate())
		require.Error(t, NewAddReviewOp(rene, 1, ApproveVerdict, "msg", []LineComment{good, c}).Validate())
	}
}
