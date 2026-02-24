package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasTable_FalseForNonExistent(t *testing.T) {
	db := newTestDB(t)
	ok, err := db.HasTable("no_such_table")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestHasTable_TrueForMigratedTables(t *testing.T) {
	db := newTestDB(t)
	tables := []string{
		"image_sets", "images", "pending_verifications",
		"admins", "user_join_history", "verification_failures",
		"user_language_preferences", "image_set_labels",
	}
	for _, table := range tables {
		ok, err := db.HasTable(table)
		require.NoError(t, err)
		assert.True(t, ok, "expected table %q to exist", table)
	}
}
