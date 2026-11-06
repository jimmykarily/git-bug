## git-bug pr review

Submit a review of a pull request

### Synopsis

Submit a review of a pull request. With neither --message nor --at, and unless --non-interactive is given, the editor opens on the diff of the pull request: write the summary above the diff, and each comment on the lines right under the diff line it is about.

```
git-bug pr review PR_ID [flags]
```

### Options

```
      --approve           Approve the changes
      --request-changes   Request changes
  -m, --message string    Provide the review summary from the command line
      --at stringArray    Comment on a line of the head commit, as PATH:LINE:MESSAGE. Can be repeated
      --non-interactive   Do not ask for user input
  -h, --help              help for review
```

### Options inherited from parent commands

```
      --no-colors   Do not color the output
```

### SEE ALSO

* [git-bug pr](git-bug_pr.md)	 - List pull requests

