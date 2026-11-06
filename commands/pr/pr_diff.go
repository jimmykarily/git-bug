package prcmd

import (
	"github.com/spf13/cobra"

	"github.com/git-bug/git-bug/commands/execenv"
)

func newPrDiffCommand(env *execenv.Env) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "diff PR_ID",
		Short:   "Display the diff of a pull request",
		Long:    "Display the changes from the merge base of the target branch and the head, to the head, in unified format.",
		Args:    cobra.ExactArgs(1),
		PreRunE: execenv.LoadBackend(env),
		RunE: execenv.CloseBackend(env, func(cmd *cobra.Command, args []string) error {
			return runPrDiff(env, args)
		}),
	}

	return cmd
}

func runPrDiff(env *execenv.Env, args []string) error {
	pr, err := env.Backend.PullRequests().ResolvePrefix(env.Ctx, args[0])
	if err != nil {
		return err
	}

	_, files, err := prDiff(env, pr.Snapshot())
	if err != nil {
		return err
	}

	writeUnifiedDiff(env.Out, files, "")
	return nil
}
