package escpos

import (
	"encoding/base64"
	"encoding/xml"
	"epos-proxy/config"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	defaultWidthDots  = 576
	defaultLeftMargin = 20
	minHeightDots     = 320
	lineSpacing       = 6
)

func ParseXMLToTSPL(body []byte, psc config.PrinterSettingConfig) ([]byte, error) {
	fragment, err := extractEPOSPrint(body)
	if err != nil {
		return nil, err
	}

	var ep XmlEPOSPrint
	if err := xml.Unmarshal(fragment, &ep); err != nil {
		return nil, fmt.Errorf("XML parse error: %w", err)
	}

	widthDots := psc.Width
	if widthDots <= 0 {
		widthDots = defaultWidthDots
	}

	// TSPL bitmap width should be byte aligned
	widthDots = (widthDots + 7) &^ 7

	currY := 20

	var commands []byte

	for _, item := range ep.Items {
		tag := strings.ToLower(item.XMLName.Local)
		attrs := attrMap(item.Attrs)

		switch tag {

		case "text":
			textCmds, nextY, err := buildTSPLText(
				item.Content,
				attrs,
				widthDots,
				currY,
			)
			if err != nil {
				return nil, err
			}

			commands = append(commands, textCmds...)
			currY = nextY

		case "image":
			imgCmds, nextY, err := buildTSPLImageElement(
				item.Content,
				attrs,
				widthDots,
				currY,
			)
			if err != nil {
				return nil, err
			}

			commands = append(commands, imgCmds...)
			currY = nextY

		case "feed":
			lines := parseInt(attrs["line"], 1)
			if lines < 1 {
				lines = 1
			}

			currY += lines * 24

		case "cut":
			// Ignore for TSPL receipt mode

		case "pulse":
			// Ignore drawer pulse

		default:
			return nil, fmt.Errorf(
				"unsupported element <%s> inside <epos-print>",
				tag,
			)
		}
	}

	heightDots := currY + psc.BottomPadding
	if heightDots < minHeightDots {
		heightDots = minHeightDots
	}

	widthMM := float64(widthDots) / 8.0
	heightMM := float64(heightDots) / 8.0

	var job []byte

	job = append(job,
		[]byte(fmt.Sprintf(
			"SIZE %.1f mm,%.1f mm\r\n",
			widthMM,
			heightMM,
		))...,
	)

	/*
		Continuous receipt paper.
		Avoid GAP endless feeding.
	*/
	job = append(job, []byte("GAP 0 mm,0 mm\r\n")...)

	job = append(job, []byte("DIRECTION 0\r\n")...)
	job = append(job, []byte("REFERENCE 0,0\r\n")...)
	job = append(job, []byte("CLS\r\n")...)

	job = append(job, commands...)

	job = append(job, []byte("PRINT 1,1\r\n")...)

	return job, nil
}

func extractEPOSPrint(body []byte) ([]byte, error) {
	s := string(body)

	start := strings.Index(s, "<epos-print")
	end := strings.LastIndex(s, "</epos-print>")

	if start == -1 || end == -1 {
		return nil, fmt.Errorf("no <epos-print> element found")
	}

	return []byte(
		s[start : end+len("</epos-print>")],
	), nil
}

func buildTSPLText(
	content string,
	attrs map[string]string,
	widthDots int,
	currY int,
) ([]byte, int, error) {

	textVal := cleanText(content)
	if textVal == "" {
		return nil, currY, nil
	}

	fontName := mapTSPLFont(attrs["font"])

	xMult := 1
	yMult := 1

	if isTrue(attrs["dw"]) {
		xMult = 2
	}

	if isTrue(attrs["dh"]) {
		yMult = 2
	}

	charW, charH := getFontDimensions(fontName, xMult, yMult)

	charLimit := widthDots / charW
	if charLimit < 5 {
		charLimit = 5
	}

	lines := wrapText(textVal, charLimit)

	var out []byte

	for _, line := range lines {
		yVal := currY

		if yAttr, ok := attrs["y"]; ok {
			yVal = parseInt(yAttr, yVal)
		}

		lineWidth := utf8.RuneCountInString(line) * charW

		xVal := computeAlignedX(
			attrs["align"],
			widthDots,
			lineWidth,
		)

		if xAttr, ok := attrs["x"]; ok {
			xVal = parseInt(xAttr, xVal)
		}

		if isTrue(attrs["em"]) {
			out = append(out, []byte("BOLD 1\r\n")...)
		} else {
			out = append(out, []byte("BOLD 0\r\n")...)
		}

		tsplLine := fmt.Sprintf(
			`TEXT %d,%d,"%s",0,%d,%d,"%s"`+"\r\n",
			xVal,
			yVal,
			fontName,
			xMult,
			yMult,
			escapeTSPL(line),
		)

		out = append(out, []byte(tsplLine)...)

		if isTrue(attrs["ul"]) {
			underline := fmt.Sprintf(
				"BAR %d,%d,%d,%d\r\n",
				xVal,
				yVal+charH-2,
				lineWidth,
				2,
			)

			out = append(out, []byte(underline)...)
		}

		currY = yVal + charH + lineSpacing
	}

	return out, currY, nil
}

func buildTSPLImageElement(
	b64 string,
	attrs map[string]string,
	widthDots int,
	currY int,
) ([]byte, int, error) {

	srcWidth := parseInt(attrs["width"], widthDots)
	if srcWidth <= 0 {
		srcWidth = widthDots
	}

	srcWidth = (srcWidth + 7) &^ 7

	raw, err := decodeBase64(cleanBase64(b64))
	if err != nil {
		return nil, currY, err
	}

	srcBytesPerRow := (srcWidth + 7) / 8

	srcHeight := parseInt(attrs["height"], 0)
	if srcHeight <= 0 {
		if srcBytesPerRow <= 0 {
			return nil, currY, fmt.Errorf("invalid image width")
		}

		srcHeight = len(raw) / srcBytesPerRow
	}

	/*
		Scale image to full printer width
	*/
	scaledRaw, scaledWidth, scaledHeight, err := scaleMonoRaster(
		raw,
		srcWidth,
		srcHeight,
		widthDots,
	)
	if err != nil {
		return nil, currY, err
	}

	yVal := currY

	if yAttr, ok := attrs["y"]; ok {
		yVal = parseInt(yAttr, yVal)
	}

	imgCmd, err := BuildTSPLImageRaw(
		scaledRaw,
		scaledWidth,
		scaledHeight,
		0,
		yVal,
	)
	if err != nil {
		return nil, currY, err
	}

	nextY := yVal + scaledHeight + 20

	return imgCmd, nextY, nil
}

func BuildTSPLImageRaw(
	raw []byte,
	width int,
	height int,
	xVal int,
	yVal int,
) ([]byte, error) {

	width = (width + 7) &^ 7

	bytesPerRow := (width + 7) / 8
	expectedLen := bytesPerRow * height

	switch {

	case len(raw) < expectedLen:
		return nil, fmt.Errorf(
			"image data too short: expected %d bytes got %d",
			expectedLen,
			len(raw),
		)

	case len(raw) > expectedLen:
		raw = raw[:expectedLen]
	}

	/*
		TSPL bitmap polarity:
		1 = black
		0 = white

		Many Epson rasters are opposite.
	*/
	for i := range raw {
		raw[i] = ^raw[i]
	}

	header := fmt.Sprintf(
		"BITMAP %d,%d,%d,%d,0,",
		xVal,
		yVal,
		bytesPerRow,
		height,
	)

	var out []byte

	out = append(out, []byte(header)...)
	out = append(out, raw...)

	/*
		CRLF safer than LF on many printers
	*/
	out = append(out, '\r', '\n')

	return out, nil
}

func scaleMonoRaster(
	raw []byte,
	srcWidth int,
	srcHeight int,
	dstWidth int,
) ([]byte, int, int, error) {

	if srcWidth <= 0 ||
		srcHeight <= 0 ||
		dstWidth <= 0 {
		return nil, 0, 0,
			fmt.Errorf("invalid image dimensions")
	}

	dstWidth = (dstWidth + 7) &^ 7

	dstHeight := (srcHeight * dstWidth) / srcWidth

	srcBytesPerRow := (srcWidth + 7) / 8
	dstBytesPerRow := (dstWidth + 7) / 8

	dst := make([]byte, dstBytesPerRow*dstHeight)

	for y := 0; y < dstHeight; y++ {

		srcY := y * srcHeight / dstHeight

		for x := 0; x < dstWidth; x++ {

			srcX := x * srcWidth / dstWidth

			srcByteIndex := srcY*srcBytesPerRow + (srcX / 8)

			if srcByteIndex >= len(raw) {
				continue
			}

			srcByte := raw[srcByteIndex]

			srcBit := (srcByte >> (7 - (srcX % 8))) & 1

			if srcBit == 1 {

				dstIndex := y*dstBytesPerRow + (x / 8)

				dst[dstIndex] |= 1 << (7 - (x % 8))
			}
		}
	}

	return dst, dstWidth, dstHeight, nil
}

func mapTSPLFont(font string) string {
	switch strings.ToLower(font) {

	case "font_a":
		return "3"

	case "font_b":
		return "2"

	case "font_c":
		return "1"

	default:
		return "3"
	}
}

func computeAlignedX(
	align string,
	widthDots int,
	contentWidth int,
) int {

	x := defaultLeftMargin

	switch strings.ToLower(align) {

	case "center":
		x = (widthDots - contentWidth) / 2

	case "right":
		x = widthDots - contentWidth - defaultLeftMargin
	}

	if x < defaultLeftMargin {
		x = defaultLeftMargin
	}

	return x
}

func getFontDimensions(
	fontName string,
	xMult int,
	yMult int,
) (int, int) {

	charW := 16
	charH := 24

	switch fontName {

	case "1":
		charW, charH = 8, 12

	case "2":
		charW, charH = 12, 20

	case "3":
		charW, charH = 16, 24

	case "4":
		charW, charH = 24, 32

	case "5":
		charW, charH = 32, 48

	case "6":
		charW, charH = 14, 19

	case "7":
		charW, charH = 21, 27

	case "8":
		charW, charH = 33, 44
	}

	return charW * xMult, charH * yMult
}

func decodeBase64(s string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(s)
	if err == nil {
		return raw, nil
	}

	raw, err = base64.RawStdEncoding.DecodeString(s)
	if err == nil {
		return raw, nil
	}

	return nil, fmt.Errorf("base64 decode failed: %w", err)
}

func escapeTSPL(s string) string {
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func isTrue(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))

	return v == "1" ||
		v == "true" ||
		v == "yes"
}
