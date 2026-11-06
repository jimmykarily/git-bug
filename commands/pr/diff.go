package prcmd

import (
	"github.com/git-bug/git-bug/commands/execenv"
	"github.com/git-bug/git-bug/entities/pullrequest"
	"github.com/git-bug/git-bug/repository"
)

// prDiff returns the merge base of the target branch and the head of the pull
// request, and the diff from that merge base to the head.
func prDiff(env *execenv.Env, snap *pullrequest.Snapshot) (repository.Hash, []repository.FileDiff, error) {
	target, err := env.Backend.PullRequests().ResolveBranch(env.Ctx, snap.Target)
	if err != nil {
		return "", nil, err
	}
	base, err := env.Repo.MergeBase(env.Ctx, target, snap.Head)
	if err != nil {
		return "", nil, err
	}
	files, err := env.Repo.DiffCommits(env.Ctx, base, snap.Head)
	if err != nil {
		return "", nil, err
	}
	return base, files, nil
}

// unchangedLine returns the line number on the old side for a line of the new
// side of the diff, when that line is an unchanged line shown in a hunk. It
// returns 0 for any other line.
func unchangedLine(files []repository.FileDiff, path string, newLine int) int {
	for _, f := range files {
		if f.Path != path {
			continue
		}
		for _, h := range f.Hunks {
			for _, l := range h.Lines {
				if l.Type == repository.DiffLineContext && l.NewLine == newLine {
					return l.OldLine
				}
			}
		}
	}
	return 0
}
