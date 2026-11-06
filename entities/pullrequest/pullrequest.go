// Package pullrequest contains the pull request data model and low-level related functions
package pullrequest

import (
	"context"
	"fmt"

	"github.com/git-bug/git-bug/entities/identity"
	"github.com/git-bug/git-bug/entity"
	"github.com/git-bug/git-bug/entity/dag"
	"github.com/git-bug/git-bug/repository"
)

var _ dag.Mutable[*Snapshot, Operation] = &PullRequest{}
var _ dag.Tracked[*Snapshot, Operation] = &PullRequest{}
var _ entity.Interface = &PullRequest{}

// 1: original format
const formatVersion = 1

const Typename = "pullrequest"
const Namespace = "pullrequests"

var def = dag.Definition{
	Typename:             Typename,
	Namespace:            Namespace,
	OperationUnmarshaler: operationUnmarshaler,
	FormatVersion:        formatVersion,
}

// Mutable is what the functions creating operations need from a pull request,
// see dag.Mutable.
type Mutable = dag.Mutable[*Snapshot, Operation]

// PullRequest holds the data of a pull request, organized in a way close to
// how it will be persisted inside Git.
type PullRequest struct {
	*dag.Entity
}

// New create a new PullRequest
func New() *PullRequest {
	return wrapper(dag.New(def))
}

func wrapper(e *dag.Entity) *PullRequest {
	return &PullRequest{Entity: e}
}

func simpleResolvers(repo repository.Repo) entity.Resolvers {
	return entity.Resolvers{
		&identity.Identity{}: identity.NewSimpleResolver(repo),
	}
}

// Read will read a pull request from a repository
func Read(ctx context.Context, repo repository.Repo, id entity.Id) (*PullRequest, error) {
	return ReadWithResolver(ctx, repo, simpleResolvers(repo), id)
}

// ReadWithResolver will read a pull request from its Id, with custom resolvers
func ReadWithResolver(ctx context.Context, repo repository.Repo, resolvers entity.Resolvers, id entity.Id) (*PullRequest, error) {
	return dag.Read(ctx, def, wrapper, repo, resolvers, id)
}

// Validate check if the PullRequest data is valid
func (pr *PullRequest) Validate() error {
	if err := pr.Entity.Validate(); err != nil {
		return err
	}

	// The very first Op should be a CreateOp
	firstOp := pr.FirstOp()
	if firstOp == nil || firstOp.Type() != CreateOp {
		return fmt.Errorf("first operation should be a Create op")
	}

	// Check that there is no more CreateOp op
	for i, op := range pr.Entity.Operations() {
		if i == 0 {
			continue
		}
		if op.Type() == CreateOp {
			return fmt.Errorf("only one Create op allowed")
		}
	}

	return nil
}

// Append add a new Operation to the PullRequest
func (pr *PullRequest) Append(op Operation) {
	pr.Entity.Append(op)
}

// CommitOperations writes the given operations, see dag.Entity.CommitOperations
func (pr *PullRequest) CommitOperations(ctx context.Context, repo repository.Repo, ops []Operation) error {
	// Sanity check: the Create op comes first, and only once. Operations committed
	// here are built by code, not read from elsewhere: breaking that is a misuse.
	creation := pr.Entity.LastCommit() == ""
	converted := make([]dag.Operation, len(ops))
	for i, op := range ops {
		isCreate := op.Type() == CreateOp
		if creation && i == 0 && !isCreate {
			panic("the first operation of a new pull request should be a Create op")
		}
		if isCreate && (!creation || i > 0) {
			panic("only one Create op allowed in a pull request")
		}
		converted[i] = op
	}
	return pr.Entity.CommitOperations(ctx, repo, converted)
}

// Operations return the ordered operations
func (pr *PullRequest) Operations() []Operation {
	source := pr.Entity.Operations()
	result := make([]Operation, len(source))
	for i, op := range source {
		result[i] = op.(Operation)
	}
	return result
}

// Compile a pull request in an easily usable snapshot
func (pr *PullRequest) Compile() *Snapshot {
	snap := &Snapshot{
		id:     pr.Id(),
		Status: OpenStatus,
	}

	for _, op := range pr.Operations() {
		op.Apply(snap)
		snap.Operations = append(snap.Operations, op)
	}

	return snap
}

// FirstOp lookup for the very first operation of the pull request.
// For a valid PullRequest, this operation should be a CreateOp
func (pr *PullRequest) FirstOp() Operation {
	if fo := pr.Entity.FirstOp(); fo != nil {
		return fo.(Operation)
	}
	return nil
}
