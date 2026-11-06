package pullrequest

import (
	"slices"
	"time"

	"github.com/git-bug/git-bug/entities/identity"
	"github.com/git-bug/git-bug/entity"
	"github.com/git-bug/git-bug/entity/dag"
	"github.com/git-bug/git-bug/repository"
)

var _ dag.Snapshot = &Snapshot{}

// Snapshot is a compiled form of the PullRequest data structure used for storage and merge
type Snapshot struct {
	id entity.Id

	Title      string
	Message    string
	Author     identity.Interface
	CreateTime time.Time
	Source     string
	Target     string
	Head       repository.Hash
	Status     Status

	Timeline []TimelineItem

	Operations []dag.Operation
}

// Id returns the pull request identifier
func (snap *Snapshot) Id() entity.Id {
	if snap.id == "" {
		// simply panic as it would be a coding error (no id provided at construction)
		panic("no id")
	}
	return snap.id
}

func (snap *Snapshot) AllOperations() []dag.Operation {
	return snap.Operations
}

func (snap *Snapshot) AppendOperation(op dag.Operation) {
	snap.Operations = append(snap.Operations, op)
}

// Clone returns a copy of the snapshot that operations can be applied to without
// the original noticing, and conversely. The slices are copied; operations,
// identities and timeline items are shared, as they don't change once created.
func (snap *Snapshot) Clone() *Snapshot {
	clone := *snap
	clone.Timeline = slices.Clone(snap.Timeline)
	clone.Operations = slices.Clone(snap.Operations)
	return &clone
}

// EditTime returns the last time a pull request was modified
func (snap *Snapshot) EditTime() time.Time {
	if len(snap.Operations) == 0 {
		return time.Unix(0, 0)
	}

	return snap.Operations[len(snap.Operations)-1].Time()
}
