package samples

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// textImage renders text with the 7x13 bitmap font and scales it up so it stays readable.
func drawText(dst *image.RGBA, x, y int, s string, scale int, c color.Color) {
	w := font.MeasureString(basicfont.Face7x13, s).Ceil()
	if w == 0 {
		return
	}
	small := image.NewRGBA(image.Rect(0, 0, w, 16))
	d := &font.Drawer{Dst: small, Src: image.NewUniform(c), Face: basicfont.Face7x13, Dot: fixed.P(0, 12)}
	d.DrawString(s)
	for sy := 0; sy < 16; sy++ {
		for sx := 0; sx < w; sx++ {
			_, _, _, a := small.At(sx, sy).RGBA()
			if a == 0 {
				continue
			}
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					dst.Set(x+sx*scale+dx, y+sy*scale+dy, c)
				}
			}
		}
	}
}

func fill(dst *image.RGBA, r image.Rectangle, c color.Color) {
	draw.Draw(dst, r, image.NewUniform(c), image.Point{}, draw.Src)
}

// IDCard draws an identity-card style image with a SAMPLE banner.
func IDCard(header string, lines []string, accent color.RGBA) []byte {
	const W, H = 1200, 760
	img := image.NewRGBA(image.Rect(0, 0, W, H))
	fill(img, img.Bounds(), color.RGBA{250, 248, 240, 255})
	fill(img, image.Rect(0, 0, W, 120), accent)
	drawText(img, 40, 30, header, 4, color.White)
	fill(img, image.Rect(40, 170, 300, 480), color.RGBA{210, 214, 222, 255}) // photo box
	drawText(img, 95, 300, "PHOTO", 4, color.RGBA{120, 120, 130, 255})
	y := 170
	for _, l := range lines {
		drawText(img, 340, y, l, 3, color.RGBA{20, 20, 30, 255})
		y += 58
	}
	fill(img, image.Rect(0, H-90, W, H), color.RGBA{200, 30, 30, 255})
	drawText(img, 40, H-75, "SAMPLE - SYNTHETIC - NOT A REAL DOCUMENT", 4, color.White)
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}

// FamilyPhoto draws a simple synthetic "family photograph" (shapes only, no real people).
// With blurry=true the image is blurred heavily, which DocCheck must reject.
func FamilyPhoto(caption string, blurry bool) []byte {
	const W, H = 1000, 750
	img := image.NewRGBA(image.Rect(0, 0, W, H))
	for y := 0; y < H; y++ {
		c := color.RGBA{uint8(150 + y*80/H), uint8(190 + y*40/H), 235, 255}
		fill(img, image.Rect(0, y, W, y+1), c)
	}
	fill(img, image.Rect(0, 560, W, H), color.RGBA{90, 160, 90, 255})
	people := []struct {
		x, h int
		c    color.RGBA
	}{{260, 330, color.RGBA{180, 60, 60, 255}}, {450, 300, color.RGBA{60, 90, 180, 255}}, {640, 200, color.RGBA{220, 170, 40, 255}}}
	for _, p := range people {
		top := 600 - p.h
		fill(img, image.Rect(p.x-55, top+80, p.x+55, 600), p.c)
		for dy := -40; dy <= 40; dy++ {
			for dx := -40; dx <= 40; dx++ {
				if dx*dx+dy*dy <= 1600 {
					img.Set(p.x+dx, top+40+dy, color.RGBA{230, 190, 150, 255})
				}
			}
		}
	}
	drawText(img, 30, 30, "SAMPLE FAMILY PHOTOGRAPH - SYNTHETIC", 3, color.RGBA{30, 30, 30, 255})
	drawText(img, 30, H-60, "Not a real family. For Child Connect testing only.", 2, color.RGBA{20, 20, 20, 255})
	if caption != "" {
		drawText(img, 30, 80, caption, 2, color.RGBA{30, 30, 30, 255})
	}
	if blurry {
		img = boxBlur(boxBlur(boxBlur(img, 9), 9), 9)
	}
	var b bytes.Buffer
	_ = jpeg.Encode(&b, img, &jpeg.Options{Quality: 88})
	return b.Bytes()
}

func boxBlur(src *image.RGBA, r int) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	tmp := image.NewRGBA(b)
	out := image.NewRGBA(b)
	pass := func(dst, s *image.RGBA, horiz bool) {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				var sr, sg, sb, n int
				for k := -r; k <= r; k++ {
					xx, yy := x, y
					if horiz {
						xx += k
					} else {
						yy += k
					}
					if xx < 0 || yy < 0 || xx >= w || yy >= h {
						continue
					}
					i := s.PixOffset(xx, yy)
					sr += int(s.Pix[i])
					sg += int(s.Pix[i+1])
					sb += int(s.Pix[i+2])
					n++
				}
				i := dst.PixOffset(x, y)
				dst.Pix[i], dst.Pix[i+1], dst.Pix[i+2], dst.Pix[i+3] = uint8(sr/n), uint8(sg/n), uint8(sb/n), 255
			}
		}
	}
	pass(tmp, src, true)
	pass(out, tmp, false)
	return out
}
