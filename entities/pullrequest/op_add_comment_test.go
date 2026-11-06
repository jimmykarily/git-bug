package pullrequest

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/git-bug/git-bug/entities/identity"
	"github.com/git-bug/git-bug/entity"
	"github.com/git-bug/git-bug/entity/dag/dagtest"
	"github.com/git-bug/git-bug/repository"
)

func TestAddCommentSerialize(t *testing.T) {
	dagtest.SerializeRoundTripTest(t, operationUnmarshaler, func(author identity.Interface, unixTime int64) (*AddCommentOperation, entity.Resolvers) {
		return NewAddCommentOp(author, unixTime, "message"), nil
	})
}

func TestAddCommentValidate(t *testing.T) {
	repo := repository.NewMockRepo()
	rene, err := identity.NewIdentity(repo, "René Descartes", "rene@descartes.fr")
	require.NoError(t, err)

	require.NoError(t, NewAddCommentOp(rene, 1, "message").Validate())
	require.Error(t, NewAddCommentOp(rene, 1, "").Validate())
	require.Error(t, NewAddCommentOp(rene, 1, "  \n").Validate())
}
