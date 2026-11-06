package prcmd

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/git-bug/git-bug/cache"
	"github.com/git-bug/git-bug/commands/execenv"
	"github.com/git-bug/git-bug/commands/input"
	"github.com/git-bug/git-bug/entities/pullrequest"
	"github.com/git-bug/git-bug/repository"
	"github.com/git-bug/git-bug/util/text"
)

type prReviewOptions struct {
	approve        bool
	requestChanges bool
	message        string
	at             []string
	nonInteractive bool
}

func newPrReviewCommand(env *execenv.Env) *cobra.Command {
	options := prReviewOptions{}

	cmd := &cobra.Command{
		Use:     "review PR_ID",
		Short:   "Submit a review of a pull request",
		Long:    "Submit a review of a pull request. With neither --message nor --at, and unless --non-interactive is given, the editor opens on the diff of the pull request: write the summary above the diff, and each comment on the lines right under the diff line it is about.",
		Args:    cobra.ExactArgs(1),
		PreRunE: execenv.LoadBackend(env, execenv.EnsureUser()),
		RunE: execenv.CloseBackend(env, func(cmd *cobra.Command, args []string) error {
			return runPrReview(env, options, args)
		}),
	}

	flags := cmd.Flags()
	flags.SortFlags = false

	flags.BoolVar(&options.approve, "approve", false, "Approve the changes")
	flags.BoolVar(&options.requestChanges, "request-changes", false, "Request changes")
	flags.StringVarP(&options.message, "message", "m", "",
		"Provide the review summary from the command line")
	flags.StringArrayVar(&options.at, "at", nil,
		"Comment on a line of the head commit, as PATH:LINE:MESSAGE. Can be repeated")
	flags.BoolVar(&options.nonInteractive, "non-interactive", false, "Do not ask for user input")

	cmd.MarkFlagsMutuallyExclusive("approve", "request-changes")

	return cmd
}

func runPrReview(env *execenv.Env, opts prReviewOptions, args []string) error {
	pr, err := env.Backend.PullRequests().ResolvePrefix(env.Ctx, args[0])
	if err != nil {
		return err
	}
	head := pr.Snapshot().Head

	verdict := pullrequest.CommentVerdict
	switch {
	case opts.approve:
		verdict = pullrequest.ApproveVerdict
	case opts.requestChanges:
		verdict = pullrequest.RequestChangesVerdict
	}

	if !opts.nonInteractive && opts.message == "" && len(opts.at) == 0 {
		return runPrReviewEditor(env, pr, verdict)
	}

	var base repository.Hash
	var files []repository.FileDiff
	if len(opts.at) > 0 {
		base, files, err = prDiff(env, pr.Snapshot())
		if err != nil {
			return fmt.Errorf("computing the diff: %w", err)
		}
	}

	comments := make([]pullrequest.LineComment, 0, len(opts.at))
	for _, at := range opts.at {
		c, err := parseAt(at)
		if err != nil {
			return err
		}
		c.Commit = head
		c.Base = base

		_, err = readLine(env.Ctx, env.Repo, head, c.Path, c.NewLine)
		switch {
		case errors.Is(err, errNoSuchLine):
			return fmt.Errorf("--at %q: no line %d in %s at the head %s", at, c.NewLine, c.Path, short(head))
		case err != nil:
			return fmt.Errorf("--at %q: %w", at, err)
		}
		var oldPath *string
		c.OldLine, oldPath = unchangedLine(files, c.Path, c.NewLine)
		if oldPath != nil {
			c.OldPath = *oldPath
		}
		comments = append(comments, c)
	}

	if _, err := pr.AddReview(env.Ctx, verdict, text.Cleanup(opts.message), comments); err != nil {
		return err
	}
	if err := pr.Commit(env.Ctx); err != nil {
		return err
	}

	env.Out.Printf("review added to %s\n", pr.Id().Human())
	return nil
}

// runPrReviewEditor opens the editor on the diff of the pull request and
// submits the review written there.
func runPrReviewEditor(env *execenv.Env, pr *cache.PullRequestCache, verdict pullrequest.Verdict) error {
	snap := pr.Snapshot()
	base, files, err := prDiff(env, snap)
	if err != nil {
		return fmt.Errorf("computing the diff: %w", err)
	}

	template := reviewTemplate(pr.Id().Human(), snap.Title, snap.Source, snap.Target, snap.Head, verdict, files)
	raw, err := input.LaunchEditorWithTemplate(env.Backend, reviewFilename, template)
	if err != nil {
		return err
	}

	name, summary, targets, err := parseReview(raw, files)
	if err != nil {
		return err
	}
	if raw == template || name == "" || (name == pullrequest.CommentVerdict.String() && summary == "" && len(targets) == 0) {
		env.Out.Println("Empty review, aborting.")
		return nil
	}
	verdict, _ = verdictByName(name)

	comments := make([]pullrequest.LineComment, 0, len(targets))
	for _, t := range targets {
		comments = append(comments, pullrequest.LineComment{
			Commit:  snap.Head,
			Base:    base,
			Path:    t.Path,
			OldPath: t.OldPath,
			OldLine: t.OldLine,
			NewLine: t.NewLine,
			Message: t.Message,
		})
	}

	if _, err := pr.AddReview(env.Ctx, verdict, summary, comments); err != nil {
		return err
	}
	if err := pr.Commit(env.Ctx); err != nil {
		return err
	}

	env.Out.Printf("review added to %s\n", pr.Id().Human())
	return nil
}

// parseAt splits PATH:LINE:MESSAGE at the first two colons, so that the
// message may contain colons.
func parseAt(at string) (pullrequest.LineComment, error) {
	parts := strings.SplitN(at, ":", 3)
	if len(parts) != 3 {
		return pullrequest.LineComment{}, fmt.Errorf("--at %q: expected PATH:LINE:MESSAGE", at)
	}
	line, err := strconv.Atoi(parts[1])
	if err != nil || line < 1 {
		return pullrequest.LineComment{}, fmt.Errorf("--at %q: the line must be a number of at least 1", at)
	}
	if text.Empty(parts[0]) {
		return pullrequest.LineComment{}, fmt.Errorf("--at %q: the path is empty", at)
	}
	if text.Empty(parts[2]) {
		return pullrequest.LineComment{}, fmt.Errorf("--at %q: the message is empty", at)
	}
	return pullrequest.LineComment{Path: parts[0], NewLine: line, Message: parts[2]}, nil
}
