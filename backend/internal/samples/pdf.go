// Package samples generates the synthetic sample documents used by the seed and by tests.
// Every file is clearly marked SAMPLE and contains no real person's data (section 19).
package samples

import (
	"bytes"
	"fmt"
	"strings"
)

// Line is one text line on a PDF page; Size is the font size in points, Bold picks Helvetica-Bold.
type Line struct {
	Text string
	Size float64
	Bold bool
	Gap  float64 // extra space before the line
}

func esc(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`)
	// Keep to WinAnsi-safe ASCII; replace anything else.
	var b strings.Builder
	for _, c := range r.Replace(s) {
		if c < 32 || c > 126 {
			b.WriteByte('?')
		} else {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// PDF builds a one-page A4 PDF with the given lines, a border and a "SAMPLE" watermark.
func PDF(title string, lines []Line) []byte {
	var content bytes.Buffer
	// Border and light watermark.
	content.WriteString("0.85 0.85 0.85 RG 2 w 30 30 535 782 re S\n")
	content.WriteString("BT /F2 60 Tf 0.93 0.93 0.93 rg 1 0 0 1 120 380 Tm (SAMPLE - SYNTHETIC) Tj ET\n")
	y := 790.0
	for _, l := range lines {
		size := l.Size
		if size == 0 {
			size = 11
		}
		y -= size*1.5 + l.Gap
		font := "F1"
		if l.Bold {
			font = "F2"
		}
		fmt.Fprintf(&content, "BT /%s %.1f Tf 0 0 0 rg 1 0 0 1 50 %.1f Tm (%s) Tj ET\n", font, size, y, esc(l.Text))
	}
	fmt.Fprintf(&content, "BT /F1 8 Tf 0.4 0.4 0.4 rg 1 0 0 1 50 40 Tm (%s) Tj ET\n", esc("SAMPLE document generated for Child Connect testing. Not a real document. "+title))

	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 4 0 R /F2 5 0 R >> >> /Contents 6 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", content.Len(), content.String()),
		fmt.Sprintf("<< /Title (%s) /Producer (Child Connect sample generator) >>", esc(title)),
	}
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n%\xE2\xE3\xCF\xD3\n")
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R /Info 7 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return out.Bytes()
}

func H(s string) Line              { return Line{Text: s, Size: 16, Bold: true} }
func T(s string) Line              { return Line{Text: s} }
func B(s string) Line              { return Line{Text: s, Bold: true} }
func Gap(s string, g float64) Line { return Line{Text: s, Gap: g} }
