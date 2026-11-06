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

The maintainers have expressed different views. The PoC is kept small enough
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
  under a review comment, multi-line ranges, and rendering diffs.

## Things to avoid

- Any change to how bugs work or to how existing commands behave.
- Any change to the on-disk format of bugs or identities.
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
    line comments. Each line comment has a commit ID, a file path, a line
    number (1-based, in the file as it is at that commit) and a message. A
    review is a single operation so that it is published all at once, like a
    submitted review on GitHub.
  - `SetHeadOp`: a new head commit, recorded when the source branch gets new
    or rewritten commits.
  - `SetStatusOp`: `open`, `closed` or `merged`. This is a status type of its
    own, because `entities/common.Status` has no merged state.
- Branch names are stored as plain branch names (`feature-x`, `trunk`), the
  way a forge shows them. They are labels, and the head commit ID is what the
  PR actually points at. When the CLI needs a branch's commit, it tries the
  local branch first, then `refs/remotes/<remote>/<name>` for each remote, and
  fails with a clear message if it finds none or several that disagree.
- Who may do what: anyone may record a head or change the status, and any
  status can follow any other, including reopening a merged PR. This matches
  bugs, where anyone may close or reopen, and git-bug has no permission model
  to build on. Rules can come later as checks in the CLI; adding them to the
  replay logic would change the state of existing PRs.
- Validation, at the operation level: non-empty title and branch names, commit
  IDs that are well-formed hashes, non-empty file paths, line numbers of at
  least 1, a known verdict and a known status. Whether a commit or file exists
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
show them, with no error. The push and pull test in step 4 guards this.

## CLI

Package `commands/pr`, mounted as `git bug pr`, modelled on `commands/bug`.
Commands take an ID or a unique ID prefix.

| Command | Effect |
| --- | --- |
| `git bug pr` | List PRs: short id, status, title, `source -> target`, author. |
| `git bug pr new --target BRANCH [--source BRANCH] [-t TITLE] [-m MSG]` | Open a PR. The source defaults to the checked-out branch. The head is the source branch's current commit. |
| `git bug pr show ID` | Header (title, status, branches, head, author), the message, then the timeline in the order things happened, each entry with its author and date. Line comments print under their review as `path:line @ shortcommit`, followed by that line of code read from the repository, marked `outdated` when the commit is not the current head. If the commit is not in the local clone (for example after a force-push), the comment still prints, with `code not available (commit abc123 not in this clone)` in place of the code. The header also prints the command to see the changes: `git diff TARGET_COMMIT...HEAD_COMMIT`, with the target branch resolved as described under "Data model" and both commits as full IDs. |
| `git bug pr comment ID -m MSG` | Add a general comment. |
| `git bug pr review ID [--approve \| --request-changes] [-m SUMMARY] [--at PATH:LINE:MESSAGE ...]` | Submit a review. `--at` can be repeated. Line comments are made on the PR's current head. The CLI fails if the file or line does not exist at that commit. |
| `git bug pr head ID [--commit HASH]` | Record a new head. Defaults to the source branch's current commit. `--commit` takes a full commit ID. Does nothing if the head has not moved. |
| `git bug pr status ID open\|closed\|merged` | Set the status. |

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

Each step is one commit, with tests written first, and leaves `go test ./...`
passing.

1. This document.
2. `entities/pullrequest`: the status and verdict types, `CreateOp`, the
   current state, the record type definition and its read, merge and remove
   functions.
3. `entities/pullrequest`: `AddCommentOp`, `AddReviewOp`, `SetHeadOp`,
   `SetStatusOp`, each with its timeline item.
4. `cache`: the PR summary, the wrapper for reading and changing a PR, the
   cache for all PRs, and its registration.
   Includes the cache reload test and the push and pull test.
5. `commands/pr`: `new`, the list, and `show`.
6. `commands/pr`: `comment`, `review`, `head`, `status`.
7. Documentation: generated CLI docs (`doc/man`, `doc/md`) and the
   `doc/feature-matrix.md` row for pull requests.

After step 7, a short demo script in the PR description shows two clones
exchanging a PR and a review through a bare remote.

## Questions for upstream

- Should a PR be a separate record type, as #373 suggests, or an extended bug?
- Is `refs/pullrequests/` the right namespace name?
- Is option A acceptable as a first step, with B as the follow-up?
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
- Draft reviews: collecting line comments over time and submitting them as one
  review.
- Replies under a line comment, multi-line ranges, editing, labels, linking a
  PR to the bugs it fixes.
- Detecting the merge automatically when the head commit becomes reachable
  from the target branch.
