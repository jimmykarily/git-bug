## git-bug pr new

Open a new pull request

```
git-bug pr new [flags]
```

### Options

```
      --source string     Branch holding the changes (default: the checked-out branch)
      --target string     Branch the changes are meant for
  -t, --title string      Provide a title to describe the pull request
  -m, --message string    Provide a message to describe the pull request
  -F, --file string       Take the title and message from the given file (the first line is the title, unless --title is given). Use - to read from the standard input
      --non-interactive   Do not ask for user input
  -h, --help              help for new
```

### Options inherited from parent commands

```
      --no-colors   Do not color the output
```

### SEE ALSO

* [git-bug pr](git-bug_pr.md)	 - List pull requests

