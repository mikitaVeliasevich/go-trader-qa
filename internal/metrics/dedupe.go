package metrics

import "strings"

// AppendNewLogLines appends lines from tail that are not already present at the
// end of existing. If the entire tail block duplicates a suffix of existing,
// existing is returned unchanged.
func AppendNewLogLines(existing, tail string) string {
	if tail == "" {
		return existing
	}
	if existing == "" {
		return tail
	}
	if strings.HasSuffix(existing, tail) {
		return existing
	}

	existingLines := splitLinesPreserveTrailingNewline(existing)
	tailLines := splitLinesPreserveTrailingNewline(tail)

	maxOverlap := 0
	for i := 1; i <= len(existingLines) && i <= len(tailLines); i++ {
		match := true
		for j := 0; j < i; j++ {
			if existingLines[len(existingLines)-i+j] != tailLines[j] {
				match = false
				break
			}
		}
		if match {
			maxOverlap = i
		}
	}

	newLines := tailLines[maxOverlap:]
	if len(newLines) == 0 {
		return existing
	}

	sep := ""
	if !strings.HasSuffix(existing, "\n") {
		sep = "\n"
	}
	return existing + sep + strings.Join(newLines, "\n")
}

func splitLinesPreserveTrailingNewline(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	if strings.HasSuffix(text, "\n") {
		lines = lines[:len(lines)-1]
	}
	return lines
}
