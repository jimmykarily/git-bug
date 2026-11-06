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
			{Commit: testHead, Path: "main.go", Line: 12, Message: "why?"},
			{Commit: repository.Hash(strings.Repeat("b", 40)), Path: "util.go", Line: 1, Message: "typo"},
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

	good := LineComment{Commit: testHead, Path: "main.go", Line: 3, Message: "hm"}

	require.NoError(t, NewAddReviewOp(rene, 1, ApproveVerdict, "", nil).Validate())
	require.NoError(t, NewAddReviewOp(rene, 1, CommentVerdict, "", []LineComment{good}).Validate())
	require.NoError(t, NewAddReviewOp(rene, 1, CommentVerdict, "msg", nil).Validate())

	// an empty review
	require.Error(t, NewAddReviewOp(rene, 1, CommentVerdict, "", nil).Validate())
	// an unknown verdict
	require.Error(t, NewAddReviewOp(rene, 1, Verdict(0), "msg", nil).Validate())

	bad := []LineComment{
		{Commit: testHead, Path: "main.go", Line: 0, Message: "hm"},
		{Commit: testHead, Path: "", Line: 3, Message: "hm"},
		{Commit: "abc", Path: "main.go", Line: 3, Message: "hm"},
		{Commit: testHead, Path: "main.go", Line: 3, Message: ""},
	}
	for _, c := range bad {
		require.Error(t, c.Validate())
		require.Error(t, NewAddReviewOp(rene, 1, ApproveVerdict, "msg", []LineComment{good, c}).Validate())
	}
}
