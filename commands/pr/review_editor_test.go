package prcmd

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/git-bug/git-bug/entities/pullrequest"
	"github.com/git-bug/git-bug/repository"
)

// reviewTestFiles is the diff of a renamed file with an unchanged, a removed
// and an added line, followed by a new file.
func reviewTestFiles() []repository.FileDiff {
	old := "old.go"
	return []repository.FileDiff{
		{
			Path:    "main.go",
			OldPath: &old,
			Hunks: []repository.DiffHunk{{OldStart: 1, OldLines: 2, NewStart: 1, NewLines: 2, Lines: []repository.DiffLine{
				{Type: repository.DiffLineContext, Content: "line one", OldLine: 1, NewLine: 1},
				{Type: repository.DiffLineDeleted, Content: "line two", OldLine: 2},
				{Type: repository.DiffLineAdded, Content: "line 2", NewLine: 2},
			}}},
		},
		{
			Path:  "new.go",
			IsNew: true,
			Hunks: []repository.DiffHunk{{OldStart: 0, OldLines: 0, NewStart: 1, NewLines: 2, Lines: []repository.DiffLine{
				{Type: repository.DiffLineAdded, Content: "hello", NewLine: 1},
				{Type: repository.DiffLineAdded, Content: "world", NewLine: 2},
			}}},
		},
	}
}

func TestReviewTemplate(t *testing.T) {
	head := repository.Hash("db6beb8000000000000000000000000000000000")
	got := reviewTemplate("0c8c916", "Add X", "feature", "trunk", head, pullrequest.ApproveVerdict, reviewTestFiles())

	expected := `# Review of 0c8c916 "Add X" (feature -> trunk, head db6beb8)
# First line: comment, approve or request-changes.
# Text before the diff is the summary. Write a comment on the line(s) right
# under the diff line it is about. Lines starting with # are ignored.
# You may delete whole files, whole hunks, or the end of a hunk, and must keep
# the other quoted lines as they are.
approve

> diff --git a/old.go b/main.go
> rename from old.go
> rename to main.go
> --- a/old.go
> +++ b/main.go
> @@ -1,2 +1,2 @@
>  line one
> -line two
> +line 2
> diff --git a/new.go b/new.go
> new file mode 100644
> --- /dev/null
> +++ b/new.go
> @@ -0,0 +1,2 @@
> +hello
> +world
`
	require.Equal(t, expected, got)
}

func TestParseReview(t *testing.T) {
	head := repository.Hash("db6beb8000000000000000000000000000000000")
	files := reviewTestFiles()
	template := reviewTemplate("0c8c916", "Add X", "feature", "trunk", head, pullrequest.CommentVerdict, files)

	t.Run("unchanged template", func(t *testing.T) {
		verdict, summary, comments, err := parseReview(template, files)
		require.NoError(t, err)
		require.Equal(t, "comment", verdict)
		require.Empty(t, summary)
		require.Empty(t, comments)
	})

	t.Run("comments on each side", func(t *testing.T) {
		text := `comment
> diff --git a/old.go b/main.go
> rename from old.go
> rename to main.go
> --- a/old.go
> +++ b/main.go
> @@ -1,2 +1,2 @@
>  line one
on both sides
> -line two
removed
> +line 2
added
`
		verdict, summary, comments, err := parseReview(text, files)
		require.NoError(t, err)
		require.Equal(t, "comment", verdict)
		require.Empty(t, summary)
		require.Equal(t, []lineTarget{
			{Path: "main.go", OldPath: "old.go", OldLine: 1, NewLine: 1, Message: "on both sides"},
			{Path: "main.go", OldPath: "old.go", OldLine: 2, Message: "removed"},
			{Path: "main.go", NewLine: 2, Message: "added"},
		}, comments)
	})

	t.Run("two-line comment, summary and approve", func(t *testing.T) {
		text := `# a note
approve

Nice work.
# ignored
Second line of the summary.

> diff --git a/new.go b/new.go
> @@ -0,0 +1,2 @@
> +hello
first line
second line

> +world
`
		verdict, summary, comments, err := parseReview(text, files)
		require.NoError(t, err)
		require.Equal(t, "approve", verdict)
		require.Equal(t, "Nice work.\nSecond line of the summary.", summary)
		require.Equal(t, []lineTarget{
			{Path: "new.go", NewLine: 1, Message: "first line\nsecond line"},
		}, comments)
	})

	t.Run("paragraphs form one comment", func(t *testing.T) {
		text := "comment\n> +++ b/new.go\n> @@ -0,0 +1,2 @@\n> +hello\nfirst paragraph\n\n\nsecond paragraph\n\n> +world\n"
		_, _, comments, err := parseReview(text, files)
		require.NoError(t, err)
		require.Equal(t, []lineTarget{
			{Path: "new.go", NewLine: 1, Message: "first paragraph\n\nsecond paragraph"},
		}, comments)
	})

	t.Run("whole first file and a hunk tail deleted", func(t *testing.T) {
		text := `request-changes
> diff --git a/new.go b/new.go
> new file mode 100644
> --- /dev/null
> +++ b/new.go
> @@ -0,0 +1,2 @@
> +hello
say hello to whom
`
		verdict, _, comments, err := parseReview(text, files)
		require.NoError(t, err)
		require.Equal(t, "request-changes", verdict)
		require.Equal(t, []lineTarget{
			{Path: "new.go", NewLine: 1, Message: "say hello to whom"},
		}, comments)
	})

	t.Run("deleted file", func(t *testing.T) {
		gone := []repository.FileDiff{{
			Path:     "gone.go",
			IsDelete: true,
			Hunks: []repository.DiffHunk{{OldStart: 1, OldLines: 1, NewStart: 0, NewLines: 0, Lines: []repository.DiffLine{
				{Type: repository.DiffLineDeleted, Content: "bye", OldLine: 1},
			}}},
		}}
		text := reviewTemplate("0c8c916", "Add X", "feature", "trunk", head, pullrequest.CommentVerdict, gone) + "why remove it\n"
		_, _, comments, err := parseReview(text, gone)
		require.NoError(t, err)
		require.Equal(t, []lineTarget{{Path: "gone.go", OldLine: 1, Message: "why remove it"}}, comments)
	})

	t.Run("blank context line with trailing spaces stripped", func(t *testing.T) {
		blank := []repository.FileDiff{{
			Path: "a.go",
			Hunks: []repository.DiffHunk{{OldStart: 1, OldLines: 2, NewStart: 1, NewLines: 3, Lines: []repository.DiffLine{
				{Type: repository.DiffLineContext, Content: "", OldLine: 1, NewLine: 1},
				{Type: repository.DiffLineAdded, Content: "x", NewLine: 2},
				{Type: repository.DiffLineContext, Content: "y", OldLine: 2, NewLine: 3},
			}}},
		}}
		_, _, comments, err := parseReview("comment\n> diff --git a/a.go b/a.go\n> @@ -1,2 +1,3 @@\n>\nwhy blank\n", blank)
		require.NoError(t, err)
		require.Equal(t, []lineTarget{{Path: "a.go", OldLine: 1, NewLine: 1, Message: "why blank"}}, comments)
	})

	for name, tc := range map[string]struct {
		text string
		err  string
	}{
		"comment under a hunk header": {
			text: "comment\n> diff --git a/new.go b/new.go\n> @@ -0,0 +1,2 @@\nhere\n",
			err:  "line 4: a comment must be under a line of the diff, not under a file or hunk header",
		},
		"comment under a file header": {
			text: "comment\n\n> diff --git a/new.go b/new.go\nhere\n",
			err:  "line 4: a comment must be under a line of the diff, not under a file or hunk header",
		},
		"line typed inside a hunk": {
			text: "comment\n> diff --git a/new.go b/new.go\n> @@ -0,0 +1,2 @@\n> +not in the diff\nhere\n",
			err:  "line 4: " + keepHunkLines,
		},
		"line deleted in the middle of a hunk": {
			text: "comment\n> +++ b/main.go\n> @@ -1,2 +1,2 @@\n> -line two\nwhy\n",
			err:  "line 4: " + keepHunkLines,
		},
		"code line without a hunk header": {
			text: "comment\n> diff --git a/new.go b/new.go\n> +hello\nhere\n",
			err:  "line 3: a quoted line of code needs its file and hunk header above it",
		},
		"unknown file": {
			text: "comment\n> diff --git a/other.go b/other.go\n",
			err:  "line 2: this file is not in the diff",
		},
		"unknown hunk": {
			text: "comment\n> +++ b/new.go\n> @@ -0,0 +1,3 @@\n",
			err:  "line 3: this hunk header is not in the diff of new.go",
		},
		"deleted file selected by +++ /dev/null": {
			text: "comment\n> +++ /dev/null\n",
			err:  "line 2: a quoted line of code needs its file and hunk header above it",
		},
		"hunk header without a file header": {
			text: "comment\n> @@ -0,0 +1,2 @@\n",
			err:  "line 2: this hunk header has no file header above it",
		},
		"unknown verdict": {
			text: "lgtm\n",
			err:  `unknown verdict "lgtm", expected comment, approve or request-changes`,
		},
		"missing verdict": {
			text: "> +hello\nhere\n",
			err:  "the first line must be the verdict (comment, approve or request-changes)",
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, _, err := parseReview(tc.text, files)
			require.EqualError(t, err, tc.err)
		})
	}

	t.Run("empty text", func(t *testing.T) {
		verdict, summary, comments, err := parseReview("# only comments\n\n", files)
		require.NoError(t, err)
		require.Empty(t, verdict)
		require.Empty(t, summary)
		require.Empty(t, comments)
	})
}

const keepHunkLines = "quoted lines inside a hunk must be kept as they are; you may delete whole files, whole hunks, or the end of a hunk"

func TestParseReviewRepeatedLine(t *testing.T) {
	file := func(path string) repository.FileDiff {
		return repository.FileDiff{
			Path: path,
			Hunks: []repository.DiffHunk{{OldStart: 1, OldLines: 1, NewStart: 1, NewLines: 3, Lines: []repository.DiffLine{
				{Type: repository.DiffLineContext, Content: "func f() error {", OldLine: 1, NewLine: 1},
				{Type: repository.DiffLineAdded, Content: "\treturn nil", NewLine: 2},
				{Type: repository.DiffLineAdded, Content: "}", NewLine: 3},
			}}},
		}
	}
	twoFiles := []repository.FileDiff{file("a.go"), file("b.go")}

	t.Run("headers kept", func(t *testing.T) {
		text := "comment\n> +++ b/b.go\n> @@ -1,1 +1,3 @@\n>  func f() error {\n> +\treturn nil\nwrap the error\n> +}\n"
		_, _, comments, err := parseReview(text, twoFiles)
		require.NoError(t, err)
		require.Equal(t, []lineTarget{{Path: "b.go", NewLine: 2, Message: "wrap the error"}}, comments)
	})

	for name, text := range map[string]string{
		"one line":  "comment\n> +\treturn nil\nwrap the error\n",
		"two lines": "comment\n>  func f() error {\n> +\treturn nil\nwrap the error\n",
	} {
		t.Run(name+" without headers", func(t *testing.T) {
			_, _, _, err := parseReview(text, twoFiles)
			require.EqualError(t, err, "line 2: a quoted line of code needs its file and hunk header above it")
		})
	}
}
