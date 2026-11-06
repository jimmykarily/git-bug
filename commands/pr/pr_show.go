package prcmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/git-bug/git-bug/commands/execenv"
	"github.com/git-bug/git-bug/entities/identity"
	"github.com/git-bug/git-bug/entities/pullrequest"
	"github.com/git-bug/git-bug/repository"
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

	target := snap.Target
	if hash, err := env.Backend.PullRequests().ResolveBranch(env.Ctx, snap.Target); err == nil {
		target = hash.String()
	}

	env.Out.Printf("%s [%s]\n", snap.Title, snap.Status)
	env.Out.Printf("%s -> %s, head %s\n", snap.Source, snap.Target, short(snap.Head))
	env.Out.Printf("opened by %s on %s\n", snap.Author.DisplayName(), snap.CreateTime.Format(dateLayout))
	env.Out.Printf("\n%s\n", snap.Message)
	env.Out.Printf("\nView the changes: git diff %s...%s\n", target, snap.Head)

	for _, item := range snap.Timeline {
		switch item := item.(type) {
		case *pullrequest.CommentTimelineItem:
			env.Out.Printf("\n--- %s\n", entryTitle(item.Author, "commented", item.UnixTime))
			env.Out.Printf("%s\n", item.Message)
		case *pullrequest.ReviewTimelineItem:
			env.Out.Printf("\n--- %s\n", entryTitle(item.Author, verdictWord(item.Verdict), item.UnixTime))
			if item.Message != "" {
				env.Out.Printf("%s\n", item.Message)
			}
			for _, c := range item.Comments {
				printLineComment(env, snap, c)
			}
		case *pullrequest.HeadTimelineItem:
			env.Out.Printf("\n--- %s\n", entryTitle(item.Author, "set the head to "+short(item.Head), item.UnixTime))
		case *pullrequest.StatusTimelineItem:
			env.Out.Printf("\n--- %s\n", entryTitle(item.Author, "set the status to "+item.Status.String(), item.UnixTime))
		}
	}

	return nil
}

func printLineComment(env *execenv.Env, snap *pullrequest.Snapshot, c pullrequest.LineComment) {
	outdated := ""
	if c.Commit != snap.Head {
		outdated = " [outdated]"
	}
	env.Out.Printf("  %s:%d @ %s%s\n", c.Path, c.Line, short(c.Commit), outdated)

	code, err := readLine(env.Ctx, env.Repo, c.Commit, c.Path, c.Line)
	switch {
	case err == nil:
		env.Out.Printf("  > %s\n", code)
	case errors.Is(err, errCommitMissing):
		env.Out.Printf("  > code not available (commit %s not in this clone)\n", short(c.Commit))
	case errors.Is(err, errNoSuchLine):
		env.Out.Printf("  > code not available (no line %d in %s)\n", c.Line, c.Path)
	default:
		env.Out.Printf("  > code not available (%s)\n", err)
	}
	env.Out.Printf("  %s\n", c.Message)
}

func entryTitle(author identity.Interface, what string, t timestamp.Timestamp) string {
	return fmt.Sprintf("%s %s on %s", author.DisplayName(), what, t.Time().Format(dateLayout))
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

// short returns the first 7 characters of a commit ID.
func short(h repository.Hash) string {
	s := h.String()
	if len(s) > 7 {
		return s[:7]
	}
	return s
}
