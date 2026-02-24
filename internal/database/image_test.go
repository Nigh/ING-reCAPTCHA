package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddImage_WithPath(t *testing.T) {
	db := newTestDB(t)
	set, err := db.CreateImageSet("cats")
	require.NoError(t, err)

	img, err := db.AddImage(set.ID, "file123", "/path/to/file.jpg")
	require.NoError(t, err)
	require.NotNil(t, img)

	assert.Equal(t, set.ID, img.SetID)
	assert.Equal(t, "file123", img.FileID)
	assert.True(t, img.FilePath.Valid)
	assert.Equal(t, "/path/to/file.jpg", img.FilePath.String)
}

func TestAddImage_WithoutPath(t *testing.T) {
	db := newTestDB(t)
	set, err := db.CreateImageSet("dogs")
	require.NoError(t, err)

	img, err := db.AddImage(set.ID, "file456", "")
	require.NoError(t, err)
	require.NotNil(t, img)

	assert.False(t, img.FilePath.Valid)
}

func TestGetRandomImageFromSet_NilForEmptySet(t *testing.T) {
	db := newTestDB(t)
	set, err := db.CreateImageSet("empty")
	require.NoError(t, err)

	img, err := db.GetRandomImageFromSet(set.ID)
	require.NoError(t, err)
	assert.Nil(t, img)
}

func TestGetRandomImageFromSet_ReturnsImage(t *testing.T) {
	db := newTestDB(t)
	set, err := db.CreateImageSet("birds")
	require.NoError(t, err)

	_, err = db.AddImage(set.ID, "bird1", "")
	require.NoError(t, err)

	img, err := db.GetRandomImageFromSet(set.ID)
	require.NoError(t, err)
	require.NotNil(t, img)
	assert.Equal(t, "bird1", img.FileID)
}

func TestUpdateImageFilePath(t *testing.T) {
	db := newTestDB(t)
	set, err := db.CreateImageSet("fish")
	require.NoError(t, err)

	img, err := db.AddImage(set.ID, "fish1", "")
	require.NoError(t, err)

	require.NoError(t, db.UpdateImageFilePath(img.ID, "/new/path.jpg"))

	// Verify via ListImagesWithoutCache — should now be empty
	uncached, err := db.ListImagesWithoutCache(0)
	require.NoError(t, err)
	for _, u := range uncached {
		assert.NotEqual(t, img.ID, u.ID)
	}
}

func TestDeleteImageByID(t *testing.T) {
	db := newTestDB(t)
	set, err := db.CreateImageSet("frogs")
	require.NoError(t, err)

	img, err := db.AddImage(set.ID, "frog1", "")
	require.NoError(t, err)

	require.NoError(t, db.DeleteImageByID(img.ID))

	count, err := db.GetImageCountBySet(set.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestListImagesWithoutCache_Limit(t *testing.T) {
	db := newTestDB(t)
	set, err := db.CreateImageSet("horses")
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		_, err := db.AddImage(set.ID, "horse"+string(rune('A'+i)), "")
		require.NoError(t, err)
	}

	// Limit to 3
	imgs, err := db.ListImagesWithoutCache(3)
	require.NoError(t, err)
	assert.Len(t, imgs, 3)

	// No limit returns all 5
	imgs, err = db.ListImagesWithoutCache(0)
	require.NoError(t, err)
	assert.Len(t, imgs, 5)
}

func TestListImagesWithoutCache_ExcludesCached(t *testing.T) {
	db := newTestDB(t)
	set, err := db.CreateImageSet("wolves")
	require.NoError(t, err)

	cached, err := db.AddImage(set.ID, "wolf1", "/cached.jpg")
	require.NoError(t, err)
	_, err = db.AddImage(set.ID, "wolf2", "")
	require.NoError(t, err)

	imgs, err := db.ListImagesWithoutCache(0)
	require.NoError(t, err)
	for _, img := range imgs {
		assert.NotEqual(t, cached.ID, img.ID)
	}
	assert.Len(t, imgs, 1)
}

func TestGetImageCountBySet(t *testing.T) {
	db := newTestDB(t)
	set, err := db.CreateImageSet("owls")
	require.NoError(t, err)

	count, err := db.GetImageCountBySet(set.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	_, err = db.AddImage(set.ID, "owl1", "")
	require.NoError(t, err)
	_, err = db.AddImage(set.ID, "owl2", "")
	require.NoError(t, err)

	count, err = db.GetImageCountBySet(set.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestGetAllImageStats(t *testing.T) {
	db := newTestDB(t)

	setA, err := db.CreateImageSet("setA")
	require.NoError(t, err)
	setB, err := db.CreateImageSet("setB")
	require.NoError(t, err)

	_, err = db.AddImage(setA.ID, "a1", "")
	require.NoError(t, err)
	_, err = db.AddImage(setA.ID, "a2", "")
	require.NoError(t, err)
	_, err = db.AddImage(setB.ID, "b1", "")
	require.NoError(t, err)

	stats, err := db.GetAllImageStats()
	require.NoError(t, err)

	assert.Equal(t, 2, stats["setA"])
	assert.Equal(t, 1, stats["setB"])
}

func TestGetAllImageStats_EmptySet(t *testing.T) {
	db := newTestDB(t)
	_, err := db.CreateImageSet("empty")
	require.NoError(t, err)

	stats, err := db.GetAllImageStats()
	require.NoError(t, err)
	assert.Equal(t, 0, stats["empty"])
}
