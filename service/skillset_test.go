package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Royaltyprogram/aiops/configs"
	"github.com/Royaltyprogram/aiops/dto/request"
	"github.com/Royaltyprogram/aiops/dto/response"
)

func TestEnsureLatestSkillSetVersionLockedTracksHistoryAndDiff(t *testing.T) {
	store := &AnalyticsStore{
		skillSetVersions: make(map[string][]*SkillSetVersion),
	}
	svc := &AnalyticsService{
		Options: Options{
			AnalyticsStore: store,
		},
	}
	project := &Project{ID: "project-1", OrgID: "org-1"}

	firstBundle, err := buildLatestSkillSetBundle(project.ID, []*Report{{
		ID:             "rep-1",
		ProjectID:      project.ID,
		Title:          "Requirement clarification drift",
		Reason:         "Requirement questions were skipped before implementation.",
		ExpectedImpact: "Fewer premature implementations.",
		Frictions:      []string{"Starts implementation before confirming missing requirements."},
		NextSteps:      []string{"Ask for missing constraints before touching code."},
		Status:         "active",
		CreatedAt:      time.Now().UTC().Add(-time.Hour),
	}})
	require.NoError(t, err)

	firstReports := []*Report{{
		ID:             "rep-1",
		ProjectID:      project.ID,
		Title:          "Requirement clarification drift",
		Reason:         "Requirement questions were skipped before implementation.",
		ExpectedImpact: "Fewer premature implementations.",
		Frictions:      []string{"Starts implementation before confirming missing requirements."},
		NextSteps:      []string{"Ask for missing constraints before touching code."},
		Status:         "active",
		CreatedAt:      time.Now().UTC().Add(-time.Hour),
	}}
	latest, previous, created := svc.ensureLatestSkillSetVersionLocked(project, firstBundle, firstReports)
	require.True(t, created)
	require.NotNil(t, latest)
	require.Nil(t, previous)
	require.Len(t, store.skillSetVersions[project.ID], 1)
	require.Equal(t, "shadow", latest.DeploymentDecision)
	require.NotNil(t, latest.ShadowEvaluation)
	require.Equal(t, "passed", latest.ShadowEvaluation.Guardrail)
	require.Equal(t, 1, latest.ShadowEvaluation.ChangedDocumentCount)
	require.Equal(t, 0, latest.ShadowEvaluation.RuleChurn)
	require.InDelta(t, 0.66, latest.ShadowEvaluation.Score, 0.01)
	require.True(t, hasSkillSetFile(firstBundle.Files, "SKILL.md", func(content string) bool {
		return strings.Contains(content, "---\nname: autoskills-personal-skillset\n") &&
			strings.Contains(content, "description: Use as the default operating skill set for this user across coding sessions") &&
			strings.Contains(content, "## Workflow")
	}))
	require.True(t, hasSkillSetFile(firstBundle.Files, "agents/openai.yaml", func(content string) bool {
		return strings.Contains(content, "display_name: \"AutoSkills Personal Skill Set\"") &&
			strings.Contains(content, "default_prompt: \"Use $autoskills-personal-skillset")
	}))

	secondReports := []*Report{{
		ID:             "rep-2",
		ProjectID:      project.ID,
		Title:          "Requirement clarification drift",
		Reason:         "Requirement questions were skipped before implementation.",
		ExpectedImpact: "Fewer premature implementations.",
		Frictions: []string{
			"Assumes the happy path without confirming constraints.",
		},
		NextSteps: []string{
			"Ask for missing constraints before touching code.",
			"State assumptions explicitly before coding.",
		},
		Status:    "active",
		CreatedAt: time.Now().UTC(),
	}}
	secondBundle, err := buildLatestSkillSetBundle(project.ID, secondReports)
	require.NoError(t, err)

	latest, previous, created = svc.ensureLatestSkillSetVersionLocked(project, secondBundle, secondReports)
	require.True(t, created)
	require.NotNil(t, latest)
	require.NotNil(t, previous)
	require.Equal(t, secondBundle.Version, latest.Version)
	require.Equal(t, firstBundle.Version, previous.Version)
	require.Len(t, store.skillSetVersions[project.ID], 2)
	require.Equal(t, "shadow", latest.DeploymentDecision)
	require.NotNil(t, latest.ShadowEvaluation)
	require.Equal(t, "passed", latest.ShadowEvaluation.Guardrail)
	require.Equal(t, 1, latest.ShadowEvaluation.ChangedDocumentCount)
	require.Equal(t, 2, latest.ShadowEvaluation.AddedRuleCount)
	require.Equal(t, 1, latest.ShadowEvaluation.RemovedRuleCount)
	require.Equal(t, 3, latest.ShadowEvaluation.RuleChurn)
	require.InDelta(t, 0.61, latest.ShadowEvaluation.Score, 0.02)

	diff := buildSkillSetVersionDiffResp(previous, latest)
	require.NotNil(t, diff)
	require.Equal(t, firstBundle.Version, diff.FromVersion)
	require.Equal(t, secondBundle.Version, diff.ToVersion)
	require.Contains(t, diff.Summary, "Version "+firstBundle.Version+" -> "+secondBundle.Version+".")
	require.NotEmpty(t, diff.ChangedFiles)
	require.Equal(t, "01-clarification.md", diff.ChangedFiles[0].Path)
	require.Contains(t, diff.ChangedFiles[0].Added, "State assumptions explicitly before coding")
	require.Contains(t, diff.ChangedFiles[0].Removed, "Starts implementation before confirming missing requirements")

	updated := svc.reconcileSkillSetVersionDecisionLocked(project, nil, &SkillSetClientState{
		ProjectID:      project.ID,
		SyncStatus:     "synced",
		AppliedVersion: secondBundle.Version,
		AppliedHash:    secondBundle.CompiledHash,
	})
	require.True(t, updated)
	require.Equal(t, "deployed", findSkillSetVersionRecord(store.skillSetVersions[project.ID], secondBundle.Version, secondBundle.CompiledHash).DeploymentDecision)

	updated = svc.reconcileSkillSetVersionDecisionLocked(project, &SkillSetClientState{
		ProjectID:      project.ID,
		SyncStatus:     "synced",
		AppliedVersion: secondBundle.Version,
		AppliedHash:    secondBundle.CompiledHash,
	}, &SkillSetClientState{
		ProjectID:      project.ID,
		SyncStatus:     "rolled_back",
		AppliedVersion: firstBundle.Version,
		AppliedHash:    firstBundle.CompiledHash,
	})
	require.True(t, updated)
	require.Equal(t, "rolled_back", findSkillSetVersionRecord(store.skillSetVersions[project.ID], secondBundle.Version, secondBundle.CompiledHash).DeploymentDecision)
	require.Equal(t, "deployed", findSkillSetVersionRecord(store.skillSetVersions[project.ID], firstBundle.Version, firstBundle.CompiledHash).DeploymentDecision)

	latest, previous, created = svc.ensureLatestSkillSetVersionLocked(project, secondBundle, secondReports)
	require.False(t, created)
	require.NotNil(t, latest)
	require.NotNil(t, previous)
	require.Len(t, store.skillSetVersions[project.ID], 2)
}

func TestEnsureLatestSkillSetVersionLockedKeepsLowConfidenceCandidateInShadow(t *testing.T) {
	store := &AnalyticsStore{
		skillSetVersions: make(map[string][]*SkillSetVersion),
	}
	svc := &AnalyticsService{
		Options: Options{
			AnalyticsStore: store,
		},
	}
	project := &Project{ID: "project-low", OrgID: "org-1"}

	reports := []*Report{{
		ID:             "rep-low",
		ProjectID:      project.ID,
		Title:          "Unstable validation advice",
		Reason:         "Validation guidance changed too often to trust.",
		ExpectedImpact: "Reduce risky rollout.",
		Frictions:      []string{"Skips validation and assumes behavior will hold."},
		NextSteps:      []string{"Pause rollout until confidence improves."},
		Confidence:     "low",
		Score:          0.2,
		Status:         "active",
		CreatedAt:      time.Now().UTC(),
	}}

	bundle, err := buildLatestSkillSetBundle(project.ID, reports)
	require.NoError(t, err)

	latest, previous, created := svc.ensureLatestSkillSetVersionLocked(project, bundle, reports)
	require.True(t, created)
	require.NotNil(t, latest)
	require.Nil(t, previous)
	require.Equal(t, "shadow", latest.DeploymentDecision)
	require.Contains(t, latest.DecisionReason, "Shadow evaluation passed")
	require.NotNil(t, latest.ShadowEvaluation)
	require.Equal(t, "passed", latest.ShadowEvaluation.Guardrail)
	require.Equal(t, 1, latest.ShadowEvaluation.ChangedDocumentCount)
	require.InDelta(t, 0.51, latest.ShadowEvaluation.Score, 0.01)
}

func TestGetLatestSkillSetBundleBackfillsWithoutCallingRefineAgent(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		http.Error(w, "read path should not hit the refine model", http.StatusInternalServerError)
	}))
	defer server.Close()

	conf := &configs.Config{}
	conf.OpenAI.APIKey = "test-key"
	conf.OpenAI.BaseURL = server.URL

	store := &AnalyticsStore{
		projects: map[string]*Project{
			"project-1": {ID: "project-1", OrgID: "org-1", Name: "demo"},
		},
		reports: map[string]*Report{
			"rep-1": {
				ID:             "rep-1",
				ProjectID:      "project-1",
				Title:          "Requirement clarification drift",
				Reason:         "Requirement questions were skipped before implementation.",
				ExpectedImpact: "Fewer premature implementations.",
				Frictions:      []string{"Starts implementation before confirming missing requirements."},
				NextSteps:      []string{"Ask for missing constraints before touching code."},
				Status:         "active",
				CreatedAt:      time.Now().UTC(),
			},
		},
		projectReports:      map[string][]string{"project-1": {"rep-1"}},
		skillSetVersions:    make(map[string][]*SkillSetVersion),
		skillSetClients:     make(map[string]*SkillSetClientState),
		skillSetDeployments: make(map[string][]*SkillSetDeploymentEvent),
	}
	svc := &AnalyticsService{
		Options: Options{
			Config:         conf,
			AnalyticsStore: store,
		},
		refineAgent: NewSkillRefineAgent(conf),
	}

	resp, err := svc.GetLatestSkillSetBundle(context.Background(), &request.SkillSetBundleReq{ProjectID: "project-1"})
	require.NoError(t, err)
	require.Equal(t, "ready", resp.Status)
	require.NotEmpty(t, resp.Version)
	require.NotEmpty(t, resp.Files)
	require.Len(t, store.skillSetVersions["project-1"], 1)
	require.Zero(t, requestCount)

	resp, err = svc.GetLatestSkillSetBundle(context.Background(), &request.SkillSetBundleReq{ProjectID: "project-1"})
	require.NoError(t, err)
	require.Equal(t, "ready", resp.Status)
	require.Len(t, store.skillSetVersions["project-1"], 1)
	require.Zero(t, requestCount)
}

func hasSkillSetFile(files []response.SkillSetFileResp, path string, match func(string) bool) bool {
	for _, file := range files {
		if file.Path != path {
			continue
		}
		return match == nil || match(file.Content)
	}
	return false
}
