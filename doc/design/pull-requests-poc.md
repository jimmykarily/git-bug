# Pull requests in git-bug: proof of concept

Status: proposal and work plan for a proof of concept (PoC), a small working
version meant to show the idea is practical. Nothing here is agreed with the
git-bug maintainers yet.

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
  PR and pushes, the other pulls and sees the same PR. This proves the claim
  that push and pull need no change.
- Command tests in the style of `commands/bug/*_test.go`.

## Work plan

Each step below is one commit. Tests are written first and fail before the code
exists. Every step ends with `go test ./...` and `go vet ./...` passing.
Commits use the repository's existing message style (`area: summary`).

Step 1 is this document.

### Rules for every step

- New code follows the matching bug code closely (same file layout, same naming
  patterns, same test helpers). When in doubt, copy the bug pattern.
- No change to existing behaviour or existing test expectations.
- Changes to existing files are limited to the ones each step names.
- Comments in code describe what the code does now, in plain words.

### Inputs that are easy to get wrong

Each of these has a test in the step that owns the code:

- A review line comment on a file or line that does not exist at the head:
  `pr review` fails before writing anything (step 6).
- A line comment whose commit is missing from the clone: `pr show` still
  prints the comment, with the "code not available" note (step 5).
- A branch that exists only as `refs/remotes/<remote>/<name>`: it is found
  (step 4).
- Pulling PRs into a second clone: they show up after `git bug pull` (step 4).
- A short or malformed commit ID in `--commit`: rejected with a clear message
  (step 6).

### Step 2: the PR record type and its creation

Files: create `entities/pullrequest/pullrequest.go`, `status.go`, `verdict.go`,
`operation.go`, `op_create.go`, `snapshot.go`, `timeline.go`, `actions.go`, and
their `_test.go` files.

Names later steps rely on:

- `const Typename = "pullrequest"`, `const Namespace = "pullrequests"`, format
  version `1`.
- `type Status int` with `OpenStatus = 1`, `ClosedStatus`, `MergedStatus`;
  `String()` gives `open`, `closed`, `merged`; `Validate() error`;
  `StatusFromString(string) (Status, error)`.
- `type Verdict int` with `CommentVerdict = 1`, `ApproveVerdict`,
  `RequestChangesVerdict`; `String()` gives `comment`, `approve`,
  `request-changes`; `Validate() error`.
- Operation types, in this order from 1: `CreateOp`, `AddCommentOp`,
  `AddReviewOp`, `SetHeadOp`, `SetStatusOp`. Unknown types go through
  `dag.NewUnknownOp`, as in `entities/bug/operation.go`.
- `type Operation = dag.OperationWithApply[*Snapshot]`,
  `type Mutable = dag.Mutable[*Snapshot, Operation]`.
- `type PullRequest struct{ *dag.Entity }` with `New()`, `Read`,
  `ReadWithResolver`, `Validate` (first operation is the only `CreateOp`),
  `Append`, `CommitOperations`, `Operations`, `Compile` (status starts as
  `OpenStatus`), `FirstOp`.
- `actions.go`: `MergeAll`, `Remove`, `RemoveAll`, `EnsureClocks`, each a thin
  call into `dag` like `entities/bug/bug_actions.go`.
- `CreateOperation` with JSON fields `title`, `message`, `source`, `target`,
  `head` (a `repository.Hash`). `NewCreateOp(author identity.Interface,
  unixTime int64, title, message, source, target string, head repository.Hash)`
  and `Create(...same...) (*PullRequest, *CreateOperation, error)`. Validation:
  title as for bugs, message printable, source and target non-empty single
  lines, head `IsValid()`.
- `Snapshot` fields: `Title`, `Message`, `Author`, `CreateTime`, `Source`,
  `Target`, `Head`, `Status`, `Timeline []TimelineItem`, `Operations`. Methods
  `Id()`, `AllOperations()`, `AppendOperation()`, `Clone()` (the cache needs
  it; cloning the slices is enough because timeline items are never edited),
  `EditTime()`.
- `type TimelineItem interface{ CombinedId() entity.CombinedId }`. Creation adds
  no timeline item; the title and message are shown in the header.

Tests:

- `TestCreateSerialize`: `dagtest.SerializeRoundTripTest` on `NewCreateOp`.
- `TestCreateValidate`: empty title, empty source, empty target, and the head
  `"abc"` each fail; a full valid op passes.
- `TestStatusFromString` and `TestVerdictString`: every value round-trips;
  `"nope"` fails.
- `TestPullRequestCommitLoad`: create a PR with `Create`, commit it to a
  `repotest.NewGoGitRepo`, read it back with `Read`, and compare the snapshot's
  title, source, target, head, status `OpenStatus` and author id.

### Step 3: the other operations

Files: create `entities/pullrequest/op_add_comment.go`, `op_add_review.go`,
`op_set_head.go`, `op_set_status.go` and their tests; add the new cases to
`operation.go`.

- `AddCommentOperation{Message}`, timeline item `CommentTimelineItem{Author,
  UnixTime, Message}`. `AddComment(pr Mutable, author, unixTime, message)
  (*AddCommentOperation, error)`. An empty message is invalid.
- `type LineComment struct{ Commit repository.Hash; Path string; Line int;
  Message string }` with JSON names `commit`, `path`, `line`, `message`, and
  `Validate() error` (valid commit, non-empty path, line at least 1, non-empty
  printable message).
- `AddReviewOperation{Verdict, Message, Comments []LineComment}`, timeline item
  `ReviewTimelineItem{Author, UnixTime, Verdict, Message, Comments}`.
  `AddReview(pr Mutable, author, unixTime, verdict, message, comments)
  (*AddReviewOperation, error)`. A `comment` verdict with no message and no
  line comments is invalid.
- `SetHeadOperation{Head}`, timeline item `HeadTimelineItem{Author, UnixTime,
  Head}`; applying it sets `Snapshot.Head`. `SetHead(pr Mutable, author,
  unixTime, head)`.
- `SetStatusOperation{Status}`, timeline item `StatusTimelineItem{Author,
  UnixTime, Status}`; applying it sets `Snapshot.Status`, any status to any
  status. `SetStatus(pr Mutable, author, unixTime, status)`.
- All `UnixTime` fields are `timestamp.Timestamp`, as in bugs.

Tests:

- A serialize round-trip test per operation.
- `TestAddReviewValidate`: an empty review, a line comment with line 0, an
  empty path, or a bad commit each fail; an approval with no message passes.
- `TestSnapshotTimeline`: apply create, comment, review, head change, and the
  statuses merged then open; check the timeline has five entries of the right
  types in order, `Head` is the new commit, the status is `OpenStatus`, and
  the review's line comment still holds the old commit.

### Step 4: the cache

Files: create `cache/pullrequest_cache.go`, `cache/pullrequest_excerpt.go`,
`cache/pullrequest_subcache.go`, `cache/pullrequest_test.go`. Modify
`cache/repo_cache.go` (field, constructor, `PullRequests()` accessor, resolvers,
observer switch) and `cache/repo_cache_common.go` (`MergeAll` list becomes
`{c.identities}, {c.bugs, c.pullRequests}`, since PRs depend only on
identities).

- `PullRequestCache` wraps
  `CachedEntityBase[*pullrequest.Snapshot, pullrequest.Operation]` with
  `AddComment(ctx, message)`, `AddReview(ctx, verdict, message, comments)`,
  `SetHead(ctx, head)`, `SetStatus(ctx, status)`. Each uses the current user
  identity and `time.Now()`, and stages the operation; the caller commits.
- `PullRequestExcerpt`: id, create and edit lamport times, create and edit unix
  times, `AuthorId`, `Title`, `Status`, `Source`, `Target`.
- `RepoCachePullRequest` embeds the generic `SubCache`, built like
  `NewRepoCacheBug`. The index data is the title, the message, and every
  comment and review message. Methods:
  - `New(ctx, title, message, source, target string, head repository.Hash)
    (*PullRequestCache, *pullrequest.CreateOperation, error)`, staged like
    bug creation.
  - `ResolveBranch(ctx, name string) (repository.Hash, error)`: the local
    `refs/heads/<name>` if present and equal to every remote's
    `refs/remotes/<remote>/<name>`; otherwise only the remotes' refs. It fails
    with `branch "<name>" is at <short local> locally but at <short remote> on
    <remote>; push or update it first` when the local branch differs from a
    remote (the first in sorted order is named), with
    `branch "<name>" not found locally or on any remote` when none exist, and
    `branch "<name>" points to different commits on remotes <a>, <b>` when
    remote-only branches disagree.
  - `BranchOnRemote(ctx, name string) (bool, error)`: whether any remote
    tracking ref exists for the branch. `pr new` uses it for its warning.

Tests:

- `TestPullRequestCacheReload`: create a PR, add a comment, a review, a head
  change and a status change, commit, close the cache, open a new one on the
  same repository, and read the same snapshot back (title, head, status,
  timeline length 4).
- `TestPullRequestPushPull`: with `repotest.SetupGoGitReposAndRemote`, A
  creates and reviews a PR and pushes; B pulls and sees one PR with the same
  title and review; B comments and pushes; A pulls and sees the comment. This
  test fails if `MergeAll` is not updated.
- `TestResolveBranch`: a local branch; a local branch that
  differs from a remote (an error) and one that agrees; a branch only under
  `refs/remotes/origin/`; a missing branch; two remotes that disagree.
  `TestBranchOnRemote` covers the remote check.

### Step 5: `git bug pr`, `pr new` and `pr show`

Files: create `commands/pr/pr.go` (the `pr` command and the list),
`pr_new.go`, `pr_show.go`, `code.go`, `testenv_test.go`, and the tests. Modify
`commands/root.go` to add the `pr` command to the entity group.

- The package is `prcmd`, with `NewPullRequestCommand(env *execenv.Env)
  *cobra.Command`. IDs are resolved with
  `env.Backend.PullRequests().ResolvePrefix`.
- The list prints one line per PR, oldest first:
  `<short id>\t<status>\t<title>\t<source> -> <target>\t<author name>`.
- `pr new`: flags `--source`, `--target` (required), `-t/--title`,
  `-m/--message`, `-F/--file`, `--non-interactive`. Title and message input
  reuse `commands/bug/input` (`BugCreateEditorInput`, `BugCreateFileInput`).
  The source defaults to the checked-out branch from `BrowseRepo().Head()`,
  and fails with `not on a branch, use --source` otherwise. The head comes
  from `ResolveBranch(source)`. It prints `<short id> created`.
- `code.go`: `readLine(ctx, repo repository.Repo, commit
  repository.Hash, path string, line int) (string, error)`, which reads the
  file with `BlobAtPath`. It returns `errCommitMissing` when the commit is not
  in the clone (checked first with `ReadCommit`), and `errNoSuchLine` when the
  file or line does not exist.
- `pr show` prints:

  ```
  <title> [<status>]
  <source> -> <target>, head <short head>
  opened by <author> on <date>

  <message>

  View the changes: git diff <target commit>...<head>

  --- <author> commented on <date>
  <message>

  --- <author> approved on <date>
  <summary>
    <path>:<line> @ <short commit> [outdated]
    > <code line, or: code not available (commit <short> not in this clone)>
    <message>

  --- <author> set the head to <short head> on <date>
  --- <author> set the status to merged on <date>
  ```

  Verdict words are `reviewed`, `approved`, `requested changes`. `[outdated]`
  appears only when the comment's commit differs from the current head. The
  date format is `Mon Jan 2 15:04:05 2006 -0700`. Short commits are 7
  characters. If the target branch cannot be resolved, the diff line uses the
  branch name.

Test helper (`testenv_test.go`): `newTestEnvWithBranches(t)` builds on
`commands/bug/testenv.NewTestEnvAndUser`. It commits a `trunk` branch with
`main.go` holding `line one\nline two\n`, and a `feature` branch on top that
adds `line three\n`. It uses `env.Repo` (`StoreData`, `StoreTree`,
`StoreCommit`) and `repotest.GoGitRepo.SetBranch`, and returns the env and
both commit IDs.

Tests:

- `TestPrNew`: with `--source feature --target trunk`, the PR exists with the
  feature commit as head and the output matches `^[0-9a-f]{7} created\n$`;
  with an unknown source it fails with the not-found message.
- `TestPrList`: two PRs give two lines in creation order.
- `TestPrShow`: a PR with a comment and a review holding one line comment on
  `main.go:3` prints the header, the diff line, the review entry and
  `> line three`.
- `TestPrShowOutdatedAndMissing`: after `SetHead` to a new commit, the comment
  is marked `[outdated]` and still shows its line. A line comment pointing at a
  commit that does not exist in the repository prints the "code not
  available" note, and the command does not fail.

### Step 6: `pr comment`, `pr review`, `pr head`, `pr status`

Files: create `commands/pr/pr_comment.go`, `pr_review.go`, `pr_head.go`,
`pr_status.go` and their tests.

- `pr comment ID`: `-m`, `-F`, editor otherwise, reusing
  `BugCommentEditorInput` and `BugCommentFileInput`.
- `pr review ID`: `--approve` and `--request-changes` (mutually exclusive),
  `-m` summary, and a repeatable `--at PATH:LINE:MESSAGE`. The text is split at
  the first two colons, so the message may contain colons. Every line comment
  is checked with `readLine` at the PR's current head before anything is
  written. A failure names the bad `--at` value. It prints
  `review added to <short id>`.
- `pr head ID`: `--commit` takes a full commit ID that must exist in the clone
  (checked with `ReadCommit`). Without it, the head comes from
  `ResolveBranch(source)`. If the head has not moved, it prints
  `head unchanged` and writes nothing.
- `pr status ID STATUS`: `STATUS` is one of `open`, `closed`, `merged`, parsed
  with `StatusFromString`.

Tests:

- `TestPrComment`: the comment appears in the timeline.
- `TestPrReview`: an approval with two `--at` values, one with a colon in its
  message, stores two line comments on the head commit. A `--at` pointing at
  line 9 of `main.go`, or at a missing file, fails, and the timeline is
  unchanged.
- `TestPrHead`: moving the `feature` branch and running `pr head` records the
  new head; running it again prints `head unchanged`; `--commit abc` fails.
- `TestPrStatus`: `merged` then `open` both apply; `nope` fails.

### Step 7: generated docs and the feature matrix

Run `go run doc/generate.go` and `go run misc/completion/generate.go` to
refresh `doc/man`, `doc/md` and the shell completions. In the "Pull-request"
row of `doc/feature-matrix.md`, set the Core and CLI columns to the partial
mark used for "Board". Check with `git status` that only generated docs and that row
changed.

### Step 8: the diff between two commits

Files: modify `repository/browse.go` (the `RepoBrowse` interface),
`repository/gogit.go`, `repository/mock_repo.go`, and
`repository/repotest/suite.go` (`RepoBrowseTest`, which runs against both
implementations).

- `MergeBase(ctx, a, b Hash) (Hash, error)`: the best common ancestor of two
  commits. Returns `ErrNotFound` if either commit is missing or they share no
  ancestor. The go-git version uses `Commit.MergeBase`.
- `DiffCommits(ctx, from, to Hash) ([]FileDiff, error)`: one `FileDiff` per
  changed file between the two commits, sorted by path, with renames as in
  `CommitDetail` and hunks built by the existing `buildDiffHunks`. Returns
  `ErrNotFound` if either commit is missing.

Tests in `RepoBrowseTest`: the merge base of two branches that fork from one
commit is that commit; the merge base of a commit and its descendant is the
commit; a missing commit gives `ErrNotFound`. `DiffCommits` between the fork
point and a branch that edits one file and adds another gives two
`FileDiff`s with the right line types and old and new line numbers; a missing
commit gives `ErrNotFound`; a cancelled context returns its error, as the
existing `CommitFileDiff` test checks.

### Step 9: line comments on both sides of the diff

Files: modify `entities/pullrequest/op_add_review.go` and its test, the cache
and CLI code that builds or reads `LineComment` (`cache/pullrequest_test.go`,
`commands/pr/pr_review.go`, `commands/pr/pr_show.go`, `commands/pr/code.go`
and their tests).

- `LineComment` becomes `{Commit, Base repository.Hash; Path, OldPath string;
  OldLine, NewLine int; Message string}` with JSON names `commit`, `base`,
  `path`, `old_path` (omitted when empty), `old_line`, `new_line`, `message`.
  The `line` field goes away; the format version stays 1 because no release
  has written PRs.
- `Validate`: `Commit` and `Base` valid, `Path` non-empty, both line numbers
  at least 0 and at least one of them at least 1, message non-empty and
  printable.
- `pr review --at PATH:LINE:MESSAGE`: `Commit` is the head, `Base` is
  `MergeBase(target commit, head)`, `NewLine` is LINE. If the line is an
  unchanged line inside the diff, `OldLine` is filled from the diff too;
  otherwise it stays 0.
- `pr show`: prints `path:N` for a comment with a `NewLine` and `path:-N` for
  one with only an `OldLine`. The code shown is read with `readLine` at
  `Commit` for `NewLine`, and at `Base` (and `OldPath` if set) for `OldLine`.

Tests: `TestAddReviewValidate` gains cases for both lines 0, a negative line
and a bad base. `TestPrReview` checks that `--at` on the added line stores
`NewLine` 3 and `OldLine` 0, and on line 1 (unchanged) stores both as 1.
`TestPrShow` gains a comment on a removed line that prints `main.go:-N` and
the removed text read at the base. The test helper may gain a branch that
removes a line.

### Step 10: `git bug pr diff`

Files: create `commands/pr/pr_diff.go`, `commands/pr/diff.go` and their tests;
modify `commands/pr/pr.go` (register the command) and `commands/pr/pr_show.go`.

- `diff.go`: `prDiff(env, snap) (base repository.Hash, files []repository.FileDiff, err error)`,
  which resolves the target branch, computes the merge base with the head and
  calls `DiffCommits`. Steps 9 and 11 use it too; if step 9 needed the merge
  base earlier, move that code here.
- `writeUnifiedDiff(w io.Writer, files []repository.FileDiff, prefix string)`:
  writes `diff --git a/P b/P`, `--- a/P` (or `/dev/null`), `+++ b/P` (or
  `/dev/null`), `@@ -os,ol +ns,nl @@`, then each line with ` `, `+` or `-`.
  Binary files print `Binary files differ`. `prefix` is put in front of every
  line; step 11 passes `> `.
- `pr diff ID` prints the diff. `pr show` prints
  `View the changes: git bug pr diff <short id>` instead of the `git diff`
  line.

Tests: `TestPrDiff` checks the exact output for the test branches (one
added line). `TestWriteUnifiedDiff` checks a new file, a deleted file, a
rename and the prefix. `TestPrShow`'s expected output changes for the new
line.

### Step 11: reviewing in the editor

Files: create `commands/pr/review_editor.go` and its test; modify
`commands/pr/pr_review.go` and its test.

- `reviewTemplate(id, title, source, target string, head repository.Hash,
  verdict pullrequest.Verdict, files []repository.FileDiff) string`: the
  comment header shown under "CLI", the verdict word on its own line, a blank
  line, then `writeUnifiedDiff` with the `> ` prefix.
- `parseReview(text string, files []repository.FileDiff) (verdict, summary
  string, comments []lineTarget, err error)`, where
  `lineTarget{Path, OldPath string; OldLine, NewLine int; Message string}`.
  Lines starting with `#` are ignored. The first other line is the verdict
  (`comment`, `approve` or `request-changes`). Unquoted text before the first
  quoted line is the summary. Placement is anchored on the quoted `diff --git`
  and `@@` header lines, matched in diff order against `files`, not on counting
  from `@@`. Under a hunk, each quoted code line must match the next line of
  that hunk exactly; the only allowed deletions are whole files, whole hunks
  and the end of a hunk. An unquoted line after a quoted code line starts a
  comment on that line, and consecutive unquoted lines join into one comment
  with paragraphs kept (a blank line separates paragraphs and does not end the
  comment); it ends at the next quoted line. A comment under a file or hunk
  header line, a changed or out-of-order quoted line, and an unknown verdict
  are errors naming the line of the edited text.
- `pr review ID`: when neither `-m` nor `--at` is given and the session is
  interactive, it computes the diff with `prDiff`, opens the editor with
  `input.LaunchEditorWithTemplate` on the template (verdict from the
  `--approve` or `--request-changes` flag, `comment` otherwise), parses the
  result, fills `Commit` (the head) and `Base`, and submits one review. A
  review with verdict `comment`, no summary and no comment prints `Empty
  review, aborting.` and writes nothing, as `bug new` does for an empty title.
  An unchanged template opened with `--approve` or `--request-changes`
  submits that verdict with no text.

Tests (`parseReview` is tested without an editor):
- a comment under an added line, a removed line and an unchanged line give
  `NewLine` only, `OldLine` only, and both;
- a two-line comment is one comment;
- a summary above the diff, and `approve` as verdict;
- deleting whole files, whole hunks or the end of a hunk still places later comments correctly, and deleting a line in the middle of a hunk is an error;
- a comment under `> @@` fails; an unknown verdict fails;
- `TestPrReviewEditor` sets the `GIT_EDITOR` (or the editor the repository
  config uses in tests; see how `commands/input` tests do it) to a script that
  appends a comment under the added line, runs `pr review` interactively, and
  checks the stored review. If driving an editor from the test is not
  practical, test the code path after the editor with the edited text passed
  in, and say so.

After the last step, regenerate `doc/man` and `doc/md` (`go run
doc/generate.go`), since `pr diff` is new and `pr review`'s help changes.

### Step 12: comments inside `pr diff`

Files: modify `commands/pr/diff.go`, `commands/pr/pr_diff.go`,
`commands/pr/pr_show.go` (only to share helpers) and their tests, and the
callers of `writeUnifiedDiff` in `commands/pr/review_editor.go`.

- `writeUnifiedDiff(w, files, prefix, after func(file repository.FileDiff,
  line repository.DiffLine))`: `after` is called after each code line (not
  after header lines) and may be nil. The review template and the parser
  pass nil, so their output does not change.
- A comment is shown inline when its `Commit` is the PR's current head and
  its `Base` is the current merge base, and it matches a line of the diff:
  same `Path` and `NewLine` for a comment with a `NewLine` (added or
  unchanged line), or a deleted line with the same `OldLine` in the file
  whose old path is the comment's `OldPath` (or `Path` if empty) for a
  comment with only an `OldLine`. Every other comment goes to the list after
  the diff.
- Under its line, each comment prints as:

  ```
      ┃ <author>, <Mon Jan 2 15:04 2006> (<verdict word>)
      ┃ <first line of the message>
      ┃ <each further line; an empty line prints as "    ┃">
  ```

  The verdict word is the one `pr show` uses (`approved`, `requested
  changes`, `reviewed`). Several comments on one line print one after the
  other, in timeline order.
- After the diff, when there are such comments, an empty line, then
  `Comments not on a line of this diff:`, then each comment printed as
  `pr show` prints a line comment (`path:line @ short`, the code line or the
  "code not available" note, `[outdated]` when it applies), preceded by the
  same `┃ <author>, <date> (<verdict>)` line.
- `pr diff --no-comments` prints the diff exactly as before this step.

Tests: `TestPrDiff` gains a review with a comment on the added line and one
on a removed line (the helper may need a branch that removes a line) and
checks the exact output; a two-paragraph comment; two comments on one line;
a comment made on an older head appears in the list after the diff and not
inline; `--no-comments` output equals the plain diff; the review template
output is unchanged (`TestReviewTemplate` still passes untouched).

After this step, regenerate `doc/man` and `doc/md`.

### Step 13: colors

Files: modify `commands/pr/pr.go`, `pr_show.go`, `pr_diff.go`, `diff.go` and
their tests. Colors come from `util/colors`, as in `commands/bug`.

- The same roles get the same colors as in `git bug bug` and `bug show`:
  IDs cyan, statuses yellow, author names magenta.
- `pr diff`, as `git diff` colors by default: `diff --git`, mode, rename,
  `---` and `+++` lines bold; `@@` lines cyan; added lines green; removed
  lines red; unchanged lines plain. Inline comments: the `┃` bar bold
  yellow on every comment line (header and empty lines included), the author
  magenta, the date dimmed, the verdict word colored as below, the message
  text bold.
- Verdict words, wherever they appear: `approved` green, `requested changes`
  red, `reviewed` yellow. `[outdated]` and the "code not available" notes
  dimmed (or plain if `util/colors` has no dim style; adding one there is
  allowed).
- `pr show`: the title bold, the ID in `View the changes` cyan, status yellow,
  author names magenta, the `path:N @ short` line of a line comment cyan, and
  the quoted code line colored like a diff line of its side (green for a
  line with a `NewLine` that the comment's diff added, red for a removed
  line, plain otherwise; if the side is not known, plain).
- `pr` (the list): ID cyan, status yellow, author magenta, as `git bug bug`.
- `--no-colors`, a persistent flag on `git bug pr`, turns colors off for every
  `pr` subcommand. Colors are also off when the output is not a terminal or
  `NO_COLOR` is set, as `fatih/color` already does, so `pr diff
  --no-comments | git apply` keeps working.
- The editor template never has colors.
- Exact-output tests keep running with colors off (the tests set
  `color.NoColor`); add tests that force colors on and check the escape
  codes for one added line, one removed line, a verdict and the list, and a
  test that `--no-colors` turns them off.

After this step, regenerate `doc/man` and `doc/md` for the new flag.

After the last step, the PR description gets a short demo: two clones and a bare
remote exchanging a PR and a review, as exact commands with their output.

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
  API) covers most of it. Step 8 adds the missing piece, the diff between two
  commits, to the same interface.

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
