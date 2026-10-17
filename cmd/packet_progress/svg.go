package main

import (
	"fmt"
	"html"
	"strings"
)

const (
	cardWidth   = 960
	cardPadding = 40
	gridColumns = 4
	gridGap     = 14
	tileHeight  = 58
)

// palette, GitHub dark
var (
	colorBackground = "#0d1117"
	colorTile       = "#161b22"
	colorBorder     = "#30363d"
	colorText       = "#e6edf3"
	colorMuted      = "#8b949e"
	statusColors    = map[status]string{
		statusStateful: "#3fb950",
		statusStatic:   "#58a6ff",
		statusStub:     "#d29922",
		statusMissing:  "#21262d",
	}
	statusLabels = map[status]string{
		statusStateful: "Stateful",
		statusStatic:   "Static",
		statusStub:     "Stub",
		statusMissing:  "Missing",
	}
	statusHints = map[status]string{
		statusStateful: "reads or writes saved state",
		statusStatic:   "game config, nothing saved",
		statusStub:     "constant, captured or ignored",
		statusMissing:  "no handler",
	}
)

type svg struct {
	strings.Builder
	clips int
}

func (s *svg) printf(format string, args ...any) {
	fmt.Fprintf(s, format, args...)
}

func (s *svg) text(x, y float64, size int, weight int, color, anchor, content string) {
	s.printf(`<text x="%.1f" y="%.1f" font-size="%d" font-weight="%d" fill="%s" text-anchor="%s">%s</text>`+"\n",
		x, y, size, weight, color, anchor, html.EscapeString(content))
}

// bar draws a rounded progress bar split by status.
func (s *svg) bar(x, y, width, height float64, counts map[status]int, total int) {
	s.clips++
	id := fmt.Sprintf("bar%d", s.clips)
	radius := height / 2
	s.printf(`<clipPath id="%s"><rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="%.1f"/></clipPath>`+"\n", id, x, y, width, height, radius)
	s.printf(`<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="%.1f" fill="%s"/>`+"\n", x, y, width, height, radius, statusColors[statusMissing])
	s.printf(`<g clip-path="url(#%s)">`+"\n", id)
	offset := x
	for _, st := range statuses[:3] {
		if total == 0 || counts[st] == 0 {
			continue
		}
		w := width * float64(counts[st]) / float64(total)
		s.printf(`<rect x="%.2f" y="%.1f" width="%.2f" height="%.1f" fill="%s"/>`+"\n", offset, y, w, height, statusColors[st])
		offset += w
	}
	s.printf("</g>\n")
}

func percent(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return 100 * float64(n) / float64(total)
}

func renderSVG(rep *report) string {
	inner := float64(cardWidth - 2*cardPadding)
	rows := (len(rep.Modules) + gridColumns - 1) / gridColumns
	gridTop := 268.0
	height := gridTop + float64(rows)*(tileHeight+gridGap) - gridGap + float64(cardPadding)

	var s svg
	s.printf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%.0f" viewBox="0 0 %d %.0f" font-family="Inter, 'Segoe UI', 'Helvetica Neue', Arial, sans-serif">`+"\n",
		cardWidth, height, cardWidth, height)
	s.printf(`<rect width="100%%" height="100%%" rx="16" fill="%s" stroke="%s"/>`+"\n", colorBackground, colorBorder)

	// header
	answered := rep.Total - rep.Counts[statusMissing]
	s.text(cardPadding, 62, 28, 700, colorText, "start", "Belfast")
	meta := fmt.Sprintf("Client commands answered · %s", rep.Generated)
	if rep.Revision != "" {
		meta += " · " + rep.Revision
	}
	s.text(cardPadding, 88, 14, 400, colorMuted, "start", meta)
	s.text(cardWidth-cardPadding, 66, 40, 700, colorText, "end", fmt.Sprintf("%.1f%%", percent(answered, rep.Total)))
	s.text(cardWidth-cardPadding, 88, 14, 400, colorMuted, "end", fmt.Sprintf("%d of %d", answered, rep.Total))

	s.bar(cardPadding, 112, inner, 16, rep.Counts, rep.Total)

	// legend
	column := inner / float64(len(statuses))
	for i, st := range statuses {
		x := cardPadding + float64(i)*column
		s.printf(`<rect x="%.1f" y="158" width="12" height="12" rx="3" fill="%s" stroke="%s"/>`+"\n", x, statusColors[st], colorBorder)
		s.text(x+20, 169, 14, 600, colorText, "start", statusLabels[st])
		s.text(x+20, 194, 22, 700, colorText, "start", fmt.Sprintf("%d", rep.Counts[st]))
		s.text(x+20+float64(len(fmt.Sprint(rep.Counts[st])))*13+8, 194, 13, 400, colorMuted, "start", fmt.Sprintf("%.0f%%", percent(rep.Counts[st], rep.Total)))
		s.text(x+20, 214, 12, 400, colorMuted, "start", statusHints[st])
	}

	s.printf(`<line x1="%d" y1="238" x2="%d" y2="238" stroke="%s"/>`+"\n", cardPadding, cardWidth-cardPadding, colorBorder)

	// one tile per protocol module
	tileWidth := (inner - float64(gridColumns-1)*gridGap) / gridColumns
	for i, m := range rep.Modules {
		x := cardPadding + float64(i%gridColumns)*(tileWidth+gridGap)
		y := gridTop + float64(i/gridColumns)*(tileHeight+gridGap)
		s.printf(`<rect x="%.1f" y="%.1f" width="%.1f" height="%d" rx="10" fill="%s" stroke="%s"/>`+"\n", x, y, tileWidth, tileHeight, colorTile, colorBorder)
		s.text(x+14, y+24, 12, 600, colorMuted, "start", fmt.Sprintf("p%d", m.ID))
		label := m.Label
		if label == "" {
			label = "—"
		}
		s.text(x+46, y+24, 14, 600, colorText, "start", label)
		s.text(x+tileWidth-14, y+24, 13, 600, colorText, "end", fmt.Sprintf("%d/%d", m.Total-m.Counts[statusMissing], m.Total))
		s.bar(x+14, y+38, tileWidth-28, 7, m.Counts, m.Total)
	}
	s.printf("</svg>\n")
	return s.String()
}
