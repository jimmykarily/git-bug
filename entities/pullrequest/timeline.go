package pullrequest

import "github.com/git-bug/git-bug/entity"

// TimelineItem is one entry of the pull request timeline.
type TimelineItem interface {
	// CombinedId returns the global identifier of the item
	CombinedId() entity.CombinedId
}
