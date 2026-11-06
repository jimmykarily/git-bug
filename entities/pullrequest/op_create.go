package pullrequest

import (
	"fmt"

	"github.com/git-bug/git-bug/entities/identity"
	"github.com/git-bug/git-bug/entity"
	"github.com/git-bug/git-bug/entity/dag"
	"github.com/git-bug/git-bug/repository"
	"github.com/git-bug/git-bug/util/text"
)

var _ Operation = &CreateOperation{}

// CreateOperation define the initial creation of a pull request
type CreateOperation struct {
	dag.OpBase
	Title   string          `json:"title"`
	Message string          `json:"message"`
	Source  string          `json:"source"`
	Target  string          `json:"target"`
	Head    repository.Hash `json:"head"`
}

func (op *CreateOperation) Id() entity.Id {
	return dag.IdOperation(op, &op.OpBase)
}

func (op *CreateOperation) Apply(snapshot *Snapshot) {
	// sanity check: will fail when adding a second Create
	if snapshot.id != "" && snapshot.id != entity.UnsetId && snapshot.id != op.Id() {
		return
	}

	// the Id of the pull request is the Id of the first Operation: CreateOperation
	snapshot.id = op.Id()

	snapshot.Title = op.Title
	snapshot.Message = op.Message
	snapshot.Author = op.Author()
	snapshot.CreateTime = op.Time()
	snapshot.Source = op.Source
	snapshot.Target = op.Target
	snapshot.Head = op.Head
}

func (op *CreateOperation) Validate() error {
	if err := op.OpBase.Validate(op, CreateOp); err != nil {
		return err
	}

	if text.Empty(op.Title) {
		return fmt.Errorf("title is empty")
	}
	if !text.SafeOneLine(op.Title) {
		return fmt.Errorf("title has unsafe characters")
	}

	if !text.Safe(op.Message) {
		return fmt.Errorf("message is not fully printable")
	}

	if text.Empty(op.Source) {
		return fmt.Errorf("source branch is empty")
	}
	if !text.SafeOneLine(op.Source) {
		return fmt.Errorf("source branch has unsafe characters")
	}

	if text.Empty(op.Target) {
		return fmt.Errorf("target branch is empty")
	}
	if !text.SafeOneLine(op.Target) {
		return fmt.Errorf("target branch has unsafe characters")
	}

	if !op.Head.IsValid() {
		return fmt.Errorf("head is not a valid commit hash")
	}

	return nil
}

func NewCreateOp(author identity.Interface, unixTime int64, title, message, source, target string, head repository.Hash) *CreateOperation {
	return &CreateOperation{
		OpBase:  dag.NewOpBase(CreateOp, author, unixTime),
		Title:   title,
		Message: message,
		Source:  source,
		Target:  target,
		Head:    head,
	}
}

// Create is a convenience function to create a pull request
func Create(author identity.Interface, unixTime int64, title, message, source, target string, head repository.Hash) (*PullRequest, *CreateOperation, error) {
	pr := New()
	op := NewCreateOp(author, unixTime, title, message, source, target, head)
	if err := op.Validate(); err != nil {
		return nil, op, err
	}
	pr.Append(op)
	return pr, op, nil
}
