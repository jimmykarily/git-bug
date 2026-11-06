package prcmd

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/git-bug/git-bug/entities/pullrequest"
	"github.com/git-bug/git-bug/repository"
	"github.com/git-bug/git-bug/util/text"
)

const reviewFilename = "PR_REVIEW_EDITMSG"

const reviewTemplateHeader = `# Reviewing %s %q (%s -> %s, head %s)
#
# The first line below is the verdict: comment, approve or request-changes.
# Text between the verdict and the diff is your summary.
# To comment on a line of the diff, add a new line right under it that does
# not start with ">", and write your comment there:
#
#   > +	greet()
#   Please add a test for greet.
#
# Do not edit the ">" lines, and keep all of them above a comment. You may
# delete whole files, whole hunks (from an "@@" line), or the end of a hunk.
# Lines starting with "#" are ignored.
%s

`

// lineTarget is a comment written in the editor under one line of the diff.
type lineTarget struct {
	Path    string
	OldPath string
	OldLine int
	NewLine int
	Message string
}

// reviewTemplate returns the text the reviewer edits: a commented header, the
// verdict on its own line, and the diff with every line quoted with "> ".
func reviewTemplate(id, title, source, target string, head repository.Hash, verdict pullrequest.Verdict, files []repository.FileDiff) string {
	var buf bytes.Buffer
	_, _ = fmt.Fprintf(&buf, reviewTemplateHeader, id, title, source, target, short(head), verdict)
	writeUnifiedDiff(&buf, files, "> ", nil)
	return buf.String()
}

// renderedFile is one file of the diff as writeUnifiedDiff writes it.
type renderedFile struct {
	path    string
	headers []string // the file header lines, from "diff --git" on
	hunks   []renderedHunk
}

// renderedHunk is a hunk header line and the code lines under it, each with
// the position a comment under it has.
type renderedHunk struct {
	header string
	lines  []string
	target []lineTarget
}

func renderFiles(files []repository.FileDiff) []renderedFile {
	res := make([]renderedFile, 0, len(files))
	for _, f := range files {
		var buf bytes.Buffer
		writeUnifiedDiff(&buf, []repository.FileDiff{f}, "", nil)
		rendered := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")

		// The file header lines come first, then each hunk as its header
		// followed by its lines.
		headers := len(rendered)
		for _, h := range f.Hunks {
			headers -= 1 + len(h.Lines)
		}
		rf := renderedFile{path: f.Path, headers: rendered[:headers]}
		i := headers
		for _, h := range f.Hunks {
			rh := renderedHunk{header: rendered[i]}
			i++
			for _, l := range h.Lines {
				target := lineTarget{Path: f.Path, OldLine: l.OldLine, NewLine: l.NewLine}
				if f.OldPath != nil && l.OldLine > 0 {
					target.OldPath = *f.OldPath
				}
				rh.lines = append(rh.lines, rendered[i])
				rh.target = append(rh.target, target)
				i++
			}
			rf.hunks = append(rf.hunks, rh)
		}
		res = append(res, rf)
	}
	return res
}

// parseReview reads the text edited from reviewTemplate. The first line that
// is not a # comment is the verdict, unquoted text before the diff is the
// summary, and unquoted text under a quoted code line, up to the next quoted
// line, is a comment on that line.
//
// Quoted lines are placed by their headers: a "diff --git" or "+++" line
// selects a file of the diff, a hunk header selects a hunk of that file, and
// the lines under it must follow that hunk exactly. So the reviewer may delete
// whole files, whole hunks or the end of a hunk, and anything else is an error.
// The verdict is empty when the text holds nothing but # comments and blank
// lines.
func parseReview(edited string, files []repository.FileDiff) (verdict, summary string, comments []lineTarget, err error) {
	diff := renderFiles(files)
	var file *renderedFile
	var hunk *renderedHunk
	// Files and hunks must come in the order of the diff, so each header
	// only looks after the current file and hunk.
	fileIndex, hunkIndex := -1, -1
	next := 0 // index in hunk.lines of the next expected line

	const (
		beforeDiff = iota
		underHeader
		underLine
	)
	state := beforeDiff
	var current lineTarget
	var summaryLines []string
	var message []string

	endComment := func() {
		if len(message) > 0 {
			c := current
			c.Message = text.Cleanup(strings.Join(message, "\n"))
			comments = append(comments, c)
			message = nil
		}
	}

	for i, line := range strings.Split(edited, "\n") {
		lineErr := func(format string, args ...any) error {
			return fmt.Errorf("line %d: "+format, append([]any{i + 1}, args...)...)
		}

		if strings.HasPrefix(line, "#") {
			continue
		}
		if verdict == "" {
			if strings.TrimSpace(line) == "" {
				continue
			}
			if strings.HasPrefix(line, ">") {
				return "", "", nil, errors.New("the first line must be the verdict (comment, approve or request-changes)")
			}
			verdict = strings.TrimSpace(line)
			if _, ok := verdictByName(verdict); !ok {
				return "", "", nil, fmt.Errorf("unknown verdict %q, expected comment, approve or request-changes", verdict)
			}
			continue
		}

		if strings.HasPrefix(line, ">") {
			endComment()
			quoted := strings.TrimPrefix(trimRight(strings.TrimPrefix(line, ">")), " ")
			state = underHeader

			switch {
			case hunk != nil && next < len(hunk.lines) && trimRight(hunk.lines[next]) == quoted:
				current = hunk.target[next]
				next++
				state = underLine

			case strings.HasPrefix(quoted, "diff --git "), strings.HasPrefix(quoted, "+++ b/"):
				found := -1
				for j := range diff {
					if containsTrimmed(diff[j].headers, quoted) {
						found = j
						break
					}
				}
				switch {
				case found < 0:
					return "", "", nil, lineErr("this file is not in the diff")
				// "+++ b/" may follow "diff --git" of the same file.
				case found < fileIndex || (found == fileIndex && hunk != nil):
					return "", "", nil, lineErr("files and hunks must stay in the order of the diff")
				}
				fileIndex, file = found, &diff[found]
				hunkIndex, hunk = -1, nil

			case strings.HasPrefix(quoted, "@@ "):
				if file == nil {
					return "", "", nil, lineErr("this hunk header has no file header above it")
				}
				found := -1
				for j := range file.hunks {
					if trimRight(file.hunks[j].header) == quoted {
						found = j
						break
					}
				}
				switch {
				case found < 0:
					return "", "", nil, lineErr("this hunk header is not in the diff of %s", file.path)
				case found <= hunkIndex:
					return "", "", nil, lineErr("files and hunks must stay in the order of the diff")
				}
				hunkIndex, hunk = found, &file.hunks[found]
				next = 0

			case hunk != nil:
				return "", "", nil, lineErr("quoted lines inside a hunk must be kept as they are; you may delete whole files, whole hunks, or the end of a hunk")

			case file != nil && containsTrimmed(file.headers, quoted):
				// another header line of the selected file

			default:
				return "", "", nil, lineErr("a quoted line of code needs its file and hunk header above it")
			}
			continue
		}

		switch state {
		case beforeDiff:
			summaryLines = append(summaryLines, line)
		case underLine:
			// Blank lines separate paragraphs of the comment; a run of
			// them counts as one.
			if strings.TrimSpace(line) == "" {
				if len(message) > 0 && message[len(message)-1] != "" {
					message = append(message, "")
				}
				continue
			}
			message = append(message, line)
		case underHeader:
			if strings.TrimSpace(line) == "" {
				continue
			}
			return "", "", nil, lineErr("a comment must be under a line of the diff, not under a file or hunk header")
		}
	}
	endComment()

	return verdict, text.Cleanup(strings.Join(summaryLines, "\n")), comments, nil
}

// trimRight removes the trailing whitespace an editor may strip.
func trimRight(s string) string {
	return strings.TrimRight(s, " \t\r")
}

func containsTrimmed(lines []string, quoted string) bool {
	for _, l := range lines {
		if trimRight(l) == quoted {
			return true
		}
	}
	return false
}

// verdictByName returns the verdict written as name in the editor.
func verdictByName(name string) (pullrequest.Verdict, bool) {
	for _, v := range []pullrequest.Verdict{pullrequest.CommentVerdict, pullrequest.ApproveVerdict, pullrequest.RequestChangesVerdict} {
		if v.String() == name {
			return v, true
		}
	}
	return 0, false
}
