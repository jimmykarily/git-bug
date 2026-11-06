package prcmd

import (
	"github.com/spf13/cobra"

	buginput "github.com/git-bug/git-bug/commands/bug/input"
	"github.com/git-bug/git-bug/commands/execenv"
	"github.com/git-bug/git-bug/util/text"
)

type prCommentOptions struct {
	messageFile    string
	message        string
	nonInteractive bool
}

func newPrCommentCommand(env *execenv.Env) *cobra.Command {
	options := prCommentOptions{}

	cmd := &cobra.Command{
		Use:     "comment PR_ID",
		Short:   "Add a comment to a pull request",
		Args:    cobra.ExactArgs(1),
		PreRunE: execenv.LoadBackend(env, execenv.EnsureUser()),
		RunE: execenv.CloseBackend(env, func(cmd *cobra.Command, args []string) error {
			return runPrComment(env, options, args)
		}),
	}

	flags := cmd.Flags()
	flags.SortFlags = false

	flags.StringVarP(&options.messageFile, "file", "F", "",
		"Take the message from the given file. Use - to read the message from the standard input")
	flags.StringVarP(&options.message, "message", "m", "",
		"Provide the new message from the command line")
	flags.BoolVar(&options.nonInteractive, "non-interactive", false, "Do not ask for user input")

	cmd.MarkFlagsMutuallyExclusive("message", "file")

	return cmd
}

func runPrComment(env *execenv.Env, opts prCommentOptions, args []string) error {
	pr, err := env.Backend.PullRequests().ResolvePrefix(env.Ctx, args[0])
	if err != nil {
		return err
	}

	if opts.messageFile != "" {
		opts.message, err = buginput.BugCommentFileInput(opts.messageFile)
		if err != nil {
			return err
		}
	}

	if opts.messageFile == "" && opts.message == "" {
		if opts.nonInteractive {
			env.Err.Println("No message given. Use -m or -F option to specify a message. Aborting.")
			return nil
		}
		opts.message, err = buginput.BugCommentEditorInput(env.Backend, "")
		if err == buginput.ErrEmptyMessage {
			env.Err.Println("Empty message, aborting.")
			return nil
		}
		if err != nil {
			return err
		}
	}

	if _, err := pr.AddComment(env.Ctx, text.Cleanup(opts.message)); err != nil {
		return err
	}

	return pr.Commit(env.Ctx)
}
