package pullrequest

import "fmt"

// Verdict is the overall outcome of a review.
type Verdict int

const (
	_ Verdict = iota
	CommentVerdict
	ApproveVerdict
	RequestChangesVerdict
)

func (v Verdict) String() string {
	switch v {
	case CommentVerdict:
		return "comment"
	case ApproveVerdict:
		return "approve"
	case RequestChangesVerdict:
		return "request-changes"
	default:
		return "unknown verdict"
	}
}

func (v Verdict) Validate() error {
	if v != CommentVerdict && v != ApproveVerdict && v != RequestChangesVerdict {
		return fmt.Errorf("invalid")
	}
	return nil
}
