package pullrequest

import (
	"fmt"

	"github.com/git-bug/git-bug/entities/identity"
	"github.com/git-bug/git-bug/entity"
	"github.com/git-bug/git-bug/entity/dag"
	"github.com/git-bug/git-bug/util/text"
	"github.com/git-bug/git-bug/util/timestamp"
)

var _ Operation = &AddCommentOperation{}

// AddCommentOperation will add a general comment to the pull request
type AddCommentOperation struct {
	dag.OpBase
	Message string `json:"message"`
}

func (op *AddCommentOperation) Id() entity.Id {
	return dag.IdOperation(op, &op.OpBase)
}

func (op *AddCommentOperation) Apply(snapshot *Snapshot) {
	snapshot.Timeline = append(snapshot.Timeline, &CommentTimelineItem{
		combinedId: entity.CombineIds(snapshot.Id(), op.Id()),
		Author:     op.Author(),
		UnixTime:   timestamp.Timestamp(op.UnixTime),
		Message:    op.Message,
	})
}

func (op *AddCommentOperation) Validate() error {
	if err := op.OpBase.Validate(op, AddCommentOp); err != nil {
		return err
	}

	if text.Empty(op.Message) {
		return fmt.Errorf("message is empty")
	}
	if !text.Safe(op.Message) {
		return fmt.Errorf("message is not fully printable")
	}

	return nil
}

func NewAddCommentOp(author identity.Interface, unixTime int64, message string) *AddCommentOperation {
	return &AddCommentOperation{
		OpBase:  dag.NewOpBase(AddCommentOp, author, unixTime),
		Message: message,
	}
}

// CommentTimelineItem is a general comment in the timeline
type CommentTimelineItem struct {
	combinedId entity.CombinedId
	Author     identity.Interface
	UnixTime   timestamp.Timestamp
	Message    string
}

func (c *CommentTimelineItem) CombinedId() entity.CombinedId {
	return c.combinedId
}

// AddComment is a convenience function to add a comment to a pull request
func AddComment(pr Mutable, author identity.Interface, unixTime int64, message string) (*AddCommentOperation, error) {
	op := NewAddCommentOp(author, unixTime, message)
	if err := op.Validate(); err != nil {
		return nil, err
	}
	pr.Append(op)
	return op, nil
}
