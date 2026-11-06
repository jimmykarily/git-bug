package prcmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/git-bug/git-bug/commands/execenv"
	"github.com/git-bug/git-bug/repository"
)

type prHeadOptions struct {
	commit string
}

func newPrHeadCommand(env *execenv.Env) *cobra.Command {
	options := prHeadOptions{}

	cmd := &cobra.Command{
		Use:     "head PR_ID",
		Short:   "Record a new head commit of a pull request",
		Long:    "Record a new head commit of a pull request. Without --commit, the head is the current commit of the source branch.",
		Args:    cobra.ExactArgs(1),
		PreRunE: execenv.LoadBackend(env, execenv.EnsureUser()),
		RunE: execenv.CloseBackend(env, func(cmd *cobra.Command, args []string) error {
			return runPrHead(env, options, args)
		}),
	}

	flags := cmd.Flags()
	flags.SortFlags = false

	flags.StringVar(&options.commit, "commit", "",
		"Full ID of the commit to record (default: the current commit of the source branch)")

	return cmd
}

func runPrHead(env *execenv.Env, opts prHeadOptions, args []string) error {
	pr, err := env.Backend.PullRequests().ResolvePrefix(env.Ctx, args[0])
	if err != nil {
		return err
	}
	snap := pr.Snapshot()

	var head repository.Hash
	if opts.commit != "" {
		head = repository.Hash(opts.commit)
		if !head.IsValid() {
			return fmt.Errorf("%q is not a full commit ID", opts.commit)
		}
		if _, err := env.Repo.ReadCommit(env.Ctx, head); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return fmt.Errorf("commit %s is not in this clone", opts.commit)
			}
			return err
		}
	} else {
		head, err = env.Backend.PullRequests().ResolveBranch(env.Ctx, snap.Source)
		if err != nil {
			return err
		}
	}

	if head == snap.Head {
		env.Out.Println("head unchanged")
		return nil
	}

	if _, err := pr.SetHead(env.Ctx, head); err != nil {
		return err
	}
	if err := pr.Commit(env.Ctx); err != nil {
		return err
	}

	env.Out.Printf("head set to %s (was %s)\n", short(head), short(snap.Head))
	return nil
}
