package main

import (
	"strings"
	"testing"

	"maintainerd/refparse"

	"github.com/stretchr/testify/require"
)

func TestExtractGitHubHandlesFromMarkdownTable(t *testing.T) {
	refBody := `# MAINTAINERS.md

## Overview

This document contains a list of maintainers in this repo. See [RESPONSIBILITIES.md](https://example.com/RESPONSIBILITIES.md#maintainer-responsibilities) that explains what the role of maintainer means, what maintainers do in this and other repos, and how they should be doing it. If you're interested in contributing, and becoming a maintainer, see CONTRIBUTING.md.

## Current Maintainers

| Maintainer | GitHub ID | Affiliation |
|--- |--- |--- |
| Alex Hart | md-test-alexh | Northwind |
| Bailey Reed | md-test-bailey-r | Northwind |
| Casey Lin | md-test-casey-lin | Northwind |
| Devon Park | md-test-devonpark | Northwind |
| Ellis Moore | md-test-ellis-m | Northwind |
| Frankie Tate | md-test-frankie-t | Contoso |
| Gray Morgan | md-test-gray-m | Contoso |
| Harper Quinn | md-test-harper-q | Contoso |
| Indigo Shaw | md-test-indigo-s | Contoso |
| Jules Rivera | md-test-jules-r | Fabrikam |
| Kai Patel | md-test-kaip | Fabrikam |
| Lane Parker | md-test-lane-p | Fabrikam |
| Morgan Lee | md-test-morgan-l | Fabrikam |
| Noel Kim | md-test-noel-k | Globex |
| Oakley Cruz | md-test-oakley-c | Initech |`

	handles := refparse.ExtractGitHubHandles(refBody)
	require.Contains(t, handles, "md-test-oakley-c")
	require.Contains(t, handles, "md-test-alexh")
	for handle := range handles {
		require.True(t, isValidGitHubHandle(handle), "invalid GitHub handle: %q", handle)
		require.True(t, strings.HasPrefix(handle, "md-test-"), "missing md-test- prefix: %q", handle)
	}
}

func isValidGitHubHandle(handle string) bool {
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
