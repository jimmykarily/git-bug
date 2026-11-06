package pullrequest

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/git-bug/git-bug/entities/identity"
	"github.com/git-bug/git-bug/entity"
	"github.com/git-bug/git-bug/entity/dag/dagtest"
	"github.com/git-bug/git-bug/repository"
)

func TestSetHeadSerialize(t *testing.T) {
	dagtest.SerializeRoundTripTest(t, operationUnmarshaler, func(author identity.Interface, unixTime int64) (*SetHeadOperation, entity.Resolvers) {
		return NewSetHeadOp(author, unixTime, testHead), nil
	})
}

func TestSetHeadValidate(t *testing.T) {
	repo := repository.NewMockRepo()
	rene, err := identity.NewIdentity(repo, "René Descartes", "rene@descartes.fr")
	require.NoError(t, err)

	require.NoError(t, NewSetHeadOp(rene, 1, testHead).Validate())
	require.Error(t, NewSetHeadOp(rene, 1, "abc").Validate())
}
