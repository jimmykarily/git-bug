package prcmd

import (
	"errors"
	"fmt"
	"io"

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
	if errors.Is(err, repository.ErrNotFound) {
		return "", nil, fmt.Errorf("no merge base between %q and the head %s in this clone", snap.Target, short(snap.Head))
	}
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
// side of the diff, when that line is an unchanged line shown in a hunk, and
// the old path of the file when the diff reports it as renamed. It returns 0
// and nil for any other line.
func unchangedLine(files []repository.FileDiff, path string, newLine int) (int, *string) {
	for _, f := range files {
		if f.Path != path {
			continue
		}
		for _, h := range f.Hunks {
			for _, l := range h.Lines {
				if l.Type == repository.DiffLineContext && l.NewLine == newLine {
					return l.OldLine, f.OldPath
				}
			}
		}
	}
	return 0, nil
}

// writeUnifiedDiff writes the files in unified diff format, with prefix in
// front of every line. Files without a trailing newline are printed as if they
// had one, because repository.DiffLine does not record it.
func writeUnifiedDiff(w io.Writer, files []repository.FileDiff, prefix string) {
	line := func(format string, args ...any) {
		_, _ = fmt.Fprintf(w, "%s%s\n", prefix, fmt.Sprintf(format, args...))
	}

	for _, f := range files {
		oldPath := f.Path
		if f.OldPath != nil {
			oldPath = *f.OldPath
		}
		line("diff --git a/%s b/%s", oldPath, f.Path)
		if f.IsBinary {
			line("Binary files differ")
			continue
		}
		switch {
		case f.IsNew:
			line("new file mode 100644")
		case f.IsDelete:
			line("deleted file mode 100644")
		}
		if f.OldPath != nil {
			if len(f.Hunks) == 0 {
				line("similarity index 100%%")
			}
			line("rename from %s", oldPath)
			line("rename to %s", f.Path)
		}
		if len(f.Hunks) == 0 {
			continue
		}
		if f.IsNew {
			line("--- /dev/null")
		} else {
			line("--- a/%s", oldPath)
		}
		if f.IsDelete {
			line("+++ /dev/null")
		} else {
			line("+++ b/%s", f.Path)
		}
		for _, h := range f.Hunks {
			line("@@ -%d,%d +%d,%d @@", h.OldStart, h.OldLines, h.NewStart, h.NewLines)
			for _, l := range h.Lines {
				marker := " "
				switch l.Type {
				case repository.DiffLineAdded:
					marker = "+"
				case repository.DiffLineDeleted:
					marker = "-"
				}
				line("%s%s", marker, l.Content)
			}
		}
	}
}
