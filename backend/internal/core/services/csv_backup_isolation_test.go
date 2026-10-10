package services

import (
	"github.com/stretchr/testify/require"
	"testing"
)

// Live blob references and account/collection policies must not be restored from
// an inventory ZIP. The archive list also drives import validation and wiping.
func TestCSVMetadataExcludedFromInventoryArchives(t *testing.T) {
	for _, table := range exportTables {
		require.NotEqual(t, "csv_exports", table.name)
		require.NotEqual(t, "users", table.name)
		require.NotEqual(t, "groups", table.name)
		require.NotEqual(t, "exports", table.name)
	}
}
