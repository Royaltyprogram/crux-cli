package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Royaltyprogram/aiops/configs"
)

func TestCloudEvaluationAgentReviewExperimentUsesOpenAIResponses(t *testing.T) {
	type responsesRequest struct {
		Model string `json:"model"`
		Input string `json:"input"`
	}

	var seen responsesRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1/responses", r.URL.Path)
		require.Equal(t, "Bearer test-openai-key", r.Header.Get("Authorization"))

		require.NoError(t, json.NewDecoder(r.Body).Decode(&seen))
		require.Equal(t, "gpt-5.4", seen.Model)
		require.Contains(t, seen.Input, "Before Rollout Queries")
		require.Contains(t, seen.Input, "After Rollout Queries")
		require.Contains(t, seen.Input, "Inspect the current approval flow before editing.")
		require.Contains(t, seen.Input, "Explain why the rollout feels slower.")

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{
  "output": [
    {
      "type": "message",
      "content": [
        {
          "type": "output_text",
          "text": "{\"decision\":\"rollback\",\"confidence\":\"high\",\"summary\":\"After the rollout the user switches from direct edits to repeated slowdown and diagnosis prompts, which indicates materially worse workflow quality.\"}"
        }
      ]
    }
  ]
}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	agent := NewCloudEvaluationAgent(&configs.Config{
		OpenAI: configs.OpenAI{
			APIKey:         "test-openai-key",
			BaseURL:        server.URL + "/v1",
			ResponsesModel: "gpt-5.4",
		},
	})

	before := []*SessionSummary{{
		Timestamp: time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC),
		RawQueries: []string{
			"Inspect the current approval flow before editing.",
			"List the exact verification steps after the patch.",
		},
	}}
	after := []*SessionSummary{{
		Timestamp: time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC),
		RawQueries: []string{
			"Explain why the rollout feels slower.",
			"Inspect the larger trace after the rollout.",
		},
	}}

	review := agent.ReviewExperiment(&Project{Name: "demo-workspace"}, before, after, summarizeSessions(before), summarizeSessions(after))
	require.Equal(t, "rollback", review.Decision)
	require.Equal(t, "high", review.Confidence)
	require.Contains(t, review.Summary, "materially worse workflow quality")
	require.Equal(t, "openai", review.Provider)
	require.Equal(t, "responses-api", review.Mode)
}
