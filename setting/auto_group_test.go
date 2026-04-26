package setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdateAutoGroupsByJsonString_EmptyStringClearsGroups(t *testing.T) {
	original := AutoGroups2JsonString()
	t.Cleanup(func() {
		require.NoError(t, UpdateAutoGroupsByJsonString(original))
	})

	require.NoError(t, UpdateAutoGroupsByJsonString(""))
	require.Empty(t, GetAutoGroups())
}
