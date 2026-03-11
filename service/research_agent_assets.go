package service

import (
	"embed"
	"strings"
)

//go:embed assets/skills/agentopt-repo-discovery/SKILL.md
var researchAgentAssetFS embed.FS

var bundledRepoDiscoverySkill = mustReadResearchAgentAsset("assets/skills/agentopt-repo-discovery/SKILL.md")

func mustReadResearchAgentAsset(path string) string {
	data, err := researchAgentAssetFS.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return ensureTrailingNewline(string(data))
}

func ensureTrailingNewline(value string) string {
	if strings.HasSuffix(value, "\n") {
		return value
	}
	return value + "\n"
}
