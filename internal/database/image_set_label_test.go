package database

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddImageSetLabel_Upsert(t *testing.T) {
	db := newTestDB(t)
	set, err := db.CreateImageSet("fruits")
	require.NoError(t, err)

	// Initial insert
	require.NoError(t, db.AddImageSetLabel(set.ID, "en", "Fruits"))

	label, err := db.GetImageSetLabel(set.ID, "en")
	require.NoError(t, err)
	assert.Equal(t, "Fruits", label)

	// Upsert — update existing
	require.NoError(t, db.AddImageSetLabel(set.ID, "en", "Fruit Basket"))

	label, err = db.GetImageSetLabel(set.ID, "en")
	require.NoError(t, err)
	assert.Equal(t, "Fruit Basket", label)
}

func TestGetImageSetLabel_NotFound(t *testing.T) {
	db := newTestDB(t)
	set, err := db.CreateImageSet("veggies")
	require.NoError(t, err)

	_, err = db.GetImageSetLabel(set.ID, "fr")
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestGetImageSetLabels(t *testing.T) {
	db := newTestDB(t)
	set, err := db.CreateImageSet("grains")
	require.NoError(t, err)

	require.NoError(t, db.AddImageSetLabel(set.ID, "en", "Grains"))
	require.NoError(t, db.AddImageSetLabel(set.ID, "zh", "谷物"))
	require.NoError(t, db.AddImageSetLabel(set.ID, "ja", "穀物"))

	labels, err := db.GetImageSetLabels(set.ID)
	require.NoError(t, err)

	assert.Equal(t, "Grains", labels["en"])
	assert.Equal(t, "谷物", labels["zh"])
	assert.Equal(t, "穀物", labels["ja"])
}

func TestGetImageSetLabels_EmptyWhenNoLabelsAdded(t *testing.T) {
	db := newTestDB(t)
	set, err := db.CreateImageSet("nuts")
	require.NoError(t, err)

	// No labels explicitly added for this set; the migration INSERT OR IGNORE
	// only ran at migration time against pre-existing rows, so a new set has none.
	labels, err := db.GetImageSetLabels(set.ID)
	require.NoError(t, err)
	assert.Empty(t, labels)
}

func TestDeleteImageSetLabel(t *testing.T) {
	db := newTestDB(t)
	set, err := db.CreateImageSet("spices")
	require.NoError(t, err)

	require.NoError(t, db.AddImageSetLabel(set.ID, "en", "Spices"))
	require.NoError(t, db.DeleteImageSetLabel(set.ID, "en"))

	_, err = db.GetImageSetLabel(set.ID, "en")
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestDeleteImageSetLabel_NoOpOnMissing(t *testing.T) {
	db := newTestDB(t)
	set, err := db.CreateImageSet("herbs")
	require.NoError(t, err)

	require.NoError(t, db.DeleteImageSetLabel(set.ID, "xx"))
}

// --- GetLocalizedLabel: 3-tier fallback ---

func TestGetLocalizedLabel_UserLang(t *testing.T) {
	db := newTestDB(t)
	set, err := db.CreateImageSet("trees")
	require.NoError(t, err)

	require.NoError(t, db.AddImageSetLabel(set.ID, "en", "Trees"))
	require.NoError(t, db.AddImageSetLabel(set.ID, "zh", "树木"))

	label, err := db.GetLocalizedLabel(set.ID, "en", "zh")
	require.NoError(t, err)
	assert.Equal(t, "Trees", label)
}

func TestGetLocalizedLabel_FallsBackToDefaultLang(t *testing.T) {
	db := newTestDB(t)
	set, err := db.CreateImageSet("rivers")
	require.NoError(t, err)

	// Only default lang label exists
	require.NoError(t, db.AddImageSetLabel(set.ID, "zh", "河流"))

	// User lang "en" not present — should fall back to default "zh"
	label, err := db.GetLocalizedLabel(set.ID, "en", "zh")
	require.NoError(t, err)
	assert.Equal(t, "河流", label)
}

func TestGetLocalizedLabel_FallsBackToOriginalLabel(t *testing.T) {
	db := newTestDB(t)
	// The migration auto-inserts a 'zh' label equal to the image_sets.label.
	// Delete it so neither user lang nor default lang has a label.
	set, err := db.CreateImageSet("mountains")
	require.NoError(t, err)

	// Remove the auto-migrated 'zh' label
	require.NoError(t, db.DeleteImageSetLabel(set.ID, "zh"))

	// Neither "fr" nor "en" labels exist — should fall back to image_sets.label
	label, err := db.GetLocalizedLabel(set.ID, "fr", "en")
	require.NoError(t, err)
	assert.Equal(t, "mountains", label)
}

func TestGetLocalizedLabel_SameLangAsDefault(t *testing.T) {
	db := newTestDB(t)
	set, err := db.CreateImageSet("lakes")
	require.NoError(t, err)

	require.NoError(t, db.AddImageSetLabel(set.ID, "zh", "湖泊"))

	// userLang == defaultLang, label exists
	label, err := db.GetLocalizedLabel(set.ID, "zh", "zh")
	require.NoError(t, err)
	assert.Equal(t, "湖泊", label)
}
