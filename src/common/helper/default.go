package helper

import (
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"

	"golang.org/x/image/draw"
)

func Btoa(str string) string {
	return base64.StdEncoding.EncodeToString([]byte(str))
}

// ConvertPNGToJPG checks if the output file exists. If not, it converts PNG to JPG.
// Returns (true, nil) if converted, (false, nil) if skipped, or an error.
func Thumbnail(srcPath, dstPath string, quality int) (bool, error) {
	// 1. Check if output file already exists
	if _, err := os.Stat(dstPath); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("failed to check output file: %w", err)
	}

	// 2. Open source PNG file
	inFile, err := os.Open(srcPath)
	if err != nil {
		return false, fmt.Errorf("failed to open source file: %w", err)
	}
	defer inFile.Close()

	// 3. Decode the PNG image
	img, _, err := image.Decode(inFile)
	if err != nil {
		return false, fmt.Errorf("failed to decode PNG: %w", err)
	}

	// 4. Calculate new dimensions dynamically based on quality factor
	// Ensure quality bounds are respected (1-100)
	if quality < 1 {
		quality = 1
	} else if quality > 100 {
		quality = 100
	}

	origBounds := img.Bounds()
	origWidth := origBounds.Dx()
	origHeight := origBounds.Dy()

	// Scaling factor matches the quality percentage (e.g., quality 70 -> 70% size)
	newWidth := (origWidth * quality) / 100
	newHeight := (origHeight * quality) / 100

	fmt.Println("Original dimensions:", origWidth, "x", origHeight, "Converted dimensions:", newWidth, "x", newHeight)

	// Create an empty destination image canvas with new dimensions
	rect := image.Rect(0, 0, newWidth, newHeight)
	dstImg := image.NewRGBA(rect)

	// Scale the image using Catmull-Rom (highest quality downscaling interpolation)
	draw.CatmullRom.Scale(dstImg, rect, img, origBounds, draw.Over, nil)

	// 5. Create target directory structure if missing
	dir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return false, fmt.Errorf("failed to create destination directory: %w", err)
	}

	// 6. Create destination JPG file
	outFile, err := os.Create(dstPath)
	if err != nil {
		return false, fmt.Errorf("failed to create destination file: %w", err)
	}
	defer outFile.Close()

	// 7. Encode and save the resized image canvas as JPG
	options := &jpeg.Options{Quality: quality}
	if err := jpeg.Encode(outFile, dstImg, options); err != nil {
		return false, fmt.Errorf("failed to encode JPG: %w", err)
	}

	return true, nil
}
