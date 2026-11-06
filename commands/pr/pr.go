package prcmd

import (
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/git-bug/git-bug/cache"
	"github.com/git-bug/git-bug/commands/execenv"
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

	cmd.AddCommand(newPrNewCommand(env))
	cmd.AddCommand(newPrShowCommand(env))

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
		return int(a.CreateLamportTime - b.CreateLamportTime)
	})

	for _, p := range excerpts {
		author, err := env.Backend.Identities().ResolveExcerpt(p.AuthorId)
		if err != nil {
			return err
		}

		env.Out.Printf("%s\t%s\t%s\t%s -> %s\t%s\n",
			p.Id().Human(),
			p.Status,
			strings.TrimSpace(p.Title),
			p.Source,
			p.Target,
			author.DisplayName(),
		)
	}
	return nil
}
