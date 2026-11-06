package prcmd

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/git-bug/git-bug/commands/execenv"
	"github.com/git-bug/git-bug/entities/pullrequest"
	"github.com/git-bug/git-bug/util/text"
)

type prReviewOptions struct {
	approve        bool
	requestChanges bool
	message        string
	at             []string
}

func newPrReviewCommand(env *execenv.Env) *cobra.Command {
	options := prReviewOptions{}

	cmd := &cobra.Command{
		Use:     "review PR_ID",
		Short:   "Submit a review of a pull request",
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

	cmd.MarkFlagsMutuallyExclusive("approve", "request-changes")

	return cmd
}

func runPrReview(env *execenv.Env, opts prReviewOptions, args []string) error {
	pr, err := env.Backend.PullRequests().ResolvePrefix(env.Ctx, args[0])
	if err != nil {
		return err
	}
	head := pr.Snapshot().Head

	comments := make([]pullrequest.LineComment, 0, len(opts.at))
	for _, at := range opts.at {
		c, err := parseAt(at)
		if err != nil {
			return err
		}
		c.Commit = head

		_, err = readLine(env.Ctx, env.Repo, head, c.Path, c.Line)
		switch {
		case errors.Is(err, errNoSuchLine):
			return fmt.Errorf("--at %q: no line %d in %s at the head %s", at, c.Line, c.Path, short(head))
		case err != nil:
			return fmt.Errorf("--at %q: %w", at, err)
		}
		comments = append(comments, c)
	}

	verdict := pullrequest.CommentVerdict
	switch {
	case opts.approve:
		verdict = pullrequest.ApproveVerdict
	case opts.requestChanges:
		verdict = pullrequest.RequestChangesVerdict
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
	return pullrequest.LineComment{Path: parts[0], Line: line, Message: parts[2]}, nil
}
