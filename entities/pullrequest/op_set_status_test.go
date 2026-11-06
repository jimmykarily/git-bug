package pullrequest

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/git-bug/git-bug/entities/identity"
	"github.com/git-bug/git-bug/entity"
	"github.com/git-bug/git-bug/entity/dag/dagtest"
	"github.com/git-bug/git-bug/repository"
)

func TestSetStatusSerialize(t *testing.T) {
	dagtest.SerializeRoundTripTest(t, operationUnmarshaler, func(author identity.Interface, unixTime int64) (*SetStatusOperation, entity.Resolvers) {
		return NewSetStatusOp(author, unixTime, MergedStatus), nil
	})
}

func TestSetStatusValidate(t *testing.T) {
	repo := repository.NewMockRepo()
	rene, err := identity.NewIdentity(repo, "René Descartes", "rene@descartes.fr")
	require.NoError(t, err)

	require.NoError(t, NewSetStatusOp(rene, 1, ClosedStatus).Validate())
	require.Error(t, NewSetStatusOp(rene, 1, Status(0)).Validate())
}
