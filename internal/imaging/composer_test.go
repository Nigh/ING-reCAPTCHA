package imaging

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComposeVerificationImage_NoImages(t *testing.T) {
	c := NewComposer(t.TempDir())
	_, err := c.ComposeVerificationImage(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no images provided")
}

func TestComposeVerificationImage_Valid(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "..", "..")
	img1 := filepath.Join(projectRoot, "images", "1.jpg")
	img2 := filepath.Join(projectRoot, "images", "2.jpg")

	cacheDir := t.TempDir()
	c := NewComposer(cacheDir)

	outPath, err := c.ComposeVerificationImage([]string{img1, img2})
	require.NoError(t, err)
	assert.NotEmpty(t, outPath)

	info, err := os.Stat(outPath)
	require.NoError(t, err)
	assert.True(t, info.Size() > 0, "output file should not be empty")
	assert.Equal(t, ".png", filepath.Ext(outPath))
}

func TestComposeVerificationImage_InvalidPath(t *testing.T) {
	c := NewComposer(t.TempDir())
	_, err := c.ComposeVerificationImage([]string{"/nonexistent/path/image.jpg"})
	require.Error(t, err)
}

func TestCleanupTempFile(t *testing.T) {
	c := NewComposer(t.TempDir())

	f, err := os.CreateTemp(t.TempDir(), "cleanup_test_*.tmp")
	require.NoError(t, err)
	tmpPath := f.Name()
	f.Close()

	// File should exist before cleanup
	_, err = os.Stat(tmpPath)
	require.NoError(t, err)

	// Cleanup should remove it
	err = c.CleanupTempFile(tmpPath)
	require.NoError(t, err)

	_, err = os.Stat(tmpPath)
	assert.True(t, os.IsNotExist(err), "file should be gone after cleanup")

	// Cleanup on already-missing file should return an error
	err = c.CleanupTempFile(tmpPath)
	assert.Error(t, err)
}

func TestClampUint8(t *testing.T) {
	assert.Equal(t, uint8(0), clampUint8(0))
	assert.Equal(t, uint8(127), clampUint8(127))
	assert.Equal(t, uint8(255), clampUint8(255))
	assert.Equal(t, uint8(0), clampUint8(-1))
	assert.Equal(t, uint8(255), clampUint8(256))
}

func TestAbs(t *testing.T) {
	assert.Equal(t, 5, abs(-5))
	assert.Equal(t, 0, abs(0))
	assert.Equal(t, 3, abs(3))
}
