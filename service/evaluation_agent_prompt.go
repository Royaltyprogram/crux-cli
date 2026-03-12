package service

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed prompts/evaluation_agent_prompt.md
var evaluationAgentPromptFS embed.FS

var evaluationAgentPromptTemplate = template.Must(template.New("evaluation_agent_prompt.md").ParseFS(
	evaluationAgentPromptFS,
	"prompts/evaluation_agent_prompt.md",
))

type evaluationAgentPromptData struct {
	ProjectName    string
	NumericSummary string
	BeforeQueries  string
	AfterQueries   string
}

func renderEvaluationAgentPrompt(project *Project, numericSummary, beforeQueries, afterQueries string) (string, error) {
	data := evaluationAgentPromptData{
		NumericSummary: strings.TrimSpace(numericSummary),
		BeforeQueries:  strings.TrimSpace(beforeQueries),
		AfterQueries:   strings.TrimSpace(afterQueries),
	}
	if project != nil {
		data.ProjectName = strings.TrimSpace(project.Name)
	}

	var buf bytes.Buffer
	if err := evaluationAgentPromptTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render evaluation agent prompt: %w", err)
	}
	return strings.TrimSpace(buf.String()), nil
}
