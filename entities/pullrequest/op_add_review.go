package pullrequest

import (
	"fmt"

	"github.com/git-bug/git-bug/entities/identity"
	"github.com/git-bug/git-bug/entity"
	"github.com/git-bug/git-bug/entity/dag"
	"github.com/git-bug/git-bug/repository"
	"github.com/git-bug/git-bug/util/text"
	"github.com/git-bug/git-bug/util/timestamp"
)

var _ Operation = &AddReviewOperation{}

// LineComment is a comment on one line of a file, as it is at a given commit
type LineComment struct {
	Commit  repository.Hash `json:"commit"`
	Path    string          `json:"path"`
	Line    int             `json:"line"`
	Message string          `json:"message"`
}

func (c LineComment) Validate() error {
	if !c.Commit.IsValid() {
		return fmt.Errorf("commit is not a valid commit hash")
	}
	if text.Empty(c.Path) {
		return fmt.Errorf("path is empty")
	}
	if !text.SafeOneLine(c.Path) {
		return fmt.Errorf("path has unsafe characters")
	}
	if c.Line < 1 {
		return fmt.Errorf("line must be at least 1")
	}
	if text.Empty(c.Message) {
		return fmt.Errorf("message is empty")
	}
	if !text.Safe(c.Message) {
		return fmt.Errorf("message is not fully printable")
	}
	return nil
}

// AddReviewOperation will add a submitted review to the pull request
type AddReviewOperation struct {
	dag.OpBase
	Verdict  Verdict       `json:"verdict"`
	Message  string        `json:"message"`
	Comments []LineComment `json:"comments"`
}

func (op *AddReviewOperation) Id() entity.Id {
	return dag.IdOperation(op, &op.OpBase)
}

func (op *AddReviewOperation) Apply(snapshot *Snapshot) {
	snapshot.Timeline = append(snapshot.Timeline, &ReviewTimelineItem{
		combinedId: entity.CombineIds(snapshot.Id(), op.Id()),
		Author:     op.Author(),
		UnixTime:   timestamp.Timestamp(op.UnixTime),
		Verdict:    op.Verdict,
		Message:    op.Message,
		Comments:   op.Comments,
	})
}

func (op *AddReviewOperation) Validate() error {
	if err := op.OpBase.Validate(op, AddReviewOp); err != nil {
		return err
	}

	if err := op.Verdict.Validate(); err != nil {
		return fmt.Errorf("verdict: %w", err)
	}

	if !text.Safe(op.Message) {
		return fmt.Errorf("message is not fully printable")
	}

	if op.Verdict == CommentVerdict && text.Empty(op.Message) && len(op.Comments) == 0 {
		return fmt.Errorf("a comment review needs a message or a line comment")
	}

	for i, c := range op.Comments {
		if err := c.Validate(); err != nil {
			return fmt.Errorf("line comment %d: %w", i+1, err)
		}
	}

	return nil
}

func NewAddReviewOp(author identity.Interface, unixTime int64, verdict Verdict, message string, comments []LineComment) *AddReviewOperation {
	return &AddReviewOperation{
		OpBase:   dag.NewOpBase(AddReviewOp, author, unixTime),
		Verdict:  verdict,
		Message:  message,
		Comments: comments,
	}
}

// ReviewTimelineItem is a submitted review in the timeline
type ReviewTimelineItem struct {
	combinedId entity.CombinedId
	Author     identity.Interface
	UnixTime   timestamp.Timestamp
	Verdict    Verdict
	Message    string
	Comments   []LineComment
}

func (r *ReviewTimelineItem) CombinedId() entity.CombinedId {
	return r.combinedId
}

// AddReview is a convenience function to add a review to a pull request
func AddReview(pr Mutable, author identity.Interface, unixTime int64, verdict Verdict, message string, comments []LineComment) (*AddReviewOperation, error) {
	op := NewAddReviewOp(author, unixTime, verdict, message, comments)
	if err := op.Validate(); err != nil {
		return nil, err
	}
	pr.Append(op)
	return op, nil
}
