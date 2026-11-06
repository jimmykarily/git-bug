package pullrequest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVerdictString(t *testing.T) {
	require.Equal(t, "comment", CommentVerdict.String())
	require.Equal(t, "approve", ApproveVerdict.String())
	require.Equal(t, "request-changes", RequestChangesVerdict.String())

	for _, v := range []Verdict{CommentVerdict, ApproveVerdict, RequestChangesVerdict} {
		require.NoError(t, v.Validate())
	}
	require.Error(t, Verdict(0).Validate())
	require.Error(t, Verdict(4).Validate())
}
