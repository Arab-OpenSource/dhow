package docscheck_test

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

type link struct {
	line   int
	target string
}

type page struct {
	headings map[int]int
	slugs    map[string]bool
	blocks   []string
	tables   []int
	links    []link
	first    string
}

var (
	headingLine = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)
	inlineCode  = regexp.MustCompile("`+[^`]*`+")
	linkPattern = regexp.MustCompile(`!?\[([^\]]*)\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
)

func isFence(line string) bool {
	trimmed := strings.TrimLeft(line, " ")
	return strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
}

func slugOf(heading string) string {
	text := linkPattern.ReplaceAllString(heading, "$1")
	text = strings.ToLower(strings.ReplaceAll(text, "`", ""))
	var out strings.Builder
	for _, r := range text {
		switch {
		case r == ' ':
			out.WriteRune('-')
		case r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r):
			out.WriteRune(r)
		}
	}
	return out.String()
}

func addSlug(slugs map[string]bool, counts map[string]int, heading string) {
	base := slugOf(heading)
	slug := base
	if n := counts[base]; n > 0 {
		slug = fmt.Sprintf("%s-%d", base, n)
	}
	counts[base]++
	slugs[slug] = true
}

func parsePage(text string) page {
	p := page{headings: map[int]int{}, slugs: map[string]bool{}}
	counts := map[string]int{}
	lines := strings.Split(text, "\n")
	if len(lines) > 0 {
		p.first = lines[0]
	}
	var block []string
	inTable := false
	for i, line := range lines {
		if block != nil {
			block = append(block, line)
			if isFence(line) && strings.Trim(strings.TrimSpace(line), "`~") == "" {
				p.blocks = append(p.blocks, strings.Join(block, "\n"))
				block = nil
			}
			continue
		}
		if isFence(line) {
			block = []string{line}
			inTable = false
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "|") {
			if !inTable {
				p.tables = append(p.tables, 0)
				inTable = true
			}
			p.tables[len(p.tables)-1]++
		} else {
			inTable = false
		}
		if m := headingLine.FindStringSubmatch(line); m != nil {
			p.headings[len(m[1])]++
			addSlug(p.slugs, counts, m[2])
		}
		for _, m := range linkPattern.FindAllStringSubmatch(inlineCode.ReplaceAllString(line, ""), -1) {
			p.links = append(p.links, link{line: i + 1, target: m[2]})
		}
	}
	if block != nil {
		p.blocks = append(p.blocks, strings.Join(block, "\n"))
	}
	return p
}
