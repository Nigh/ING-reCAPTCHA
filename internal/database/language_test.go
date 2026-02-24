package database

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUserLanguage_ErrNoRowsForUnknownUser(t *testing.T) {
	db := newTestDB(t)

	_, err := db.GetUserLanguage(99999)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestSetUserLanguage_Insert(t *testing.T) {
	db := newTestDB(t)

	require.NoError(t, db.SetUserLanguage(1, "en"))

	lang, err := db.GetUserLanguage(1)
	require.NoError(t, err)
	assert.Equal(t, "en", lang)
}

func TestSetUserLanguage_Upsert(t *testing.T) {
	db := newTestDB(t)

	require.NoError(t, db.SetUserLanguage(1, "en"))
	require.NoError(t, db.SetUserLanguage(1, "zh"))

	lang, err := db.GetUserLanguage(1)
	require.NoError(t, err)
	assert.Equal(t, "zh", lang)
}

func TestSetUserLanguage_MultipleUsers(t *testing.T) {
	db := newTestDB(t)

	require.NoError(t, db.SetUserLanguage(1, "en"))
	require.NoError(t, db.SetUserLanguage(2, "ja"))

	lang1, err := db.GetUserLanguage(1)
	require.NoError(t, err)
	assert.Equal(t, "en", lang1)

	lang2, err := db.GetUserLanguage(2)
	require.NoError(t, err)
	assert.Equal(t, "ja", lang2)
}
