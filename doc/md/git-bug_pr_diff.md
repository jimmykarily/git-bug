## git-bug pr diff

Display the diff of a pull request

### Synopsis

Display the changes from the merge base of the target branch and the head, to the head, in unified format, with each line comment under the line it is about. Comments that are not on a line of this diff are listed after it.

```
git-bug pr diff PR_ID [flags]
```

### Options

```
      --no-comments   Print the plain diff, which git apply accepts
  -h, --help          help for diff
```

### Options inherited from parent commands

```
      --no-colors   Do not color the output
```

### SEE ALSO

* [git-bug pr](git-bug_pr.md)	 - List pull requests

