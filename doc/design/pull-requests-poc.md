# Pull requests in git-bug: proof of concept

Status: proposal for a proof of concept (PoC), a small working version meant
to show the idea is practical. Nothing here is agreed with the git-bug
maintainers yet. The steps that built it, one commit each, are in
[pull-requests-poc-plan.md](pull-requests-poc-plan.md).

## Why

Today a project that uses git-bug still needs a forge (GitHub, GitLab, ...) for
code review. If the forge account is suspended, the review history and any
review in progress are out of reach until the forge restores access. git-bug
already solves this for issues: they live in the repository and travel with
`git bug push` and `git bug pull`. This PoC tests whether the same can be done
for pull requests (PRs) with little code, so the conversation with upstream can
start from something concrete.

## Prior discussion upstream

- [#373 "support for pull requests"](https://github.com/git-bug/git-bug/issues/373)
  is open. The original author, Michael Muré, supports the idea. In 2022 he
  wrote that the shared storage code in `entity/dag` makes a new kind of
  record "easy-ish" to add, and listed the design questions to answer first: the data
  model, the supported interactions, the CLI UX, and whether a code explorer is
  needed. He prefers a PR to be a separate kind of record instead of an extended bug.
- [#215](https://github.com/git-bug/git-bug/issues/215) asks for comments
  attached to a file and line, as a building block for #373.
- [#1328](https://github.com/git-bug/git-bug/issues/1328) asked about GitLab
  merge requests. It was closed in 2025 with the position that git-bug is an
  issue tracker and code review does not fit its planned future form.
- [#843 "project board"](https://github.com/git-bug/git-bug/pull/843) is open.
  It was meant to prepare the cache and the user interfaces for more kinds of
  records.

Michael Muré supports the idea in #373, while the 2025 closing of #1328 says
code review does not fit git-bug's plans. The PoC is kept small enough
to judge in one sitting, and it answers the design questions from #373 with
working code.

## Goals

- A new `pullrequest` record type, stored in git under `refs/pullrequests/`,
  built on `entity/dag` the same way bugs are.
- A CLI that lets a user open a PR, read it, comment, review with comments on
  specific lines, record new commits, and change its status.
- PRs travel with the existing `git bug push` and `git bug pull`. The only sync
  change is one entry in the list of record types that pull merges.
- A diff small enough for a maintainer to review, with tests in the style of the
  existing ones.

## Non-goals

- Bridges (GitHub, GitLab, ...). They come later. The data model should not
  block them, but no bridge code is written.
- Terminal UI, web UI and GraphQL API. The CLI is where git-bug features land
  first.
- Moving the code itself through git-bug. See "Transporting the code" below.
- Running the merge. `git merge` stays the user's job; the PR records the
  outcome.
- Editing comments or titles, labels, the query language, threads of replies
  under a review comment, and multi-line ranges.

## Things to avoid

- Any change to how bugs work or to how existing commands behave.
- Any change to the on-disk format of bugs.
- Any change to the on-disk format of identities. This cannot be fully kept:
  registering a new record type makes every identity version written
  afterwards record two more clocks, `pullrequests-create` and
  `pullrequests-edit`. A git-bug that does not know about PRs refuses to edit
  such an identity ("version has less lamport clocks than before"). See the questions for upstream.
- A diff that touches many unrelated files. New code goes in new files; edits
  to existing files are limited to registration points.

## Transporting the code

A PR points at commits. A reviewer on another machine needs those commits to
read the changes.

### Option A (this PoC): code travels with plain git

The author pushes the branch as usual with `git push`. The PR records the
source branch name, the target branch name and the commit ID of the head of
the source branch. Every review comment records the commit it was made on.

```
alice$ git push origin feature-x
alice$ git bug pr new --target trunk -t "Add X" -m "..."
alice$ git bug push

bob$   git fetch origin && git bug pull
bob$   git bug pr show 3f2a
```

Limitation: if the author deletes or force-pushes the branch, older commits a
review comment points at may disappear from other clones. The comment text
survives; the code it points at may not.

### Option B (later): git-bug pins the reviewed commits

Each time a PR records a head commit (on creation and on every head update),
git-bug also writes a ref that points at that commit, for example
`refs/pullrequests-heads/<pr id>/<commit id>`. `git bug push` and
`git bug pull` sync that namespace as well. This is similar to GitHub's hidden
`refs/pull/<n>/head`. The effects:

- Reviewed commits survive a deleted or rewritten branch, so old review
  comments always resolve to code.
- A reviewer needs only `git bug pull` to get both the PR and its code.
- A contributor can propose code to a remote where they are allowed to push
  git-bug refs but not branches. Whether remotes should allow that is a policy
  question for upstream.

What B needs on top of A:

- writing the refs when a head is recorded;
- adding the namespace to push and fetch;
- copying fetched refs from `refs/remotes/<remote>/...` into the local
  namespace, since these refs are plain pointers to commits and the existing
  merge code only handles git-bug records;
- removing the refs when a PR is removed.

The data model of A already records commit IDs, so B needs no format change.

## Data model

Package `entities/pullrequest`, modelled on `entities/bug`.

git-bug never stores the current state of a bug directly. It stores the list
of changes made to it ("operations": create, add a comment, change the status,
...) and replays them to get the current state. PRs work the same way.

- Typename `pullrequest`, namespace `pullrequests`, format version 1.
- Operations:
  - `CreateOp`: title, message, source branch, target branch, head commit.
  - `AddCommentOp`: a general comment (message).
  - `AddReviewOp`: one submitted review. It holds a verdict (`comment`,
    `approve` or `request-changes`), an optional summary message, and a list of
    line comments. A review is a single operation so that it is published all
    at once, like a submitted review on GitHub.
  - `SetHeadOp`: a new head commit, recorded when the source branch gets new
    or rewritten commits.
  - `SetStatusOp`: `open`, `closed` or `merged`. This is a status type of its
    own, because `entities/common.Status` has no merged state.
- A line comment points at one line of the PR's diff, which is the diff from
  the merge base of the target and the head, to the head. It holds:
  - `commit`: the head the comment was made on.
  - `base`: the merge base that diff was computed from.
  - `path`: the file path at the head, and `old_path`, set only when the file
    was renamed. The diff does not detect renames today, as for per-commit
    diffs, so a renamed file shows as a deletion plus an addition and
    `old_path` is only set once rename detection is added.
  - `old_line` and `new_line`: the line number in the file at `base` and at
    `commit`. A removed line has only `old_line`, an added line only
    `new_line`, an unchanged line both. Unused numbers are 0.
  - the message.

  These fields carry what GitHub, GitLab and Gitea need, so a bridge can map
  comments both ways. GitHub uses `side` (LEFT or RIGHT) and `line`, GitLab
  uses `old_line`, `new_line` and the base commit, Gitea uses `old_position`
  and `new_position`. All three accept comments only on lines in the diff.
  git-bug also accepts a comment on any line of a file at the head (with `--at`,
  see "CLI"); a bridge would have to export such a comment as a general
  comment that names the file and line.
- Branch names are stored as plain branch names (`feature-x`, `trunk`), the
  way a forge shows them. They are labels, and the head commit ID is what the
  PR actually points at. When the CLI needs a branch's commit, it tries the
  local branch first, then `refs/remotes/<remote>/<name>` for each remote, and
  fails with a clear message if it finds none or several that disagree. If the
  local branch exists and a remote has the same branch at a different commit,
  it also fails (`branch "x" is at 1234567 locally but at 89abcde on origin;
  push or update it first`), so a stale local branch cannot silently set the
  head. `pr new` still works when the source branch is on no remote, but
  prints a warning that the branch has to be pushed for reviewers to see the
  code.
- Who may do what: anyone may record a head or change the status, and any
  status can follow any other, including reopening a merged PR. This matches
  bugs, where anyone may close or reopen, and git-bug has no permission model
  to build on. Rules can come later as checks in the CLI; adding them to the
  replay logic would change the state of existing PRs.
- Validation, at the operation level: non-empty title and branch names, commit
  IDs that are well-formed hashes, non-empty file paths, line numbers that are
  not negative with at least one of them set, a known verdict and a known
  status. Whether a commit or file exists
  is checked by the CLI, because operations have no access to the repository.
- Authorship comes from git-bug identities (the users created with
  `git bug user new`). `entity/dag` records the identity of whoever made each
  operation, so the PR's author, and the author of every comment, review, head
  change and status change, need no extra field.
- Line comments always keep the commit they were made on. A rebase or new
  commits change the PR's head, and never the stored comments.
- Current state (the "snapshot", what replaying the operations produces): id,
  title, message, author, creation time, source and target
  branch, current head, status, and a timeline in operation order holding
  comments, reviews, head changes and status changes.

## Cache

git-bug keeps a local cache so that listing and opening records is fast. PRs
need the same three pieces bugs have, each modelled on its bug counterpart:

- `cache/pullrequest_cache.go`: a wrapper used to read and change one PR.
- `cache/pullrequest_excerpt.go`: a short summary of a PR, used for listing.
  It holds what the list command shows: id, title, status, author, source,
  target and times.
- `cache/pullrequest_subcache.go`: the cache for all PRs. It loads them and
  keeps the search index, which holds the title and all messages.

The PR cache is registered in `cache/repo_cache.go` (constructor, resolvers,
observer switch). Push and fetch loop over every registered cache, so they
carry PRs with no further change. The merge half of `git bug pull` uses its
own fixed list in `RepoCache.MergeAll` (`cache/repo_cache_common.go`), and PRs
are added there after bugs. Without that entry, pull would fetch PRs and never
show them, with no error. `TestPullRequestPushPull` guards this.

## CLI

Package `commands/pr`, mounted as `git bug pr`, modelled on `commands/bug`.
Commands take an ID or a unique ID prefix.

| Command | Effect |
| --- | --- |
| `git bug pr` | List PRs: short id, status, title, `source -> target`, author. |
| `git bug pr new --target BRANCH [--source BRANCH] [-t TITLE] [-m MSG]` | Open a PR. The source defaults to the checked-out branch. The head is the source branch's current commit. It refuses when there is nothing to review: the source and target are the same branch, or the source has no commit the target lacks (same commit, or already merged), and when the two branches share no history. |
| `git bug pr show ID` | Header (title, status, branches, head, author), the message, then the timeline in operation order (as for bugs), each entry with its author and date. Line comments print under their review as `path:line @ shortcommit` (`path:-line` for a removed line), followed by that line of code, read at the head for added and unchanged lines and at the base for removed lines, and marked `outdated` when the comment's commit is not the current head. If the commit is not in the local clone (for example after a force-push), the comment still prints, with `code not available (commit abc123 not in this clone)` in place of the code. The header also prints `View the changes: git bug pr diff <id>`. |
| `git bug pr diff ID [--no-comments]` | Print the PR's diff (merge base of target and head, to the head) in unified format, with each line comment printed under the line it is about. Comments that are not on a line of this diff (made on an older head, or with `--at` outside the diff) are listed after it. `--no-comments` prints the plain diff, which `git apply` accepts. |
| `git bug pr ... --no-colors` | Any `pr` command without colors. Colors are also off when the output is not a terminal or `NO_COLOR` is set. |
| `git bug pr comment ID -m MSG` | Add a general comment. |
| `git bug pr review ID [--approve \| --request-changes] [-m SUMMARY] [--at PATH:LINE:MESSAGE ...]` | Submit a review. With neither `-m` nor `--at`, in an interactive session, it opens the editor on the PR's diff (see below). `--at` can be repeated and comments on line LINE of the file at the head, which may be outside the diff. The CLI fails if the file or line does not exist at the head. |
| `git bug pr head ID [--commit HASH]` | Record a new head. Defaults to the source branch's current commit. `--commit` takes a full commit ID. Prints `head set to <new> (was <old>)`, or `head unchanged` and does nothing if the head has not moved. |
| `git bug pr status ID open\|closed\|merged` | Set the status. |

Reviewing in the editor: `pr review ID` writes the verdict line, then the
PR's diff with every diff line quoted with `> `. The reviewer writes comments
on unquoted lines right under the diff line they are about, and text above the
diff becomes the summary. On save, each comment is attached to the diff line
above it, on whichever side that line is on, and all of them are submitted as
one review. Blank lines inside a comment separate its paragraphs, and a comment
ends at the next quoted line. Quoted lines tell where each comment goes: a
`diff --git` or `+++ b/` line names the file, and an `@@` line names the hunk
and the line numbers. Files and hunks must stay in the order of the diff. The
reviewer may delete whole files, whole hunks, or the end of a hunk, and must
keep the file and hunk header and every quoted line above the line they comment
on. Other changes to the quoted lines are errors, as is a quoted line of code
with no file and hunk header above it. The parser cannot tell when the reviewer
deletes lines whose kept neighbours repeat them, for example the start of a
hunk, or one of two files with identical changes; the comment then lands on the
deleted copy. A review with the verdict `comment`, no summary and no comment is
aborted and nothing is written. An unchanged template opened with `--approve`
or `--request-changes` submits that verdict with no text.

```
# Reviewing 0c8c916 "Add X" (feature -> trunk, head db6beb8)
#
# The first line below is the verdict: comment, approve or request-changes.
# Text between the verdict and the diff is your summary.
# To comment on a line of the diff, add a new line right under it that does
# not start with ">", and write your comment there:
#
#   > +	greet()
#   Please add a test for greet.
#
# Do not edit the ">" lines, and keep all of them above a comment. You may
# delete whole files, whole hunks (from an "@@" line), or the end of a hunk.
# Lines starting with "#" are ignored.
comment

> diff --git a/main.go b/main.go
> --- a/main.go
> +++ b/main.go
> @@ -1,2 +1,3 @@
>  line one
>  line two
> +line three
This should be a constant.
```

Messages follow the existing conventions where cheap: `-m` for inline, `-F` for
a file, and the editor when neither is given and the session is interactive.

## Tests

Tests are written before the code, following the existing patterns:

- Operation tests per operation (serialization round trip and validation), as
  in `entities/bug/op_*_test.go`.
- Snapshot tests that apply a sequence of operations and check the result.
- A cache test that creates a PR, adds a comment, a review and a head change,
  closes the cache, reopens it and reads the same state back.
- A sync test: two repositories sharing a bare remote; one opens and reviews a
  PR and pushes, the other pulls and sees the same PR. It fails without the
  entry for PRs in the merge list (see "Cache").
- Command tests in the style of `commands/bug/*_test.go`.

## Answers to the design questions in #373

- What would the data model look like? See "Data model".
- What interactions are supported? Opening a PR, general comments, reviews
  with line comments, recording a new head, and changing the status. See
  "CLI".
- What would the CLI tooling look like? See "CLI".
- Does this need a code explorer first? Not for the CLI. The reviewer has the
  code locally, so git and their own editor are the explorer: `git switch` to
  the branch and read the files. `pr diff` prints the PR's changes, and
  `pr review` opens the same diff in the editor for line comments. A web UI
  would need one, and the code browsing support added in #1541 (file tree,
  file contents, history and per-commit diffs, exposed through the GraphQL
  API) covers most of it. This branch adds the missing piece, the diff between
  two commits, to the same interface.

## Questions for upstream

- Should a PR be a separate record type, as #373 suggests, or an extended bug?
- Is `refs/pullrequests/` the right namespace name?
- Is option A acceptable as a first step, with B as the follow-up?
- How should new record types be introduced without breaking identity edits in
  older versions? Registering PRs adds two clocks to every identity version
  written afterwards, and an older git-bug then refuses to edit that identity.
- What should a bridge map a GitHub or GitLab PR to, so the model above does
  not need a format change later?

## Later work

- Option B (pinning reviewed commits in git-bug refs).
- Bridges: import and export of GitHub and GitLab PRs.
- Terminal UI, web UI and GraphQL support.
- Moving outdated line comments to their new position, as GitHub does when the
  commented line is unchanged in the new head. This compares the file at the
  comment's commit with the file at the head. Comments already store their
  commit, so this needs no format change.
- Storing a copy of the diff lines with each line comment, as GitHub and
  Gitea return a `diff_hunk`. `pr show` could then print a comment's code even
  when its commit is no longer in the clone after a force-push.
- Draft reviews: collecting line comments over time and submitting them as one
  review.
- Replies under a line comment, multi-line ranges, editing, labels, linking a
  PR to the bugs it fixes.
- Detecting the merge automatically when the head commit becomes reachable
  from the target branch.
