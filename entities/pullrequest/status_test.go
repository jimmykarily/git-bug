package pullrequest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStatusFromString(t *testing.T) {
	for _, s := range []Status{OpenStatus, ClosedStatus, MergedStatus} {
		require.NoError(t, s.Validate())
		got, err := StatusFromString(s.String())
		require.NoError(t, err)
		require.Equal(t, s, got)
	}

	require.Equal(t, "open", OpenStatus.String())
	require.Equal(t, "closed", ClosedStatus.String())
	require.Equal(t, "merged", MergedStatus.String())

	_, err := StatusFromString("nope")
	require.Error(t, err)
	require.Error(t, Status(0).Validate())
	require.Error(t, Status(4).Validate())
}
