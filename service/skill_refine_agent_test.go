package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSkillRefineResponseValidJSON(t *testing.T) {
	originals := []compiledSkillCategory{
		{
			Category:     "clarification",
			Title:        "Clarify Before Building",
			Rules:        []string{"Try asking for missing constraints before coding."},
			AntiPatterns: []string{"Starts implementation before confirming requirements."},
			Confidence:   0.85,
			ReportIDs:    []string{"rep-1"},
		},
		{
			Category:     "validation",
			Title:        "Validate Before Declaring Done",
			Rules:        []string{"Consider running tests after your changes."},
			AntiPatterns: []string{"Skips validation when under time pressure."},
			Confidence:   0.78,
			ReportIDs:    []string{"rep-2"},
		},
	}

	raw := `[
		{
			"category": "clarification",
			"rules": ["When requirements are incomplete, ask the user for the missing constraints before writing any code."],
			"anti_patterns": ["Do not begin implementation until all stated requirements have been confirmed with the user."]
		},
		{
			"category": "validation",
			"rules": ["After making changes, run the relevant test suite before reporting completion."],
			"anti_patterns": ["Never skip test validation regardless of time constraints."]
		}
	]`

	result, err := parseSkillRefineResponse(raw, originals)
	require.NoError(t, err)
	require.Len(t, result, 2)

	require.Equal(t, "clarification", result[0].Category)
	require.Equal(t, "Clarify Before Building", result[0].Title)
	require.Equal(t, 0.85, result[0].Confidence)
	require.Equal(t, []string{"rep-1"}, result[0].ReportIDs)
	require.Len(t, result[0].Rules, 1)
	require.Contains(t, result[0].Rules[0], "ask the user")

	require.Equal(t, "validation", result[1].Category)
	require.Len(t, result[1].AntiPatterns, 1)
	require.Contains(t, result[1].AntiPatterns[0], "Never skip")
}

func TestParseSkillRefineResponseStripsMarkdownFences(t *testing.T) {
	originals := []compiledSkillCategory{
		{
			Category:     "planning",
			Title:        "Plan Before Touching Code",
			Rules:        []string{"Outline a plan first."},
			AntiPatterns: []string{"Dives into code without a plan."},
		},
	}

	raw := "```json\n" + `[{"category":"planning","rules":["Before writing code, produce a step-by-step plan."],"anti_patterns":["Do not start editing files without an explicit plan."]}]` + "\n```"

	result, err := parseSkillRefineResponse(raw, originals)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, "Before writing code, produce a step-by-step plan.", result[0].Rules[0])
}

func TestParseSkillRefineResponseCountMismatchReturnsError(t *testing.T) {
	originals := []compiledSkillCategory{
		{Category: "clarification", Rules: []string{"rule1"}, AntiPatterns: []string{"anti1"}},
		{Category: "validation", Rules: []string{"rule2"}, AntiPatterns: []string{"anti2"}},
	}

	raw := `[{"category":"clarification","rules":["rewritten"],"anti_patterns":["rewritten"]}]`

	_, err := parseSkillRefineResponse(raw, originals)
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected 2")
}

func TestParseSkillRefineResponseInvalidJSONReturnsError(t *testing.T) {
	originals := []compiledSkillCategory{
		{Category: "clarification", Rules: []string{"rule1"}, AntiPatterns: []string{"anti1"}},
	}

	_, err := parseSkillRefineResponse("not json at all", originals)
	require.Error(t, err)
}

func TestParseSkillRefineResponseRuleCountMismatchKeepsOriginal(t *testing.T) {
	originals := []compiledSkillCategory{
		{
			Category:     "clarification",
			Rules:        []string{"rule1", "rule2"},
			AntiPatterns: []string{"anti1"},
		},
	}

	raw := `[{"category":"clarification","rules":["only one rewritten rule"],"anti_patterns":["rewritten anti"]}]`

	result, err := parseSkillRefineResponse(raw, originals)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, []string{"rule1", "rule2"}, result[0].Rules, "original rules preserved when count mismatches")
	require.Equal(t, []string{"rewritten anti"}, result[0].AntiPatterns, "anti-patterns updated when count matches")
}

func TestRefineCategoriesNilAgentReturnsOriginals(t *testing.T) {
	categories := []compiledSkillCategory{
		{Category: "clarification", Rules: []string{"ask first"}},
	}

	var agent *SkillRefineAgent
	result, err := agent.RefineCategories(context.Background(), categories)
	require.NoError(t, err)
	require.Equal(t, categories, result)
}

func TestRefineCategoriesEmptyAPIKeyReturnsOriginals(t *testing.T) {
	agent := &SkillRefineAgent{apiKey: ""}
	categories := []compiledSkillCategory{
		{Category: "clarification", Rules: []string{"ask first"}},
	}

	result, err := agent.RefineCategories(context.Background(), categories)
	require.NoError(t, err)
	require.Equal(t, categories, result)
}

func TestBuildSkillRefinePromptContainsCategoriesJSON(t *testing.T) {
	categories := []compiledSkillCategory{
		{
			Category:     "planning",
			Title:        "Plan Before Touching Code",
			Rules:        []string{"Outline steps before editing."},
			AntiPatterns: []string{"Jumps straight to code."},
		},
	}

	prompt, err := buildSkillRefinePrompt(categories)
	require.NoError(t, err)
	require.Contains(t, prompt, `"category": "planning"`)
	require.Contains(t, prompt, "Outline steps before editing.")
	require.Contains(t, prompt, "Jumps straight to code.")
	require.Contains(t, prompt, "rewriting behavioral rules")
}

func TestSanitizeRefineLines(t *testing.T) {
	input := []string{"  valid rule  ", "", "  ", "another rule"}
	result := sanitizeRefineLines(input)
	require.Equal(t, []string{"valid rule", "another rule"}, result)
}
