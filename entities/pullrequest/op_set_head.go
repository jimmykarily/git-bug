package pullrequest

import (
	"fmt"

	"github.com/git-bug/git-bug/entities/identity"
	"github.com/git-bug/git-bug/entity"
	"github.com/git-bug/git-bug/entity/dag"
	"github.com/git-bug/git-bug/repository"
	"github.com/git-bug/git-bug/util/timestamp"
)

var _ Operation = &SetHeadOperation{}

// SetHeadOperation will record a new head commit for the pull request
type SetHeadOperation struct {
	dag.OpBase
	Head repository.Hash `json:"head"`
}

func (op *SetHeadOperation) Id() entity.Id {
	return dag.IdOperation(op, &op.OpBase)
}

func (op *SetHeadOperation) Apply(snapshot *Snapshot) {
	snapshot.Head = op.Head

	snapshot.Timeline = append(snapshot.Timeline, &HeadTimelineItem{
		combinedId: entity.CombineIds(snapshot.Id(), op.Id()),
		Author:     op.Author(),
		UnixTime:   timestamp.Timestamp(op.UnixTime),
		Head:       op.Head,
	})
}

func (op *SetHeadOperation) Validate() error {
	if err := op.OpBase.Validate(op, SetHeadOp); err != nil {
		return err
	}

	if !op.Head.IsValid() {
		return fmt.Errorf("head is not a valid commit hash")
	}

	return nil
}

func NewSetHeadOp(author identity.Interface, unixTime int64, head repository.Hash) *SetHeadOperation {
	return &SetHeadOperation{
		OpBase: dag.NewOpBase(SetHeadOp, author, unixTime),
		Head:   head,
	}
}

// HeadTimelineItem is a head change in the timeline
type HeadTimelineItem struct {
	combinedId entity.CombinedId
	Author     identity.Interface
	UnixTime   timestamp.Timestamp
	Head       repository.Hash
}

func (h *HeadTimelineItem) CombinedId() entity.CombinedId {
	return h.combinedId
}

// SetHead is a convenience function to record a new head of a pull request
func SetHead(pr Mutable, author identity.Interface, unixTime int64, head repository.Hash) (*SetHeadOperation, error) {
	op := NewSetHeadOp(author, unixTime, head)
	if err := op.Validate(); err != nil {
		return nil, err
	}
	pr.Append(op)
	return op, nil
}
