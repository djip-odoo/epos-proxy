package escpos

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"image"
	"image/color"
	imagedraw "image/draw"
	"strings"

	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type xmlRawItem struct {
	XMLName xml.Name
	Attrs   []xml.Attr `xml:",any,attr"`
	Content string     `xml:",chardata"`
}

type xmlEPOSPrint struct {
	XMLName xml.Name     `xml:"epos-print"`
	Items   []xmlRawItem `xml:",any"`
}

func ParseXML(body []byte, targetWidth int) ([]byte, error) {
	s := string(body)

	start := strings.Index(s, "<epos-print")
	end := strings.LastIndex(s, "</epos-print>")

	if start == -1 || end == -1 {
		return nil, fmt.Errorf("no <epos-print> element found")
	}

	fragment := s[start : end+len("</epos-print>")]

	var ep xmlEPOSPrint

	if err := xml.Unmarshal([]byte(fragment), &ep); err != nil {
		return nil, fmt.Errorf("XML parse error: %w", err)
	}

	img, err := renderReceipt(ep.Items, targetWidth)
	if err != nil {
		return nil, err
	}
	return imageToEscPosBytes(img, targetWidth)
}

func renderReceipt(items []xmlRawItem, targetWidth int) (image.Image, error) {
	width := getReceiptWidth(items)
	if width == 0 {
		width = targetWidth
	}

	const padding = 10

	canvas := image.NewRGBA(image.Rect(0, 0, width, 8000))

	imagedraw.Draw(
		canvas,
		canvas.Bounds(),
		&image.Uniform{color.White},
		image.Point{},
		imagedraw.Src,
	)

	tt, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return nil, err
	}

	face, err := opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    24, // increase here
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, err
	}

	d := &font.Drawer{
		Dst:  canvas,
		Src:  image.Black,
		Face: face,
	}

	y := 30

	for _, item := range items {

		tag := strings.ToLower(item.XMLName.Local)
		attrs := attrMap(item.Attrs)

		switch tag {

		case "text":

			text := cleanText(item.Content)

			charLimit := width / 18

			if charLimit < 10 {
				charLimit = 10
			}

			lines := wrapText(text, charLimit)

			for _, line := range lines {

				x := padding

				if attrs["align"] == "center" {
					w := d.MeasureString(line).Round()
					x = (width - w) / 2
				}

				if attrs["align"] == "right" {
					w := d.MeasureString(line).Round()
					x = width - w - padding
				}

				d.Dot = fixed.P(x, y)
				d.DrawString(line)

				y += 40
			}

		case "feed":

			lines := parseInt(attrs["line"], 1)
			y += lines * 24

		case "image":

			raw := cleanBase64(item.Content)

			data, err := base64.StdEncoding.DecodeString(raw)
			if err != nil {
				continue
			}

			w := parseInt(attrs["width"], width)
			h := parseInt(attrs["height"], 0)

			if h == 0 {
				h = (len(data) * 8) / w
			}

			bw := image.NewGray(image.Rect(0, 0, w, h))

			byteIndex := 0

			for yy := 0; yy < h; yy++ {

				for xx := 0; xx < w; xx += 8 {

					if byteIndex >= len(data) {
						break
					}

					b := data[byteIndex]
					byteIndex++

					for bit := 0; bit < 8; bit++ {

						px := xx + bit

						if px >= w {
							break
						}

						mask := byte(1 << (7 - bit))

						if b&mask != 0 {
							bw.SetGray(px, yy, color.Gray{Y: 0})
						} else {
							bw.SetGray(px, yy, color.Gray{Y: 255})
						}
					}
				}
			}

			x := padding

			if attrs["align"] == "center" {
				x = (width - w) / 2
			}

			if attrs["align"] == "right" {
				x = width - w - padding
			}

			imagedraw.Draw(
				canvas,
				image.Rect(x, y, x+w, y+h),
				bw,
				image.Point{},
				imagedraw.Over,
			)

			y += h + 20
		}
	}

	finalImg := canvas.SubImage(
		image.Rect(0, 0, width, y+20),
	)

	return finalImg, nil
}

func imageToEscPosBytes(img image.Image, width int) ([]byte, error) {

	b := img.Bounds()

	srcW := b.Dx()
	srcH := b.Dy()

	aspect := float64(srcH) / float64(srcW)

	height := int(float64(width) * aspect)

	resized := image.NewRGBA(image.Rect(0, 0, width, height))

	draw.CatmullRom.Scale(
		resized,
		resized.Bounds(),
		img,
		img.Bounds(),
		imagedraw.Over,
		nil,
	)

	bw := image.NewGray(resized.Bounds())

	for y := 0; y < height; y++ {

		for x := 0; x < width; x++ {

			r, g, b, _ := resized.At(x, y).RGBA()

			gray := uint8((r + g + b) / 3 >> 8)

			if gray > 127 {
				bw.SetGray(x, y, color.Gray{Y: 255})
			} else {
				bw.SetGray(x, y, color.Gray{Y: 0})
			}
		}
	}

	widthBytes := (width + 7) / 8

	payload := make([]byte, 0)

	// GS v 0
	payload = append(payload,
		0x1d, 0x76, 0x30, 0x00,
	)

	// little endian width/height
	payload = append(payload,
		byte(widthBytes&0xFF),
		byte((widthBytes>>8)&0xFF),
		byte(height&0xFF),
		byte((height>>8)&0xFF),
	)

	for y := 0; y < height; y++ {

		for xByte := 0; xByte < widthBytes; xByte++ {

			var b byte

			for bit := 0; bit < 8; bit++ {

				x := xByte*8 + bit

				if x >= width {
					continue
				}

				if bw.GrayAt(x, y).Y == 0 {
					b |= (1 << (7 - bit))
				}
			}

			payload = append(payload, b)
		}
	}

	return payload, nil
}

func attrMap(attrs []xml.Attr) map[string]string {

	m := make(map[string]string)

	for _, a := range attrs {
		m[strings.ToLower(a.Name.Local)] = a.Value
	}

	return m
}

func parseInt(s string, def int) int {

	var n int

	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return def
	}

	return n
}

func cleanBase64(s string) string {

	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\t", "")

	return s
}

func cleanText(s string) string {

	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\r", "")

	return s
}

func wrapText(text string, limit int) []string {

	var lines []string

	for len(text) > limit {
		lines = append(lines, text[:limit])
		text = text[limit:]
	}

	if len(text) > 0 {
		lines = append(lines, text)
	}

	return lines
}

func getReceiptWidth(items []xmlRawItem) int {
	receiptWidth := 0

	for _, item := range items {

		if strings.ToLower(item.XMLName.Local) != "image" {
			continue
		}

		attrs := attrMap(item.Attrs)

		imgWidth := parseInt(attrs["width"], 0)

		if imgWidth > 0 {
			receiptWidth = imgWidth
			break
		}
	}

	return receiptWidth
}
