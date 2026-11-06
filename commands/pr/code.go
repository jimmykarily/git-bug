package prcmd

import (
	"bufio"
	"context"
	"errors"

	"github.com/git-bug/git-bug/repository"
)

var (
	errCommitMissing = errors.New("commit not in this clone")
	errNoSuchLine    = errors.New("no such file or line")
)

// readLine returns the given line (1-based) of the file at path, as it is at
// the commit. It returns errCommitMissing when the commit is not in the
// repository, and errNoSuchLine when the file or the line does not exist.
func readLine(ctx context.Context, repo repository.Repo, commit repository.Hash, path string, line int) (string, error) {
	if _, err := repo.ReadCommit(ctx, commit); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", errCommitMissing
		}
		return "", err
	}

	reader, _, _, err := repo.BlobAtPath(ctx, commit.String(), path)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", errNoSuchLine
		}
		return "", err
	}
	defer reader.Close()

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(nil, 1<<20)
	for n := 1; scanner.Scan(); n++ {
		if n == line {
			return scanner.Text(), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", errNoSuchLine
}
