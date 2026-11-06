package prcmd

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/git-bug/git-bug/commands/execenv"
	"github.com/git-bug/git-bug/entities/identity"
	"github.com/git-bug/git-bug/entities/pullrequest"
	"github.com/git-bug/git-bug/repository"
	"github.com/git-bug/git-bug/util/colors"
	"github.com/git-bug/git-bug/util/text"
	"github.com/git-bug/git-bug/util/timestamp"
)

const dateLayout = "Mon Jan 2 15:04:05 2006 -0700"

func newPrShowCommand(env *execenv.Env) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show PR_ID",
		Short:   "Display the details of a pull request",
		Args:    cobra.ExactArgs(1),
		PreRunE: execenv.LoadBackend(env),
		RunE: execenv.CloseBackend(env, func(cmd *cobra.Command, args []string) error {
			return runPrShow(env, args)
		}),
	}

	return cmd
}

func runPrShow(env *execenv.Env, args []string) error {
	pr, err := env.Backend.PullRequests().ResolvePrefix(env.Ctx, args[0])
	if err != nil {
		return err
	}
	snap := pr.Snapshot()

	env.Out.Printf("%s [%s]\n", colors.Bold(snap.Title), colors.Yellow(snap.Status))
	env.Out.Printf("%s -> %s, head %s\n", snap.Source, snap.Target, short(snap.Head))
	env.Out.Printf("opened by %s on %s\n", colors.Magenta(snap.Author.DisplayName()), snap.CreateTime.Format(dateLayout))
	if !text.Empty(snap.Message) {
		env.Out.Printf("\n%s\n", snap.Message)
	}
	env.Out.Printf("\nView the changes: git bug pr diff %s\n", colors.Cyan(pr.Id().Human()))

	diffs := diffCache{}
	for _, item := range snap.Timeline {
		switch item := item.(type) {
		case *pullrequest.CommentTimelineItem:
			env.Out.Printf("\n--- %s\n", entryTitle(item.Author, "commented", item.UnixTime))
			env.Out.Printf("%s\n", item.Message)
		case *pullrequest.ReviewTimelineItem:
			env.Out.Printf("\n--- %s\n", entryTitle(item.Author, coloredVerdict(item.Verdict), item.UnixTime))
			if item.Message != "" {
				env.Out.Printf("%s\n", item.Message)
			}
			for _, c := range item.Comments {
				printLineComment(env, snap, c, diffs)
			}
		case *pullrequest.HeadTimelineItem:
			env.Out.Printf("\n--- %s\n", entryTitle(item.Author, "set the head to "+short(item.Head), item.UnixTime))
		case *pullrequest.StatusTimelineItem:
			env.Out.Printf("\n--- %s\n", entryTitle(item.Author, "set the status to "+colors.Yellow(item.Status.String()), item.UnixTime))
		}
	}

	return nil
}

func printLineComment(env *execenv.Env, snap *pullrequest.Snapshot, c pullrequest.LineComment, diffs diffCache) {
	outdated := ""
	if c.Commit != snap.Head {
		outdated = " " + colors.Dim("[outdated]")
	}

	// A comment with a new line is read at the commit. One with only an old
	// line is on a removed line, which is read at the base.
	commit, path, line := c.Commit, c.Path, c.NewLine
	label := strconv.Itoa(c.NewLine)
	if c.NewLine == 0 {
		commit, line = c.Base, c.OldLine
		if c.OldPath != "" {
			path = c.OldPath
		}
		label = "-" + strconv.Itoa(c.OldLine)
	}
	env.Out.Printf("  %s%s\n", colors.Cyan(fmt.Sprintf("%s:%s @ %s", c.Path, label, short(c.Commit))), outdated)

	code, err := readLine(env.Ctx, env.Repo, commit, path, line)
	switch {
	case err == nil:
		style := lineStyle(env, c, diffs)
		if style == nil {
			env.Out.Printf("  > %s\n", code)
		} else {
			env.Out.Printf("  > %s\n", style(code))
		}
	case errors.Is(err, errCommitMissing):
		env.Out.Printf("  > %s\n", colors.Dim(fmt.Sprintf("code not available (commit %s not in this clone)", short(commit))))
	case errors.Is(err, errNoSuchLine):
		env.Out.Printf("  > %s\n", colors.Dim(fmt.Sprintf("code not available (no line %d in %s)", line, path)))
	default:
		env.Out.Printf("  > %s\n", colors.Dim(fmt.Sprintf("code not available (%s)", err)))
	}
	for _, l := range strings.Split(c.Message, "\n") {
		if l == "" {
			env.Out.Printf("\n")
		} else {
			env.Out.Printf("  %s\n", l)
		}
	}
}

// diffCache holds the diffs computed during one command run, by base and
// commit, so that many comments on the same commits compute each diff once.
type diffCache map[[2]repository.Hash][]repository.FileDiff

func (d diffCache) get(env *execenv.Env, base, commit repository.Hash) ([]repository.FileDiff, error) {
	key := [2]repository.Hash{base, commit}
	if files, ok := d[key]; ok {
		return files, nil
	}
	files, err := env.Repo.DiffCommits(env.Ctx, base, commit)
	if err != nil {
		return nil, err
	}
	d[key] = files
	return files, nil
}

// lineStyle returns the color of the code line a comment is about: green when
// the comment's diff adds the line, red when it removes it, and nil for an
// unchanged line or when the diff cannot be computed.
func lineStyle(env *execenv.Env, c pullrequest.LineComment, diffs diffCache) func(a ...interface{}) string {
	if color.NoColor {
		return nil
	}
	if c.NewLine == 0 {
		return colors.Red
	}
	files, err := diffs.get(env, c.Base, c.Commit)
	if err != nil {
		return nil
	}
	for _, f := range files {
		if f.Path != c.Path {
			continue
		}
		for _, h := range f.Hunks {
			for _, l := range h.Lines {
				if l.Type == repository.DiffLineAdded && l.NewLine == c.NewLine {
					return colors.Green
				}
			}
		}
	}
	return nil
}

func entryTitle(author identity.Interface, what string, t timestamp.Timestamp) string {
	return fmt.Sprintf("%s %s on %s", colors.Magenta(author.DisplayName()), what, t.Time().Format(dateLayout))
}

func verdictWord(v pullrequest.Verdict) string {
	switch v {
	case pullrequest.ApproveVerdict:
		return "approved"
	case pullrequest.RequestChangesVerdict:
		return "requested changes"
	default:
		return "reviewed"
	}
}

// coloredVerdict is verdictWord in green for an approval, red for a request
// for changes and yellow for a plain review.
func coloredVerdict(v pullrequest.Verdict) string {
	switch v {
	case pullrequest.ApproveVerdict:
		return colors.Green(verdictWord(v))
	case pullrequest.RequestChangesVerdict:
		return colors.Red(verdictWord(v))
	default:
		return colors.Yellow(verdictWord(v))
	}
}

// short returns the first 7 characters of a commit ID.
func short(h repository.Hash) string {
	s := h.String()
	if len(s) > 7 {
		return s[:7]
	}
	return s
}
