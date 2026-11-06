package pullrequest

import (
	"context"
	"encoding/json"

	"github.com/git-bug/git-bug/entity"
	"github.com/git-bug/git-bug/entity/dag"
)

const (
	_ dag.OperationType = iota
	CreateOp
	AddCommentOp
	AddReviewOp
	SetHeadOp
	SetStatusOp
)

// Operation define the interface to fulfill for an edit operation of a pull request
type Operation = dag.OperationWithApply[*Snapshot]

func operationUnmarshaler(_ context.Context, raw json.RawMessage, _ entity.Resolvers) (dag.Operation, error) {
	var t struct {
		OperationType dag.OperationType `json:"type"`
	}

	if err := json.Unmarshal(raw, &t); err != nil {
		return nil, err
	}

	var op dag.Operation

	switch t.OperationType {
	case CreateOp:
		op = &CreateOperation{}
	case AddCommentOp:
		op = &AddCommentOperation{}
	case AddReviewOp:
		op = &AddReviewOperation{}
	case SetHeadOp:
		op = &SetHeadOperation{}
	case SetStatusOp:
		op = &SetStatusOperation{}
	default:
		return dag.NewUnknownOp[*Snapshot](raw)
	}

	err := json.Unmarshal(raw, &op)
	if err != nil {
		return nil, err
	}

	return op, nil
}
