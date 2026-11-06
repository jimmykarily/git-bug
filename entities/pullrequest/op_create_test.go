package pullrequest

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/git-bug/git-bug/entities/identity"
	"github.com/git-bug/git-bug/entity"
	"github.com/git-bug/git-bug/entity/dag/dagtest"
	"github.com/git-bug/git-bug/repository"
)

var testHead = repository.Hash(strings.Repeat("a", 40))
var testBase = repository.Hash(strings.Repeat("c", 40))

func TestCreateSerialize(t *testing.T) {
	dagtest.SerializeRoundTripTest(t, operationUnmarshaler, func(author identity.Interface, unixTime int64) (*CreateOperation, entity.Resolvers) {
		return NewCreateOp(author, unixTime, "title", "message", "feature", "trunk", testHead), nil
	})
}

func TestCreateValidate(t *testing.T) {
	repo := repository.NewMockRepo()
	rene, err := identity.NewIdentity(repo, "René Descartes", "rene@descartes.fr")
	require.NoError(t, err)
	now := time.Now().Unix()

	require.NoError(t, NewCreateOp(rene, now, "title", "message", "feature", "trunk", testHead).Validate())

	require.Error(t, NewCreateOp(rene, now, "", "message", "feature", "trunk", testHead).Validate())
	require.Error(t, NewCreateOp(rene, now, "title", "message", "", "trunk", testHead).Validate())
	require.Error(t, NewCreateOp(rene, now, "title", "message", "feature", "", testHead).Validate())
	require.Error(t, NewCreateOp(rene, now, "title", "message", "feature", "trunk", "abc").Validate())
}

func TestCreate(t *testing.T) {
	repo := repository.NewMockRepo()
	rene, err := identity.NewIdentity(repo, "René Descartes", "rene@descartes.fr")
	require.NoError(t, err)

	_, _, err = Create(rene, time.Now().Unix(), "", "message", "feature", "trunk", testHead)
	require.Error(t, err)
}
