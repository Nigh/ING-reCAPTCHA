package imaging

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/disintegration/imaging"
)

type Composer struct {
	cachePath string
	rngPool   sync.Pool
}

func NewComposer(cachePath string) *Composer {
	return &Composer{
		cachePath: cachePath,
		rngPool: sync.Pool{
			New: func() interface{} {
				return rand.New(rand.NewSource(time.Now().UnixNano()))
			},
		},
	}
}

func (c *Composer) getRng() *rand.Rand {
	return c.rngPool.Get().(*rand.Rand)
}

func (c *Composer) putRng(rng *rand.Rand) {
	c.rngPool.Put(rng)
}

func (c *Composer) ComposeVerificationImage(imagePaths []string) (string, error) {
	if len(imagePaths) == 0 {
		return "", fmt.Errorf("no images provided")
	}

	rng := c.getRng()
	defer c.putRng(rng)

	images := make([]image.Image, 0, len(imagePaths))
	for _, path := range imagePaths {
		img, err := imaging.Open(path)
		if err != nil {
			return "", fmt.Errorf("failed to open image %s: %w", path, err)
		}
		images = append(images, img)
	}

	composed := c.compose(images, rng)
	composed = c.addRandomNoise(composed, rng)
	composed = c.addInterferenceLines(composed, rng)

	outputPath := filepath.Join(c.cachePath, fmt.Sprintf("verify_%d.png", time.Now().UnixNano()))
	if err := imaging.Save(composed, outputPath); err != nil {
		return "", fmt.Errorf("failed to save composed image: %w", err)
	}

	return outputPath, nil
}

const (
	cellSize   = 200
	padding    = 10
	numberSize = 30
)

func (c *Composer) compose(images []image.Image, rng *rand.Rand) *image.RGBA {
	n := len(images)
	cols := n
	if n > 3 {
		cols = 3
	}
	rows := (n + cols - 1) / cols

	width := cols*cellSize + (cols+1)*padding
	height := rows*cellSize + (rows+1)*padding + numberSize

	canvas := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill with Ingress-style dark background
	bgColor := color.RGBA{R: 20, G: 25, B: 30, A: 255}
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)

	// Add XM-style background pattern
	c.addXMBackground(canvas, rng)

	for i, img := range images {
		col := i % cols
		row := i / cols

		// Resize image to fit within cell (no cropping, preserve aspect ratio)
		resized := imaging.Fit(img, cellSize-20, cellSize-20, imaging.Lanczos)

		// Random slight rotation (-3 to +3 degrees)
		angle := (rng.Float64() - 0.5) * 6
		resized = imaging.Rotate(resized, angle, color.Transparent)

		// Random brightness adjustment (-5% to +5%)
		brightness := 1.0 + (rng.Float64()-0.5)*0.1
		resized = imaging.AdjustBrightness(resized, (brightness-1)*100)

		// Center the image within the cell
		imgWidth := resized.Bounds().Dx()
		imgHeight := resized.Bounds().Dy()
		x := padding + col*(cellSize+padding) + (cellSize-imgWidth)/2
		y := padding + row*(cellSize+padding) + numberSize + (cellSize-imgHeight)/2

		// Draw image
		draw.Draw(canvas, image.Rect(x, y, x+imgWidth, y+imgHeight),
			resized, image.Point{}, draw.Over)

		// Draw number label
		c.drawNumber(canvas, i+1, padding+col*(cellSize+padding)+cellSize/2-15, padding+row*(cellSize+padding))
	}

	return canvas
}

func (c *Composer) addXMBackground(canvas *image.RGBA, rng *rand.Rand) {
	bounds := canvas.Bounds()

	// Add subtle grid pattern
	gridColor := color.RGBA{R: 30, G: 40, B: 50, A: 100}
	for x := 0; x < bounds.Dx(); x += 20 {
		for y := 0; y < bounds.Dy(); y++ {
			if rng.Float64() < 0.3 {
				canvas.Set(x, y, gridColor)
			}
		}
	}
	for y := 0; y < bounds.Dy(); y += 20 {
		for x := 0; x < bounds.Dx(); x++ {
			if rng.Float64() < 0.3 {
				canvas.Set(x, y, gridColor)
			}
		}
	}

	// Add random XM particles
	xmColors := []color.RGBA{
		{R: 0, G: 200, B: 200, A: 80},   // Cyan (Resistance)
		{R: 0, G: 255, B: 0, A: 80},     // Green (Enlightened)
		{R: 255, G: 255, B: 255, A: 50}, // White (neutral XM)
	}

	for i := 0; i < 50; i++ {
		x := rng.Intn(bounds.Dx())
		y := rng.Intn(bounds.Dy())
		col := xmColors[rng.Intn(len(xmColors))]
		size := rng.Intn(3) + 1

		for dx := -size; dx <= size; dx++ {
			for dy := -size; dy <= size; dy++ {
				if dx*dx+dy*dy <= size*size {
					px, py := x+dx, y+dy
					if px >= 0 && px < bounds.Dx() && py >= 0 && py < bounds.Dy() {
						canvas.Set(px, py, col)
					}
				}
			}
		}
	}
}

func (c *Composer) drawNumber(canvas *image.RGBA, num int, x, y int) {
	// Simple number drawing using basic shapes
	numColor := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	bgCircle := color.RGBA{R: 50, G: 60, B: 70, A: 200}

	// Draw background circle
	cx, cy := x+15, y+15
	radius := 12
	for dx := -radius; dx <= radius; dx++ {
		for dy := -radius; dy <= radius; dy++ {
			if dx*dx+dy*dy <= radius*radius {
				canvas.Set(cx+dx, cy+dy, bgCircle)
			}
		}
	}

	// Draw number (simple pixel patterns for 1-6)
	patterns := map[int][][2]int{
		1: {{0, -6}, {0, -4}, {0, -2}, {0, 0}, {0, 2}, {0, 4}, {0, 6}, {-2, -4}},
		2: {{-4, -6}, {-2, -6}, {0, -6}, {2, -6}, {4, -6}, {4, -4}, {4, -2}, {2, 0}, {0, 0}, {-2, 2}, {-4, 4}, {-4, 6}, {-2, 6}, {0, 6}, {2, 6}, {4, 6}},
		3: {{-4, -6}, {-2, -6}, {0, -6}, {2, -6}, {4, -6}, {4, -4}, {4, -2}, {2, 0}, {0, 0}, {4, 2}, {4, 4}, {4, 6}, {2, 6}, {0, 6}, {-2, 6}, {-4, 6}},
		4: {{4, -6}, {4, -4}, {4, -2}, {4, 0}, {4, 2}, {4, 4}, {4, 6}, {2, 0}, {0, 0}, {-2, 0}, {-4, 0}, {-4, -2}, {-2, -4}, {0, -6}},
		5: {{-4, -6}, {-2, -6}, {0, -6}, {2, -6}, {4, -6}, {-4, -4}, {-4, -2}, {-4, 0}, {-2, 0}, {0, 0}, {2, 0}, {4, 2}, {4, 4}, {4, 6}, {2, 6}, {0, 6}, {-2, 6}, {-4, 6}},
		6: {{-4, -6}, {-2, -6}, {0, -6}, {2, -6}, {4, -6}, {-4, -4}, {-4, -2}, {-4, 0}, {-4, 2}, {-4, 4}, {-4, 6}, {-2, 6}, {0, 6}, {2, 6}, {4, 6}, {4, 4}, {4, 2}, {2, 0}, {0, 0}, {-2, 0}},
	}

	if pattern, ok := patterns[num]; ok {
		for _, p := range pattern {
			for dx := -1; dx <= 1; dx++ {
				for dy := -1; dy <= 1; dy++ {
					canvas.Set(cx+p[0]+dx, cy+p[1]+dy, numColor)
				}
			}
		}
	}
}

func (c *Composer) addRandomNoise(img *image.RGBA, rng *rand.Rand) *image.RGBA {
	bounds := img.Bounds()
	noiseIntensity := 0.02

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if rng.Float64() < noiseIntensity {
				original := img.RGBAAt(x, y)
				noise := uint8(rng.Intn(30) - 15)
				newR := clampUint8(int(original.R) + int(noise))
				newG := clampUint8(int(original.G) + int(noise))
				newB := clampUint8(int(original.B) + int(noise))
				img.SetRGBA(x, y, color.RGBA{R: newR, G: newG, B: newB, A: original.A})
			}
		}
	}
	return img
}

func (c *Composer) addInterferenceLines(img *image.RGBA, rng *rand.Rand) *image.RGBA {
	bounds := img.Bounds()
	lineColors := []color.RGBA{
		{R: 0, G: 150, B: 150, A: 40},
		{R: 0, G: 200, B: 0, A: 40},
		{R: 100, G: 100, B: 100, A: 30},
	}

	numLines := rng.Intn(3) + 2
	for i := 0; i < numLines; i++ {
		col := lineColors[rng.Intn(len(lineColors))]
		x1 := rng.Intn(bounds.Dx())
		y1 := rng.Intn(bounds.Dy())
		x2 := rng.Intn(bounds.Dx())
		y2 := rng.Intn(bounds.Dy())

		c.drawLine(img, x1, y1, x2, y2, col)
	}
	return img
}

func (c *Composer) drawLine(img *image.RGBA, x1, y1, x2, y2 int, col color.RGBA) {
	dx := abs(x2 - x1)
	dy := abs(y2 - y1)
	sx, sy := 1, 1
	if x1 >= x2 {
		sx = -1
	}
	if y1 >= y2 {
		sy = -1
	}
	err := dx - dy

	for {
		img.Set(x1, y1, col)
		if x1 == x2 && y1 == y2 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x1 += sx
		}
		if e2 < dx {
			err += dx
			y1 += sy
		}
	}
}

func (c *Composer) CleanupTempFile(path string) error {
	return os.Remove(path)
}

func clampUint8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
