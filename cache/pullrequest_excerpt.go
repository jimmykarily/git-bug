package cache

import (
	"time"

	"github.com/git-bug/git-bug/entities/pullrequest"
	"github.com/git-bug/git-bug/entity"
	"github.com/git-bug/git-bug/util/lamport"
)

var _ Excerpt = &PullRequestExcerpt{}

// PullRequestExcerpt holds a subset of the pull request values to be able to
// list them efficiently without having to read and compile each raw pull
// request.
type PullRequestExcerpt struct {
	id entity.Id

	CreateLamportTime lamport.Time
	EditLamportTime   lamport.Time
	CreateUnixTime    int64
	EditUnixTime      int64

	AuthorId entity.Id
	Title    string
	Status   pullrequest.Status
	Source   string
	Target   string
}

func newPullRequestExcerpt(p *sharedPullRequest) *PullRequestExcerpt {
	snap := p.Compile()

	return &PullRequestExcerpt{
		id:                p.Id(),
		CreateLamportTime: p.CreateLamportTime(),
		EditLamportTime:   p.EditLamportTime(),
		CreateUnixTime:    snap.Operations[0].Time().Unix(),
		EditUnixTime:      snap.EditTime().Unix(),
		AuthorId:          snap.Author.Id(),
		Title:             snap.Title,
		Status:            snap.Status,
		Source:            snap.Source,
		Target:            snap.Target,
	}
}

func (p *PullRequestExcerpt) setId(id entity.Id) {
	p.id = id
}

func (p *PullRequestExcerpt) Id() entity.Id {
	return p.id
}

func (p *PullRequestExcerpt) CreateTime() time.Time {
	return time.Unix(p.CreateUnixTime, 0)
}

func (p *PullRequestExcerpt) EditTime() time.Time {
	return time.Unix(p.EditUnixTime, 0)
}
