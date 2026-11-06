package pullrequest

import (
	"fmt"

	"github.com/git-bug/git-bug/entities/identity"
	"github.com/git-bug/git-bug/entity"
	"github.com/git-bug/git-bug/entity/dag"
	"github.com/git-bug/git-bug/util/timestamp"
)

var _ Operation = &SetStatusOperation{}

// SetStatusOperation will change the status of a pull request
type SetStatusOperation struct {
	dag.OpBase
	Status Status `json:"status"`
}

func (op *SetStatusOperation) Id() entity.Id {
	return dag.IdOperation(op, &op.OpBase)
}

func (op *SetStatusOperation) Apply(snapshot *Snapshot) {
	snapshot.Status = op.Status

	snapshot.Timeline = append(snapshot.Timeline, &StatusTimelineItem{
		combinedId: entity.CombineIds(snapshot.Id(), op.Id()),
		Author:     op.Author(),
		UnixTime:   timestamp.Timestamp(op.UnixTime),
		Status:     op.Status,
	})
}

func (op *SetStatusOperation) Validate() error {
	if err := op.OpBase.Validate(op, SetStatusOp); err != nil {
		return err
	}

	if err := op.Status.Validate(); err != nil {
		return fmt.Errorf("status: %w", err)
	}

	return nil
}

func NewSetStatusOp(author identity.Interface, unixTime int64, status Status) *SetStatusOperation {
	return &SetStatusOperation{
		OpBase: dag.NewOpBase(SetStatusOp, author, unixTime),
		Status: status,
	}
}

// StatusTimelineItem is a status change in the timeline
type StatusTimelineItem struct {
	combinedId entity.CombinedId
	Author     identity.Interface
	UnixTime   timestamp.Timestamp
	Status     Status
}

func (s *StatusTimelineItem) CombinedId() entity.CombinedId {
	return s.combinedId
}

// SetStatus is a convenience function to change the status of a pull request
func SetStatus(pr Mutable, author identity.Interface, unixTime int64, status Status) (*SetStatusOperation, error) {
	op := NewSetStatusOp(author, unixTime, status)
	if err := op.Validate(); err != nil {
		return nil, err
	}
	pr.Append(op)
	return op, nil
}
