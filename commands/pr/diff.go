package prcmd

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/git-bug/git-bug/commands/execenv"
	"github.com/git-bug/git-bug/entities/pullrequest"
	"github.com/git-bug/git-bug/repository"
	"github.com/git-bug/git-bug/util/colors"
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
// had one, because repository.DiffLine does not record it. When after is not
// nil, it is called after each line of code, but not after header lines.
// The output has no colors.
func writeUnifiedDiff(w io.Writer, files []repository.FileDiff, prefix string, after func(file repository.FileDiff, line repository.DiffLine)) {
	writeDiff(w, files, prefix, false, after)
}

// writeDiff is writeUnifiedDiff that colors the lines as git diff does when
// colored is true: bold file headers, cyan hunk headers, green added lines and
// red removed lines. The colors are still off when the output is not a
// terminal or when they are disabled.
func writeDiff(w io.Writer, files []repository.FileDiff, prefix string, colored bool, after func(file repository.FileDiff, line repository.DiffLine)) {
	paint := func(style func(a ...interface{}) string, s string) string {
		if colored && style != nil {
			return style(s)
		}
		return s
	}
	lineWith := func(style func(a ...interface{}) string, format string, args ...any) {
		_, _ = fmt.Fprintf(w, "%s%s\n", prefix, paint(style, fmt.Sprintf(format, args...)))
	}
	line := func(format string, args ...any) {
		lineWith(colors.Bold, format, args...)
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
			lineWith(colors.Cyan, "@@ -%d,%d +%d,%d @@", h.OldStart, h.OldLines, h.NewStart, h.NewLines)
			for _, l := range h.Lines {
				marker := " "
				var style func(a ...interface{}) string
				switch l.Type {
				case repository.DiffLineAdded:
					marker, style = "+", colors.Green
				case repository.DiffLineDeleted:
					marker, style = "-", colors.Red
				}
				lineWith(style, "%s%s", marker, l.Content)
				if after != nil {
					after(f, l)
				}
			}
		}
	}
}

// diffCommentDateLayout is the date layout of a comment printed in the diff.
const diffCommentDateLayout = "Mon Jan 2 15:04 2006"

// diffComment is a line comment with the review it belongs to.
type diffComment struct {
	review  *pullrequest.ReviewTimelineItem
	comment pullrequest.LineComment
}

// onLine tells whether the comment is about the line of the file in the diff
// that has the given head and merge base. A comment with a new line matches
// an added or unchanged line. One with only an old line matches a deleted
// line of the file that had the comment's old path at the base.
func (d diffComment) onLine(head, base repository.Hash, f repository.FileDiff, l repository.DiffLine) bool {
	c := d.comment
	if c.Commit != head || c.Base != base {
		return false
	}
	if c.NewLine != 0 {
		return l.Type != repository.DiffLineDeleted && f.Path == c.Path && l.NewLine == c.NewLine
	}
	oldPath, fileOldPath := c.Path, f.Path
	if c.OldPath != "" {
		oldPath = c.OldPath
	}
	if f.OldPath != nil {
		fileOldPath = *f.OldPath
	}
	return l.Type == repository.DiffLineDeleted && fileOldPath == oldPath && l.OldLine == c.OldLine
}

// commentIndent is what a comment printed in the diff starts with.
const commentIndent = "    ┃"

// header returns the line that starts a printed comment: the author, the date
// and the verdict of its review.
func (d diffComment) header() string {
	return fmt.Sprintf("%s %s%s (%s)\n", colors.Dim(commentIndent), colors.Magenta(d.review.Author.DisplayName()),
		colors.Dim(", "+d.review.UnixTime.Time().Format(diffCommentDateLayout)), coloredVerdict(d.review.Verdict))
}

// printDiffWithComments prints the diff with each line comment under the line
// it is about, then lists the comments that are not on a line of the diff.
func printDiffWithComments(env *execenv.Env, snap *pullrequest.Snapshot, base repository.Hash, files []repository.FileDiff) {
	var comments []diffComment
	for _, item := range snap.Timeline {
		if r, ok := item.(*pullrequest.ReviewTimelineItem); ok {
			for _, c := range r.Comments {
				comments = append(comments, diffComment{review: r, comment: c})
			}
		}
	}

	shown := make([]bool, len(comments))
	writeDiff(env.Out, files, "", true, func(f repository.FileDiff, l repository.DiffLine) {
		for i, c := range comments {
			if !c.onLine(snap.Head, base, f, l) {
				continue
			}
			shown[i] = true
			env.Out.Printf("%s", c.header())
			for _, m := range strings.Split(c.comment.Message, "\n") {
				if m == "" {
					env.Out.Printf("%s\n", colors.Dim(commentIndent))
				} else {
					env.Out.Printf("%s %s\n", colors.Dim(commentIndent), m)
				}
			}
		}
	})

	first := true
	for i, c := range comments {
		if shown[i] {
			continue
		}
		if first {
			env.Out.Printf("\nComments not on a line of this diff:\n")
			first = false
		}
		env.Out.Printf("%s", c.header())
		printLineComment(env, snap, c.comment)
	}
}
