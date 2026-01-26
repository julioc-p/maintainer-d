package refparse

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ExtractGitHubHandles scans maintainer ref content and returns a set of detected GitHub handles.
func ExtractGitHubHandles(refBody string) map[string]struct{} {
	result := make(map[string]struct{})
	if refBody == "" {
		return result
	}
	parseMarkdownTablesForHandles(refBody, result)
	// Match @username
	atRe := regexp.MustCompile(`(?i)(^|[^a-z0-9_-])@([a-z0-9-]{1,39})`)
	for _, match := range atRe.FindAllStringSubmatch(refBody, -1) {
		if len(match) < 3 {
			continue
		}
		handle := strings.ToLower(match[2])
		result[handle] = struct{}{}
	}
	// Match github.com/username (filter out repo paths like github.com/org/repo)
	urlRe := regexp.MustCompile(`(?i)github\.com/([a-z0-9-]{1,39})`)
	for _, match := range urlRe.FindAllStringSubmatchIndex(refBody, -1) {
		if len(match) < 4 {
			continue
		}
		handle := strings.ToLower(refBody[match[2]:match[3]])
		if handle == "organizations" || handle == "orgs" || handle == "repos" {
			continue
		}
		// If the matched handle is followed by a path segment, skip it.
		if match[1] < len(refBody) && refBody[match[1]] == '/' {
			continue
		}
		result[handle] = struct{}{}
	}
	// Match YAML list items like "- username"
	listItemRe := regexp.MustCompile(`(?i)^\s*[-*]\s*([a-z0-9][a-z0-9-]{0,38})\b`)
	// Match YAML key "github: username"
	keyRe := regexp.MustCompile(`(?i)^\s*github\s*:\s*([a-z0-9][a-z0-9-]{0,38})\b`)
	for _, line := range strings.Split(refBody, "\n") {
		if match := listItemRe.FindStringSubmatch(line); len(match) > 1 {
			handle := strings.ToLower(match[1])
			if handle != "organizations" && handle != "orgs" && handle != "repos" {
				result[handle] = struct{}{}
			}
		}
		if match := keyRe.FindStringSubmatch(line); len(match) > 1 {
			handle := strings.ToLower(match[1])
			if handle != "organizations" && handle != "orgs" && handle != "repos" {
				result[handle] = struct{}{}
			}
		}
	}
	return result
}

func parseMarkdownTablesForHandles(refBody string, result map[string]struct{}) {
	lines := strings.Split(refBody, "\n")
	if len(lines) < 2 {
		return
	}
	headerMatch := func(header string) bool {
		normalized := strings.ToLower(strings.TrimSpace(header))
		switch normalized {
		case "github", "github id", "github username", "github handle", "github account":
			return true
		}
		return false
	}
	isSeparatorRow := func(cells []string) bool {
		if len(cells) == 0 {
			return false
		}
		for _, cell := range cells {
			trimmed := strings.TrimSpace(cell)
			if trimmed == "" {
				continue
			}
			for _, ch := range trimmed {
				if ch != '-' && ch != ':' {
					return false
				}
			}
		}
		return true
	}
	parseRow := func(line string) []string {
		if !strings.Contains(line, "|") {
			return nil
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			return nil
		}
		trimmed = strings.TrimPrefix(trimmed, "|")
		trimmed = strings.TrimSuffix(trimmed, "|")
		parts := strings.Split(trimmed, "|")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return parts
	}
	isValidHandle := func(handle string) bool {
		handle = strings.ToLower(strings.TrimSpace(handle))
		if handle == "" || handle == "organizations" || handle == "orgs" || handle == "repos" {
			return false
		}
		if len(handle) > 39 {
			return false
		}
		for i, r := range handle {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
				continue
			}
			if r == '_' && i == 0 {
				return false
			}
			return false
		}
		return true
	}
	for i := 0; i+1 < len(lines); i++ {
		headerCells := parseRow(lines[i])
		if len(headerCells) == 0 {
			continue
		}
		separatorCells := parseRow(lines[i+1])
		if len(separatorCells) == 0 || !isSeparatorRow(separatorCells) {
			continue
		}
		githubIndex := -1
		for idx, cell := range headerCells {
			if headerMatch(cell) {
				githubIndex = idx
				break
			}
		}
		if githubIndex < 0 {
			continue
		}
		for row := i + 2; row < len(lines); row++ {
			rowCells := parseRow(lines[row])
			if len(rowCells) == 0 {
				break
			}
			if isSeparatorRow(rowCells) {
				break
			}
			if githubIndex >= len(rowCells) {
				continue
			}
			cell := strings.TrimSpace(rowCells[githubIndex])
			if cell == "" {
				continue
			}
			cell = strings.Trim(cell, "`")
			cell = strings.TrimPrefix(cell, "@")
			if !isValidHandle(cell) {
				continue
			}
			result[strings.ToLower(cell)] = struct{}{}
		}
		i++
	}
}

// MaintainerRefContains checks if the maintainer ref contains a handle (case-insensitive) with word boundaries.
func MaintainerRefContains(refBody, handle string) (bool, error) {
	if handle == "" {
		return false, errors.New("handle is empty")
	}
	escaped := regexp.QuoteMeta(handle)
	pattern := fmt.Sprintf(`(?i)(^|[^a-z0-9_-])@?%s([^a-z0-9_-]|$)`, escaped)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, err
	}
	return re.MatchString(refBody), nil
}
