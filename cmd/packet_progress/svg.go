package main

import (
	"fmt"
	"html"
	"math"
	"strconv"
	"strings"
	"time"
)

// The card follows the client's white panels (home screen HQ pop-over,
// announcements): a cool white wash, condensed display type, a coloured tick
// before each title and the red diamond the client puts on what needs
// attention.
const (
	cardWidth  = 960
	padLeft    = 40
	padRight   = 48
	padY       = 40
	sectionGap = 28
	gridGap    = 12
	tileHeight = 76

	colTime     = 64
	colModule   = 84
	colCommands = 330
)

var (
	colorInk    = "#14171C"
	colorMuted  = "#6B7686"
	colorFaint  = "#8A95A5"
	colorLine   = "#DCE3EE"
	colorTrack  = "#E3E8F0"
	colorHead   = "#EAF0F8"
	colorAlert  = "#D9534F"
	colorIdle   = "#C3CCD8"
	colorAccent = "#16A7FF"

	statusColors = map[status]string{
		statusStateful: colorAccent,
		statusStatic:   "#34558A",
		statusStub:     "#D9A21B",
		statusMissing:  colorTrack,
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

// font families: display (condensed capitals) and body
const (
	display = "d"
	body    = "b"
)

const fonts = `@import url('https://fonts.googleapis.com/css2?family=Bebas+Neue&amp;family=Noto+Sans:wght@400;600;700&amp;display=swap');
.d{font-family:'Bebas Neue','Oswald','DejaVu Sans Condensed','Arial Narrow',sans-serif}
.b{font-family:'Noto Sans','DejaVu Sans','Segoe UI',Arial,sans-serif}`

type svg struct {
	strings.Builder
}

func (s *svg) printf(format string, args ...any) {
	fmt.Fprintf(s, format, args...)
}

// text draws content at baseline y; display text is set in capitals.
func (s *svg) text(x, y float64, family string, size float64, weight int, color, anchor, content string) {
	if family == display {
		content = strings.ToUpper(content)
	}
	s.printf(`<text class="%s" x="%.1f" y="%.1f" font-size="%g" font-weight="%d" fill="%s" text-anchor="%s">%s</text>`+"\n",
		family, x, y, size, weight, color, anchor, html.EscapeString(content))
}

// width estimates the advance of content: there is no text measurement in an
// SVG, the layout only needs to be close.
func width(content, family string, size float64) float64 {
	per := 0.55
	if family == display {
		per = 0.37
	}
	w := 0.0
	for _, r := range content {
		if r == ' ' {
			w += 0.26
		} else {
			w += per
		}
	}
	return w * size
}

// truncate shortens content to fit max, with an ellipsis.
func truncate(content, family string, size, max float64) string {
	if width(content, family, size) <= max {
		return content
	}
	runes := []rune(content)
	for len(runes) > 1 && width(string(runes)+"…", family, size) > max {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

// wrap breaks content into lines of at most max wide.
func wrap(content, family string, size, max float64) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(content) {
		if line != "" && width(line+" "+word, family, size) > max {
			lines = append(lines, line)
			line = word
			continue
		}
		if line != "" {
			line += " "
		}
		line += word
	}
	return append(lines, line)
}

// bar draws a progress bar split by status over the missing track.
func (s *svg) bar(x, y, w, h float64, counts map[status]int, total int) {
	s.printf(`<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="%s"/>`+"\n", x, y, w, h, colorTrack)
	offset := x
	for _, st := range statuses[:3] {
		if total == 0 || counts[st] == 0 {
			continue
		}
		part := w * float64(counts[st]) / float64(total)
		s.printf(`<rect x="%.2f" y="%.1f" width="%.2f" height="%.1f" fill="%s"/>`+"\n", offset, y, part, h, statusColors[st])
		offset += part
	}
}

// pill draws a "+N" tag with its right edge at x, its top at y.
func (s *svg) pill(right, top, size float64, label, fill, border string) {
	w := width(label, display, size) + 12
	h := size*1.1 + 2
	s.printf(`<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="3" fill="%s" stroke="%s" stroke-width="1.5"/>`+"\n", right-w, top, w, h, fill, border)
	s.text(right-w/2, top+h/2+size*0.35, display, size, 400, "#FFFFFF", "middle", label)
}

// diamond draws a badge centred on cx, cy: "!" or a check mark.
func (s *svg) diamond(cx, cy, size float64, fill, border, mark string) {
	s.printf(`<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="%s" transform="rotate(45 %.1f %.1f)"`, cx-size/2, cy-size/2, size, size, fill, cx, cy)
	if border != "" {
		s.printf(` stroke="%s" stroke-width="1.5" filter="url(#lift)"`, border)
	}
	s.printf("/>\n")
	if mark == "!" {
		s.text(cx, cy+size*0.23, body, math.Round(size*0.65), 700, "#FFFFFF", "middle", "!")
		return
	}
	scale := size * 0.6 / 10
	s.printf(`<path d="M1.5 5.2 4 7.6 8.5 2.6" transform="translate(%.1f %.1f) scale(%.2f)" fill="none" stroke="#FFFFFF" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>`+"\n",
		cx-5*scale, cy-5*scale, scale)
}

func percent(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return 100 * float64(n) / float64(total)
}

// chip is a CS id in the latest changes: solid for a new handler, outlined in
// its former status for one answering better than before.
type chip struct {
	label string
	was   status
	x, y  float64
	w     float64
}

const chipSize = 15

// row lays out one commit of the latest changes from its top.
type row struct {
	*commit
	lines  []string
	chips  []chip
	height float64
}

func layoutRow(c *commit, changeWidth float64) row {
	r := row{commit: c, lines: wrap(c.Summary, body, 13, changeWidth)}
	x, y := 0.0, 0.0
	for _, ch := range c.Changes {
		label := strconv.Itoa(int(ch.ID))
		w := width(label, display, chipSize) + 14
		if x > 0 && x+w > colCommands-24 {
			x, y = 0, y+24
		}
		r.chips = append(r.chips, chip{label: label, was: ch.Was, x: x, y: y, w: w})
		x += w + 4
	}
	content := max(30, 18*float64(len(r.lines))+16, y+20)
	r.height = content + 18
	return r
}

func commitTime(t time.Time, since string) string {
	if strings.HasSuffix(since, "h") {
		return t.Local().Format("15:04")
	}
	return t.Local().Format("Jan 2")
}

func renderSVG(rep *report) string {
	inner := float64(cardWidth - padLeft - padRight)
	right := float64(cardWidth - padRight)
	answered := rep.Total - rep.Counts[statusMissing]
	h := rep.History
	recent := h != nil && (h.Gained > 0 || len(h.Commits) > 0)

	out := &svg{}
	y := float64(padY)

	// header: the name on the left, the share answered on the right
	out.printf(`<rect x="%d" y="%.1f" width="5" height="97" rx="2" fill="%s"/>`+"\n", padLeft, y, colorAccent)
	out.text(padLeft+19, y+52, display, 64, 400, colorInk, "start", "Belfast")
	out.text(padLeft+19, y+75, body, 14, 700, colorInk, "start", "Implementation progress")
	meta := rep.Generated
	if rep.Revision != "" {
		meta += ", revision " + rep.Revision
	}
	out.text(padLeft+19, y+93, body, 12, 400, colorMuted, "start", meta)
	out.text(right, y+58, display, 72, 400, colorInk, "end", fmt.Sprintf("%.1f%%", percent(answered, rep.Total)))
	out.text(right, y+80, body, 13, 600, colorMuted, "end", fmt.Sprintf("%d of %d commands", answered, rep.Total))
	if recent {
		label := "in the last " + h.Since
		out.text(right, y+99, body, 12, 600, colorMuted, "end", label)
		out.pill(right-width(label, body, 12)-6, y+86, 15, fmt.Sprintf("+%d", h.Gained), colorAccent, colorAccent)
	}
	y += 103 + sectionGap

	// overall bar under a black rule, the tab pointer marking how far it got
	out.printf(`<rect x="%d" y="%.1f" width="%.1f" height="1.5" fill="%s"/>`+"\n", padLeft, y, inner, colorInk)
	out.bar(padLeft, y+1.5, inner, 12, rep.Counts, rep.Total)
	tip := padLeft + inner*float64(answered)/float64(max(rep.Total, 1))
	out.printf(`<path d="M%.1f %.1f h14 l-7 8 z" fill="%s"/>`+"\n", tip-7, y+15.5, colorAccent)
	y += 23.5 + sectionGap - 12

	// legend
	column := (inner - 3*16) / 4
	for i, st := range statuses {
		x := padLeft + float64(i)*(column+16)
		out.printf(`<rect x="%.1f" y="%.1f" width="12" height="12" fill="%s" stroke="%s" stroke-opacity="0.12"/>`+"\n", x, y+4, statusColors[st], colorInk)
		out.text(x+22, y+14, body, 13, 700, colorInk, "start", statusLabels[st])
		n := strconv.Itoa(rep.Counts[st])
		out.text(x+22, y+50, display, 34, 400, colorInk, "start", n)
		out.text(x+22+width(n, display, 34)+6, y+50, body, 12, 600, colorMuted, "start", fmt.Sprintf("%.0f%%", percent(rep.Counts[st], rep.Total)))
		out.text(x+22, y+66, body, 11, 400, colorMuted, "start", statusHints[st])
	}
	y += 67 + sectionGap - 10

	// what the tile badges mean
	out.printf(`<rect x="%d" y="%.1f" width="%.1f" height="1" fill="%s"/>`+"\n", padLeft, y, inner, colorLine)
	keyY := y + 21
	x := float64(padLeft)
	keys := []struct{ badge, label string }{{"!", "Nothing answered yet"}}
	if recent {
		keys = append(keys, struct{ badge, label string }{"+N", "Commands answered in the last " + h.Since})
	}
	keys = append(keys, struct{ badge, label string }{"check", "Every command answered"})
	for _, key := range keys {
		switch key.badge {
		case "!":
			out.diamond(x+7, keyY, 14*0.72, colorAlert, "", "!")
			x += 14
		case "check":
			out.diamond(x+7, keyY, 14*0.72, statusColors[statusStatic], "", "check")
			x += 14
		default:
			w := width(key.badge, display, 14) + 10
			out.printf(`<rect x="%.1f" y="%.1f" width="%.1f" height="17" rx="3" fill="%s"/>`+"\n", x, keyY-8.5, w, colorAccent)
			out.text(x+w/2, keyY+5, display, 14, 400, "#FFFFFF", "middle", key.badge)
			x += w
		}
		out.text(x+8, keyY+4.5, body, 12, 600, colorMuted, "start", key.label)
		x += 8 + width(key.label, body, 12) + 28
	}
	y += 29 + sectionGap

	// one HQ-style card per protocol module
	tileWidth := (inner - 3*gridGap) / 4
	for i, m := range rep.Modules {
		tx := padLeft + float64(i%4)*(tileWidth+gridGap)
		ty := y + float64(i/4)*(tileHeight+gridGap)
		done := m.Total - m.Counts[statusMissing]
		tick := colorIdle
		for _, st := range statuses[:3] {
			if m.Counts[st] > 0 {
				tick = statusColors[st]
				break
			}
		}
		count := fmt.Sprintf("%d/%d", done, m.Total)
		countColor := colorInk
		if done == 0 {
			countColor = colorFaint
		}
		label := m.Label
		if label == "" {
			label = "—"
		}
		out.printf(`<rect x="%.1f" y="%.1f" width="%.1f" height="%d" rx="10" fill="url(#tile)" stroke="%s" filter="url(#lift)"/>`+"\n", tx, ty, tileWidth, tileHeight, colorLine)
		out.printf(`<rect x="%.1f" y="%.1f" width="4" height="22" rx="2" fill="%s"/>`+"\n", tx+12, ty+16, tick)
		countWidth := width(count, display, 20)
		out.text(tx+24, ty+33, display, 24, 400, colorInk, "start", truncate(strings.ToUpper(label), display, 24, tileWidth-24-12-countWidth-8))
		out.text(tx+24, ty+45, body, 9, 600, colorFaint, "start", fmt.Sprintf("P%d", m.ID))
		out.text(tx+tileWidth-12, ty+34, display, 20, 400, countColor, "end", count)
		out.bar(tx+12, ty+tileHeight-17, tileWidth-24, 5, m.Counts, m.Total)

		// one badge, attention first, then done, then recent progress
		switch {
		case done == 0:
			out.diamond(tx+tileWidth-3, ty+3, 20*0.72, colorAlert, "#FFFFFF", "!")
		case done == m.Total:
			out.diamond(tx+tileWidth-3, ty+3, 20*0.72, statusColors[statusStatic], "#FFFFFF", "check")
		case recent && h.Gains[m.ID] > 0:
			out.pill(tx+tileWidth+6, ty-8, 15, fmt.Sprintf("+%d", h.Gains[m.ID]), colorAccent, "#FFFFFF")
		}
	}
	rows := (len(rep.Modules) + 3) / 4
	y += float64(rows)*(tileHeight+gridGap) - gridGap

	// latest changes, one row per commit and module
	if recent && len(h.Commits) > 0 {
		y += sectionGap
		out.printf(`<rect x="%d" y="%.1f" width="4" height="22" rx="2" fill="%s"/>`+"\n", padLeft, y+3, colorAccent)
		out.text(padLeft+14, y+24, display, 28, 400, colorInk, "start", "Latest changes")
		ruleX := padLeft + 14 + width("LATEST CHANGES", display, 28) + 16
		out.printf(`<rect x="%.1f" y="%.1f" width="%.1f" height="1.5" fill="%s"/>`+"\n", ruleX, y+13, right-ruleX, colorInk)
		y += 28 + 10

		colChange := float64(padLeft + colTime + colModule)
		colCmd := right - colCommands
		out.printf(`<rect x="%d" y="%.1f" width="%.1f" height="29" fill="%s"/>`+"\n", padLeft, y, inner, colorHead)
		for _, head := range []struct {
			x     float64
			label string
		}{{padLeft, "Time"}, {padLeft + colTime, "Module"}, {colChange, "Change"}, {colCmd, "Commands answered"}} {
			out.text(head.x+12, y+19, body, 11, 700, colorMuted, "start", head.label)
		}
		y += 29
		for _, c := range h.Commits {
			r := layoutRow(c, colCmd-colChange-24)
			top := y + 9
			out.text(padLeft+12, top+13, body, 13, 600, colorMuted, "start", commitTime(c.Time, h.Since))
			out.text(padLeft+colTime+12, top+15, display, 18, 400, colorInk, "start", truncate(strings.ToUpper(c.Label), display, 18, colModule-16))
			out.text(padLeft+colTime+12, top+27, body, 9, 600, colorFaint, "start", fmt.Sprintf("P%d", c.Module))
			for i, line := range r.lines {
				out.text(colChange+12, top+13+float64(i)*18, body, 13, 600, colorInk, "start", line)
			}
			out.text(colChange+12, top+13+float64(len(r.lines))*18, body, 11, 400, colorFaint, "start", c.Rev)
			for _, ch := range r.chips {
				fill, border, ink := colorAccent, colorAccent, "#FFFFFF"
				if ch.was != statusMissing {
					fill, border, ink = "#FFFFFF", statusColors[ch.was], colorInk
				}
				cx, cy := colCmd+12+ch.x, top+ch.y
				out.printf(`<rect x="%.1f" y="%.1f" width="%.1f" height="20" rx="3" fill="%s" stroke="%s"><title>CS_%s</title></rect>`+"\n", cx, cy, ch.w, fill, border, ch.label)
				out.text(cx+ch.w/2, cy+15.5, display, chipSize, 400, ink, "middle", ch.label)
			}
			y += r.height
			out.printf(`<rect x="%d" y="%.1f" width="%.1f" height="1" fill="%s"/>`+"\n", padLeft, y-1, inner, colorTrack)
		}

		// chip key, and how many commits did not fit
		y += 10
		x := float64(padLeft)
		for _, key := range []struct {
			fill, border, ink, label string
		}{
			{colorAccent, colorAccent, "#FFFFFF", "new handler"},
			{"#FFFFFF", statusColors[statusStub], colorInk, "answered better than before (outlined in its former status)"},
		} {
			w := width("CS ID", display, 13) + 10
			out.printf(`<rect x="%.1f" y="%.1f" width="%.1f" height="17" rx="3" fill="%s" stroke="%s"/>`+"\n", x, y, w, key.fill, key.border)
			out.text(x+w/2, y+13, display, 13, 400, key.ink, "middle", "CS id")
			out.text(x+w+8, y+13, body, 11, 600, colorMuted, "start", key.label)
			x += w + 8 + width(key.label, body, 11) + 20
		}
		if h.More > 0 {
			out.text(right, y+13, body, 11, 600, colorMuted, "end", fmt.Sprintf("%d more commits not shown", h.More))
		}
		y += 17
	}
	height := y + padY

	var doc svg
	doc.printf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%.0f" viewBox="0 0 %d %.0f">`+"\n", cardWidth, height, cardWidth, height)
	doc.printf("<defs>\n<style>%s</style>\n", fonts)
	doc.printf(`<radialGradient id="ground" gradientUnits="userSpaceOnUse" cx="0" cy="0" r="1" gradientTransform="scale(%d %.0f)"><stop offset="0" stop-color="#E3ECF8"/><stop offset="0.45" stop-color="#F4F7FC"/><stop offset="0.75" stop-color="#FFFFFF"/></radialGradient>`+"\n", cardWidth*6/5, height*0.7)
	doc.printf(`<linearGradient id="tile" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#FFFFFF"/><stop offset="1" stop-color="#F1F5FB"/></linearGradient>` + "\n")
	doc.printf(`<filter id="lift" x="-10%%" y="-20%%" width="120%%" height="150%%"><feDropShadow dx="0" dy="2" stdDeviation="3" flood-color="#34558A" flood-opacity="0.14"/></filter>` + "\n")
	doc.printf("</defs>\n")
	doc.printf(`<rect width="%d" height="%.0f" rx="18" fill="url(#ground)"/>`+"\n", cardWidth, height)
	doc.WriteString(out.String())
	doc.printf("</svg>\n")
	return doc.String()
}
