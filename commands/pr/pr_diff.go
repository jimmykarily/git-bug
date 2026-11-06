package prcmd

import (
	"github.com/spf13/cobra"

	"github.com/git-bug/git-bug/commands/execenv"
)

type prDiffOptions struct {
	noComments bool
}

func newPrDiffCommand(env *execenv.Env) *cobra.Command {
	options := prDiffOptions{}

	cmd := &cobra.Command{
		Use:     "diff PR_ID",
		Short:   "Display the diff of a pull request",
		Long:    "Display the changes from the merge base of the target branch and the head, to the head, in unified format, with each line comment under the line it is about. Comments that are not on a line of this diff are listed after it.",
		Args:    cobra.ExactArgs(1),
		PreRunE: execenv.LoadBackend(env),
		RunE: execenv.CloseBackend(env, func(cmd *cobra.Command, args []string) error {
			return runPrDiff(env, options, args)
		}),
	}

	flags := cmd.Flags()
	flags.SortFlags = false

	flags.BoolVar(&options.noComments, "no-comments", false,
		"Print the plain diff, which git apply accepts")

	return cmd
}

func runPrDiff(env *execenv.Env, opts prDiffOptions, args []string) error {
	pr, err := env.Backend.PullRequests().ResolvePrefix(env.Ctx, args[0])
	if err != nil {
		return err
	}

	snap := pr.Snapshot()
	base, files, err := prDiff(env, snap)
	if err != nil {
		return err
	}

	if opts.noComments {
		writeUnifiedDiff(env.Out, files, "", nil)
		return nil
	}
	printDiffWithComments(env, snap, base, files)
	return nil
}
