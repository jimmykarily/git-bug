package prcmd

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/git-bug/git-bug/entities/pullrequest"
	"github.com/git-bug/git-bug/repository"
	"github.com/git-bug/git-bug/util/text"
)

const reviewFilename = "PR_REVIEW_EDITMSG"

const reviewTemplateHeader = `# Review of %s %q (%s -> %s, head %s)
# First line: comment, approve or request-changes.
# Text before the diff is the summary. Write a comment on the line(s) right
# under the diff line it is about. Lines starting with # are ignored.
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
	writeUnifiedDiff(&buf, files, "> ")
	return buf.String()
}

// quotedLine is one line of the diff as written in the template, without its
// quote. target is nil for file and hunk header lines.
type quotedLine struct {
	text   string
	target *lineTarget
}

// quotedLines returns the lines writeUnifiedDiff writes for files, each with
// the position a comment under it would have.
func quotedLines(files []repository.FileDiff) []quotedLine {
	var res []quotedLine
	for _, f := range files {
		var buf bytes.Buffer
		writeUnifiedDiff(&buf, []repository.FileDiff{f}, "")
		rendered := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")

		// The file header lines come first, then each hunk as its header
		// followed by its lines.
		headers := len(rendered)
		for _, h := range f.Hunks {
			headers -= 1 + len(h.Lines)
		}
		for _, l := range rendered[:headers] {
			res = append(res, quotedLine{text: l})
		}
		i := headers
		for _, h := range f.Hunks {
			res = append(res, quotedLine{text: rendered[i]})
			i++
			for _, l := range h.Lines {
				target := &lineTarget{Path: f.Path, OldLine: l.OldLine, NewLine: l.NewLine}
				if f.OldPath != nil && l.OldLine > 0 {
					target.OldPath = *f.OldPath
				}
				res = append(res, quotedLine{text: rendered[i], target: target})
				i++
			}
		}
	}
	return res
}

// parseReview reads the text edited from reviewTemplate. The first line that
// is not a # comment is the verdict, unquoted text before the diff is the
// summary, and unquoted text under a quoted diff line is a comment on that
// line. Each quoted line is matched with the next identical line of the diff
// computed from files, so the reviewer may delete quoted lines, and a quoted
// line that is not in files cannot be commented on. The verdict is empty when
// the text holds nothing but # comments and blank lines.
func parseReview(text string, files []repository.FileDiff) (verdict, summary string, comments []lineTarget, err error) {
	diff := quotedLines(files)
	next := 0 // index in diff of the first line not matched yet

	const (
		beforeDiff = iota
		underHeader
		underUnknown
		underLine
	)
	state := beforeDiff
	var current *lineTarget
	var summaryLines []string
	var message []string

	endComment := func() {
		if len(message) > 0 {
			c := *current
			c.Message = cleanupText(strings.Join(message, "\n"))
			comments = append(comments, c)
			message = nil
		}
	}

	for i, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		if verdict == "" {
			if strings.TrimSpace(line) == "" {
				continue
			}
			verdict = strings.TrimSpace(line)
			if _, ok := verdictByName(verdict); !ok {
				return "", "", nil, fmt.Errorf("unknown verdict %q, expected comment, approve or request-changes", verdict)
			}
			continue
		}

		if strings.HasPrefix(line, ">") {
			endComment()
			quoted := strings.TrimRight(strings.TrimPrefix(line, ">"), " \t\r")
			quoted = strings.TrimPrefix(quoted, " ")
			state = underUnknown
			for j := next; j < len(diff); j++ {
				if strings.TrimRight(diff[j].text, " \t\r") == quoted {
					next = j + 1
					current = diff[j].target
					state = underHeader
					if current != nil {
						state = underLine
					}
					break
				}
			}
			continue
		}

		switch state {
		case beforeDiff:
			summaryLines = append(summaryLines, line)
		case underLine:
			if strings.TrimSpace(line) == "" {
				endComment()
				continue
			}
			message = append(message, line)
		default:
			if strings.TrimSpace(line) == "" {
				continue
			}
			if state == underHeader {
				return "", "", nil, fmt.Errorf("line %d: a comment must be under a line of the diff, not under a file or hunk header", i+1)
			}
			return "", "", nil, fmt.Errorf("line %d: the comment is not under a line of the diff", i+1)
		}
	}
	endComment()

	return verdict, cleanupText(strings.Join(summaryLines, "\n")), comments, nil
}

// cleanupText calls text.Cleanup, which parseReview cannot name because its
// argument is called text.
func cleanupText(s string) string {
	return text.Cleanup(s)
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
