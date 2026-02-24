package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateImageSet(t *testing.T) {
	db := newTestDB(t)

	set, err := db.CreateImageSet("animals")
	require.NoError(t, err)
	require.NotNil(t, set)

	assert.Greater(t, set.ID, int64(0))
	assert.Equal(t, "animals", set.Label)
}

func TestCreateImageSet_DuplicateReturnsError(t *testing.T) {
	db := newTestDB(t)

	_, err := db.CreateImageSet("animals")
	require.NoError(t, err)

	_, err = db.CreateImageSet("animals")
	assert.Error(t, err)
}

func TestGetImageSetByLabel_Found(t *testing.T) {
	db := newTestDB(t)
	_, err := db.CreateImageSet("plants")
	require.NoError(t, err)

	set, err := db.GetImageSetByLabel("plants")
	require.NoError(t, err)
	require.NotNil(t, set)
	assert.Equal(t, "plants", set.Label)
}

func TestGetImageSetByLabel_NilForMissing(t *testing.T) {
	db := newTestDB(t)

	set, err := db.GetImageSetByLabel("nonexistent")
	require.NoError(t, err)
	assert.Nil(t, set)
}

func TestGetAllImageSets(t *testing.T) {
	db := newTestDB(t)

	sets, err := db.GetAllImageSets()
	require.NoError(t, err)
	assert.Empty(t, sets)

	_, err = db.CreateImageSet("zebras")
	require.NoError(t, err)
	_, err = db.CreateImageSet("ants")
	require.NoError(t, err)

	sets, err = db.GetAllImageSets()
	require.NoError(t, err)
	require.Len(t, sets, 2)

	// Ordered by label
	assert.Equal(t, "ants", sets[0].Label)
	assert.Equal(t, "zebras", sets[1].Label)
}

func TestDeleteImageSet_CascadesToImages(t *testing.T) {
	db := newTestDB(t)

	set, err := db.CreateImageSet("reptiles")
	require.NoError(t, err)

	_, err = db.AddImage(set.ID, "lizard1", "")
	require.NoError(t, err)
	_, err = db.AddImage(set.ID, "snake1", "")
	require.NoError(t, err)

	count, err := db.GetImageCountBySet(set.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	require.NoError(t, db.DeleteImageSet("reptiles"))

	// Set should be gone
	found, err := db.GetImageSetByLabel("reptiles")
	require.NoError(t, err)
	assert.Nil(t, found)

	// Images should be cascaded away
	count, err = db.GetImageCountBySet(set.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestDeleteImageSet_NoOpOnMissing(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, db.DeleteImageSet("ghost"))
}

func TestGetRandomImageSets(t *testing.T) {
	db := newTestDB(t)

	for _, label := range []string{"a", "b", "c", "d", "e"} {
		_, err := db.CreateImageSet(label)
		require.NoError(t, err)
	}

	sets, err := db.GetRandomImageSets(3)
	require.NoError(t, err)
	assert.Len(t, sets, 3)
}

func TestGetRandomImageSets_FewerThanRequested(t *testing.T) {
	db := newTestDB(t)

	_, err := db.CreateImageSet("only")
	require.NoError(t, err)

	sets, err := db.GetRandomImageSets(5)
	require.NoError(t, err)
	assert.Len(t, sets, 1)
}

func TestGetImageSetCount(t *testing.T) {
	db := newTestDB(t)

	count, err := db.GetImageSetCount()
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	_, err = db.CreateImageSet("one")
	require.NoError(t, err)
	_, err = db.CreateImageSet("two")
	require.NoError(t, err)

	count, err = db.GetImageSetCount()
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}
