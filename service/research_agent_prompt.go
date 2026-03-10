package service

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed prompts/research_agent_instruction_prompt.md
var researchAgentPromptFS embed.FS

var researchAgentInstructionPromptTemplate = template.Must(template.New("research_agent_instruction_prompt.md").ParseFS(
	researchAgentPromptFS,
	"prompts/research_agent_instruction_prompt.md",
))

type researchAgentInstructionPromptData struct {
	ProjectName          string
	SampledQueryCount    int
	SampledQueriesPrompt string
	UsageSummaryPrompt   string
	RecentSessionsPrompt string
}

func renderResearchAgentInstructionPrompt(project *Project, sampledQueries []string, usageSummary researchUsageSummary) (string, error) {
	data := researchAgentInstructionPromptData{
		SampledQueryCount:    len(sampledQueries),
		SampledQueriesPrompt: formatSampledQueriesForPrompt(sampledQueries),
		UsageSummaryPrompt:   formatUsageSummaryForPrompt(usageSummary),
		RecentSessionsPrompt: formatRecentSessionsForPrompt(usageSummary.RecentSessions),
	}
	if project != nil {
		data.ProjectName = strings.TrimSpace(project.Name)
	}

	var buf bytes.Buffer
	if err := researchAgentInstructionPromptTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render research agent prompt: %w", err)
	}
	return strings.TrimSpace(buf.String()), nil
}

func formatSampledQueriesForPrompt(sampledQueries []string) string {
	if len(sampledQueries) == 0 {
		return "- none"
	}
	lines := make([]string, 0, len(sampledQueries))
	for idx, query := range sampledQueries {
		lines = append(lines, fmt.Sprintf("sample_query_%d: %s", idx+1, strings.TrimSpace(query)))
	}
	return strings.Join(lines, "\n")
}

func formatUsageSummaryForPrompt(usageSummary researchUsageSummary) string {
	lines := []string{
		fmt.Sprintf("- sessions=%d", usageSummary.SessionCount),
		fmt.Sprintf("- raw_queries=%d", usageSummary.RawQueryCount),
		fmt.Sprintf("- total_input_tokens=%d", usageSummary.TotalInputTokens),
		fmt.Sprintf("- total_output_tokens=%d", usageSummary.TotalOutputTokens),
		fmt.Sprintf("- total_cached_input_tokens=%d", usageSummary.TotalCachedInputTokens),
		fmt.Sprintf("- total_reasoning_output_tokens=%d", usageSummary.TotalReasoningOutputTokens),
		fmt.Sprintf("- avg_tokens_per_query=%d", usageSummary.AvgTokensPerQuery),
		fmt.Sprintf("- avg_first_response_latency_ms=%d", usageSummary.AvgFirstResponseLatencyMS),
		fmt.Sprintf("- avg_session_duration_ms=%d", usageSummary.AvgSessionDurationMS),
		fmt.Sprintf("- total_function_calls=%d", usageSummary.TotalFunctionCalls),
		fmt.Sprintf("- total_tool_errors=%d", usageSummary.TotalToolErrors),
		fmt.Sprintf("- total_tool_wall_time_ms=%d", usageSummary.TotalToolWallTimeMS),
		fmt.Sprintf("- sessions_with_function_calls=%d", usageSummary.SessionsWithFunctionCalls),
		fmt.Sprintf("- sessions_with_tool_errors=%d", usageSummary.SessionsWithToolErrors),
	}
	return strings.Join(lines, "\n")
}

func formatRecentSessionsForPrompt(recentSessions []researchSessionSnapshot) string {
	if len(recentSessions) == 0 {
		return "- none"
	}
	lines := make([]string, 0, len(recentSessions))
	for _, session := range recentSessions {
		lines = append(lines, fmt.Sprintf(
			"- %s | tool=%s | queries=%d | input_tokens=%d | output_tokens=%d | cached_input_tokens=%d | reasoning_output_tokens=%d | first_response_latency_ms=%d | session_duration_ms=%d | function_calls=%d | tool_errors=%d | tool_wall_time_ms=%d",
			session.TimestampLabel,
			session.Tool,
			session.QueryCount,
			session.InputTokens,
			session.OutputTokens,
			session.CachedInputTokens,
			session.ReasoningOutputTokens,
			session.FirstResponseLatencyMS,
			session.SessionDurationMS,
			session.FunctionCallCount,
			session.ToolErrorCount,
			session.ToolWallTimeMS,
		))
	}
	return strings.Join(lines, "\n")
}
