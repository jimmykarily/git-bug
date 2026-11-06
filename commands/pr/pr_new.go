package prcmd

import (
	"errors"
	"fmt"
	"slices"

	"github.com/spf13/cobra"

	buginput "github.com/git-bug/git-bug/commands/bug/input"
	"github.com/git-bug/git-bug/commands/execenv"
	"github.com/git-bug/git-bug/repository"
	"github.com/git-bug/git-bug/util/text"
)

type prNewOptions struct {
	source         string
	target         string
	title          string
	message        string
	messageFile    string
	nonInteractive bool
}

func newPrNewCommand(env *execenv.Env) *cobra.Command {
	options := prNewOptions{}

	cmd := &cobra.Command{
		Use:     "new",
		Short:   "Open a new pull request",
		PreRunE: execenv.LoadBackend(env, execenv.EnsureUser()),
		RunE: execenv.CloseBackend(env, func(cmd *cobra.Command, args []string) error {
			return runPrNew(env, options)
		}),
	}

	flags := cmd.Flags()
	flags.SortFlags = false

	flags.StringVar(&options.source, "source", "",
		"Branch holding the changes (default: the checked-out branch)")
	flags.StringVar(&options.target, "target", "",
		"Branch the changes are meant for")
	_ = cmd.MarkFlagRequired("target")
	flags.StringVarP(&options.title, "title", "t", "",
		"Provide a title to describe the pull request")
	flags.StringVarP(&options.message, "message", "m", "",
		"Provide a message to describe the pull request")
	flags.StringVarP(&options.messageFile, "file", "F", "",
		"Take the title and message from the given file (the first line is the title, unless --title is given). Use - to read from the standard input")
	flags.BoolVar(&options.nonInteractive, "non-interactive", false, "Do not ask for user input")

	cmd.MarkFlagsMutuallyExclusive("message", "file")

	return cmd
}

func runPrNew(env *execenv.Env, opts prNewOptions) error {
	var err error

	if opts.source == "" {
		opts.source, err = checkedOutBranch(env)
		if err != nil {
			return err
		}
	}

	head, err := env.Backend.PullRequests().ResolveBranch(env.Ctx, opts.source)
	if err != nil {
		return err
	}

	err = checkSomethingToReview(env, opts.source, opts.target, head)
	if err != nil {
		return err
	}

	if opts.messageFile != "" {
		opts.title, opts.message, err = buginput.BugCreateFileInput(opts.messageFile, opts.title)
		if err != nil {
			return err
		}
	}

	if !opts.nonInteractive && opts.messageFile == "" && (opts.message == "" || opts.title == "") {
		opts.title, opts.message, err = buginput.BugCreateEditorInput(env.Backend, opts.title, opts.message)

		if err == buginput.ErrEmptyTitle {
			env.Out.Println("Empty title, aborting.")
			return nil
		}
		if err != nil {
			return err
		}
	}

	pr, _, err := env.Backend.PullRequests().New(
		env.Ctx,
		text.CleanupOneLine(opts.title),
		text.Cleanup(opts.message),
		opts.source,
		opts.target,
		head,
	)
	if err != nil {
		return err
	}
	err = pr.Commit(env.Ctx)
	if err != nil {
		return err
	}

	env.Out.Printf("%s created\n", pr.Id().Human())

	onRemote, err := env.Backend.PullRequests().BranchOnRemote(env.Ctx, opts.source)
	if err != nil {
		return err
	}
	if !onRemote {
		env.Err.Printf("warning: branch %q is not on any remote yet; push it so reviewers can see the code\n", opts.source)
	}

	return nil
}

// checkSomethingToReview fails when the source holds no commit that the target
// lacks: the same branch, the same commit under another name, or a source
// already merged into the target. Finding the target's ancestors walks its
// whole history, which is slow on very large repositories.
func checkSomethingToReview(env *execenv.Env, source, target string, head repository.Hash) error {
	if source == target {
		return fmt.Errorf("source and target are the same branch %q", source)
	}

	targetCommit, err := env.Backend.PullRequests().ResolveBranch(env.Ctx, target)
	if err != nil {
		return err
	}

	ancestors, err := env.Repo.ListCommits(env.Ctx, targetCommit)
	if err != nil {
		return err
	}
	if slices.Contains(ancestors, head) {
		return fmt.Errorf("nothing to review: %q has no commits that %q does not have", source, target)
	}

	return nil
}

// checkedOutBranch returns the name of the branch HEAD points to.
func checkedOutBranch(env *execenv.Env) (string, error) {
	ref, err := env.Backend.BrowseRepo().Head(env.Ctx)
	if errors.Is(err, repository.ErrNotFound) || (err == nil && ref.Type != repository.GitRefTypeBranch) {
		return "", errors.New("not on a branch, use --source")
	}
	if err != nil {
		return "", err
	}
	return ref.ShortName, nil
}
