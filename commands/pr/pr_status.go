package prcmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/git-bug/git-bug/commands/execenv"
	"github.com/git-bug/git-bug/entities/pullrequest"
)

func newPrStatusCommand(env *execenv.Env) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "status PR_ID open|closed|merged",
		Short:   "Set the status of a pull request",
		Args:    cobra.ExactArgs(2),
		PreRunE: execenv.LoadBackend(env, execenv.EnsureUser()),
		RunE: execenv.CloseBackend(env, func(cmd *cobra.Command, args []string) error {
			return runPrStatus(env, args)
		}),
	}

	return cmd
}

func runPrStatus(env *execenv.Env, args []string) error {
	pr, err := env.Backend.PullRequests().ResolvePrefix(env.Ctx, args[0])
	if err != nil {
		return err
	}

	status, err := pullrequest.StatusFromString(args[1])
	if err != nil {
		return fmt.Errorf("unknown status %q, expected open, closed or merged", args[1])
	}

	if _, err := pr.SetStatus(env.Ctx, status); err != nil {
		return err
	}
	return pr.Commit(env.Ctx)
}
