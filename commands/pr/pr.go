package prcmd

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/git-bug/git-bug/cache"
	"github.com/git-bug/git-bug/commands/execenv"
	"github.com/git-bug/git-bug/util/colors"
)

func NewPullRequestCommand(env *execenv.Env) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pr",
		Short:   "List pull requests",
		Long:    "Display a summary of each pull request, oldest first.",
		Args:    cobra.NoArgs,
		PreRunE: execenv.LoadBackend(env),
		RunE: execenv.CloseBackend(env, func(cmd *cobra.Command, args []string) error {
			return runPr(env)
		}),
	}

	// The flag turns colors off as soon as it is parsed, for this command and
	// every subcommand, without relying on a PersistentPreRun that a
	// subcommand's own PreRunE would shadow.
	cmd.PersistentFlags().BoolFunc("no-colors", "Do not color the output", func(value string) error {
		off, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		if off {
			color.NoColor = true
		}
		return nil
	})
	cmd.PersistentFlags().Lookup("no-colors").DefValue = "false"

	cmd.AddCommand(newPrNewCommand(env))
	cmd.AddCommand(newPrShowCommand(env))
	cmd.AddCommand(newPrDiffCommand(env))
	cmd.AddCommand(newPrCommentCommand(env))
	cmd.AddCommand(newPrReviewCommand(env))
	cmd.AddCommand(newPrHeadCommand(env))
	cmd.AddCommand(newPrStatusCommand(env))

	return cmd
}

func runPr(env *execenv.Env) error {
	ids := env.Backend.PullRequests().AllIds()

	excerpts := make([]*cache.PullRequestExcerpt, len(ids))
	for i, id := range ids {
		excerpt, err := env.Backend.PullRequests().ResolveExcerpt(id)
		if err != nil {
			return err
		}
		excerpts[i] = excerpt
	}

	slices.SortStableFunc(excerpts, func(a, b *cache.PullRequestExcerpt) int {
		return cmp.Compare(a.CreateLamportTime, b.CreateLamportTime)
	})

	for _, p := range excerpts {
		author, err := env.Backend.Identities().ResolveExcerpt(p.AuthorId)
		if err != nil {
			return err
		}

		env.Out.Printf("%s\t%s\t%s\t%s -> %s\t%s\n",
			colors.Cyan(p.Id().Human()),
			colors.Yellow(p.Status),
			strings.TrimSpace(p.Title),
			p.Source,
			p.Target,
			colors.Magenta(author.DisplayName()),
		)
	}
	return nil
}
