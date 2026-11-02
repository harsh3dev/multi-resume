package compile

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"path"
	"strings"
)

// PlaceholderKey is the pseudo object key callers put in the assets map for an
// image the user has not uploaded yet. Compile writes a generated stand-in
// picture at that path instead of fetching anything from the object store, so a
// template that references a photo still compiles and the preview shows a grey
// silhouette in its place.
const PlaceholderKey = "placeholder:v1"

// placeholderSize is the pixel size of the generated picture; templates scale
// it to whatever box they reserve.
const placeholderSize = 240

// PlaceholderImage returns generated image bytes suited to the file extension
// of texPath (.jpg/.jpeg → JPEG, anything else → PNG).
func PlaceholderImage(texPath string) []byte {
	img := image.NewRGBA(image.Rect(0, 0, placeholderSize, placeholderSize))
	bg := color.RGBA{R: 0xE4, G: 0xE7, B: 0xEB, A: 0xFF}
	fg := color.RGBA{R: 0xB4, G: 0xBA, B: 0xC2, A: 0xFF}
	for y := 0; y < placeholderSize; y++ {
		for x := 0; x < placeholderSize; x++ {
			img.SetRGBA(x, y, bg)
		}
	}
	// Head and shoulders: a circle over a wider, lower circle.
	fillCircle(img, placeholderSize/2, placeholderSize*2/5, placeholderSize/6, fg)
	fillCircle(img, placeholderSize/2, placeholderSize*6/5-10, placeholderSize*11/24, fg)
	var buf bytes.Buffer
	switch strings.ToLower(path.Ext(texPath)) {
	case ".jpg", ".jpeg":
		_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85})
	default:
		_ = png.Encode(&buf, img)
	}
	return buf.Bytes()
}

func fillCircle(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	b := img.Bounds()
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			if !(image.Point{X: x, Y: y}).In(b) {
				continue
			}
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy <= r*r {
				img.SetRGBA(x, y, c)
			}
		}
	}
}
