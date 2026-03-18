package service

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Royaltyprogram/aiops/configs"
	"github.com/Royaltyprogram/aiops/dto/request"
)

func TestAnalyticsStoreTargetedPersistenceRoundTrip(t *testing.T) {
	conf := &configs.Config{}
	conf.App.StorePath = filepath.Join(t.TempDir(), "autoskills-store.json")

	store, err := NewAnalyticsStore(conf)
	require.NoError(t, err)

	now := time.Now().UTC().Round(time.Second)

	store.mu.Lock()
	store.seq = 42
	store.organizations["org-1"] = &Organization{ID: "org-1", Name: "Org 1"}
	store.users["user-1"] = &User{
		ID:        "user-1",
		OrgID:     "org-1",
		Email:     "user-1@example.com",
		Name:      "User 1",
		Role:      userRoleAdmin,
		Status:    userStatusActive,
		CreatedAt: now,
	}
	store.accessTokens["token-1"] = &AccessToken{
		ID:        "token-1",
		OrgID:     "org-1",
		UserID:    "user-1",
		Label:     "CLI token",
		Kind:      TokenKindCLI,
		TokenHash: "hash-1",
		CreatedAt: now,
	}
	store.agents["agent-1"] = &Agent{
		ID:           "agent-1",
		OrgID:        "org-1",
		UserID:       "user-1",
		RegisteredAt: now,
	}
	store.projects["project-1"] = &Project{
		ID:             "project-1",
		OrgID:          "org-1",
		AgentID:        "agent-1",
		Name:           "Shared workspace",
		DefaultTool:    "codex",
		LastIngestedAt: cloneTime(&now),
		ConnectedAt:    now,
	}
	store.configSnapshots["project-1"] = []*ConfigSnapshot{{
		ID:           "snapshot-1",
		ProjectID:    "project-1",
		Tool:         "codex",
		ProfileID:    "profile-1",
		CapturedAt:   now,
		Settings:     map[string]any{"mcp": true},
		HooksEnabled: true,
	}}
	store.sessionSummaries["project-1"] = []*SessionSummary{{
		ID:        "session-1",
		ProjectID: "project-1",
		Tool:      "codex",
		RawQueries: []string{
			"inspect persistence path",
		},
		Timestamp: now,
	}}
	store.reports["report-1"] = &Report{
		ID:        "report-1",
		ProjectID: "project-1",
		Title:     "Feedback",
		Status:    "active",
		CreatedAt: now,
	}
	store.projectReports["project-1"] = []string{"report-1"}
	store.reportResearch["project-1"] = &ReportResearchStatus{
		ProjectID:    "project-1",
		State:        "waiting_for_min_sessions",
		ReportCount:  1,
		SessionCount: 1,
	}
	store.sessionImportJobs["import-1"] = &SessionImportJob{
		ID:        "import-1",
		ProjectID: "project-1",
		OrgID:     "org-1",
		AgentID:   "agent-1",
		Status:    sessionImportJobStatusReceiving,
		CreatedAt: now,
		UpdatedAt: now,
		Failures:  []SessionImportJobFailure{},
	}
	store.audits = append(store.audits, &AuditEvent{
		ID:        "audit-1",
		OrgID:     "org-1",
		ProjectID: "project-1",
		Type:      "session.ingested",
		Message:   "session summary uploaded",
		Result:    "success",
		CreatedAt: now,
	})
	require.NoError(t, store.withTxLocked(func(tx *sql.Tx) error {
		if err := store.persistOrganizationLocked(tx, "org-1"); err != nil {
			return err
		}
		if err := store.persistUserLocked(tx, "user-1"); err != nil {
			return err
		}
		if err := store.persistAccessTokenLocked(tx, "token-1"); err != nil {
			return err
		}
		if err := store.persistAgentLocked(tx, "agent-1"); err != nil {
			return err
		}
		if err := store.persistProjectLocked(tx, "project-1"); err != nil {
			return err
		}
		if err := store.persistConfigSnapshotLocked(tx, "project-1", "snapshot-1"); err != nil {
			return err
		}
		if err := store.persistSessionSummaryLocked(tx, "project-1", "session-1"); err != nil {
			return err
		}
		if err := store.persistReportLocked(tx, "report-1"); err != nil {
			return err
		}
		if err := store.persistProjectReportsLocked(tx, "project-1"); err != nil {
			return err
		}
		if err := store.persistReportResearchLocked(tx, "project-1"); err != nil {
			return err
		}
		if err := store.persistSessionImportJobLocked(tx, "import-1"); err != nil {
			return err
		}
		return store.persistAuditLocked(tx, "audit-1")
	}))
	store.mu.Unlock()

	loaded, err := NewAnalyticsStore(conf)
	require.NoError(t, err)

	loaded.mu.RLock()
	defer loaded.mu.RUnlock()

	require.Equal(t, uint64(42), loaded.seq)
	require.Contains(t, loaded.organizations, "org-1")
	require.Contains(t, loaded.users, "user-1")
	require.Contains(t, loaded.accessTokens, "token-1")
	require.Contains(t, loaded.agents, "agent-1")
	require.Contains(t, loaded.projects, "project-1")
	require.Len(t, loaded.configSnapshots["project-1"], 1)
	require.Len(t, loaded.sessionSummaries["project-1"], 1)
	require.Equal(t, []string{"report-1"}, loaded.projectReports["project-1"])
	require.NotNil(t, loaded.reportResearch["project-1"])
	require.Contains(t, loaded.sessionImportJobs, "import-1")
	require.Len(t, loaded.audits, 1)
}

func TestMarkAccessTokenSeenAsyncPersistsDirtyTokenRows(t *testing.T) {
	conf := &configs.Config{}
	conf.App.StorePath = filepath.Join(t.TempDir(), "autoskills-store.json")

	store, err := NewAnalyticsStore(conf)
	require.NoError(t, err)

	now := time.Now().UTC().Round(time.Second)

	store.mu.Lock()
	store.accessTokens["token-1"] = &AccessToken{ID: "token-1", OrgID: "org-1", UserID: "user-1", Kind: TokenKindCLI, TokenHash: "hash-1", CreatedAt: now}
	store.accessTokens["token-2"] = &AccessToken{ID: "token-2", OrgID: "org-1", UserID: "user-1", Kind: TokenKindCLI, TokenHash: "hash-2", CreatedAt: now}
	require.NoError(t, store.withTxLocked(func(tx *sql.Tx) error {
		if err := store.persistAccessTokenLocked(tx, "token-1"); err != nil {
			return err
		}
		return store.persistAccessTokenLocked(tx, "token-2")
	}))
	store.mu.Unlock()

	seenAt1 := now.Add(2 * time.Minute)
	seenAt2 := now.Add(3 * time.Minute)
	store.MarkAccessTokenSeenAsync("token-1", seenAt1)
	store.MarkAccessTokenSeenAsync("token-2", seenAt2)

	require.Eventually(t, func() bool {
		store.mu.RLock()
		defer store.mu.RUnlock()
		return !store.lastSeenFlush && !store.lastSeenDirty && len(store.lastSeenTokens) == 0
	}, 3*time.Second, 50*time.Millisecond)

	loaded, err := NewAnalyticsStore(conf)
	require.NoError(t, err)

	loaded.mu.RLock()
	defer loaded.mu.RUnlock()

	require.NotNil(t, loaded.accessTokens["token-1"].LastSeenAt)
	require.NotNil(t, loaded.accessTokens["token-2"].LastSeenAt)
	require.True(t, loaded.accessTokens["token-1"].LastSeenAt.Equal(seenAt1))
	require.True(t, loaded.accessTokens["token-2"].LastSeenAt.Equal(seenAt2))
}

func TestUploadSessionSummaryPersistsRoundTrip(t *testing.T) {
	svc, _, conf, ctx, project := newAnalyticsPersistenceFixture(t)

	recordedAt := time.Now().UTC().Round(time.Second)
	resp, err := svc.UploadSessionSummary(ctx, &request.SessionSummaryReq{
		ProjectID:  project.ID,
		SessionID:  "session-1",
		Tool:       "codex",
		RawQueries: []string{"trace the request path"},
		Timestamp:  recordedAt,
	})
	require.NoError(t, err)
	require.Equal(t, "session-1", resp.SessionID)

	loaded, err := NewAnalyticsStore(conf)
	require.NoError(t, err)

	loaded.mu.RLock()
	defer loaded.mu.RUnlock()

	require.Len(t, loaded.sessionSummaries[project.ID], 1)
	require.Equal(t, "session-1", loaded.sessionSummaries[project.ID][0].ID)
	require.NotNil(t, loaded.projects[project.ID].LastIngestedAt)
	require.True(t, loaded.projects[project.ID].LastIngestedAt.Equal(recordedAt))
	require.NotNil(t, loaded.reportResearch[project.ID])
	require.Equal(t, "waiting_for_min_sessions", loaded.reportResearch[project.ID].State)
	require.Len(t, loaded.audits, 1)
	require.Equal(t, "session.ingested", loaded.audits[0].Type)
}

func TestCleanupExpiredSessionImportJobsPersistsCancellationAndDeletion(t *testing.T) {
	svc, store, conf, _, project := newAnalyticsPersistenceFixture(t)

	now := time.Now().UTC().Round(time.Second)
	store.mu.Lock()
	store.sessionImportJobs["import-stale"] = &SessionImportJob{
		ID:        "import-stale",
		ProjectID: project.ID,
		OrgID:     project.OrgID,
		AgentID:   project.AgentID,
		Status:    sessionImportJobStatusReceiving,
		CreatedAt: now.Add(-48 * time.Hour),
		UpdatedAt: now.Add(-48 * time.Hour),
		Failures:  []SessionImportJobFailure{},
	}
	store.sessionImportJobs["import-terminal"] = &SessionImportJob{
		ID:          "import-terminal",
		ProjectID:   project.ID,
		OrgID:       project.OrgID,
		AgentID:     project.AgentID,
		Status:      sessionImportJobStatusSucceeded,
		CreatedAt:   now.Add(-10 * 24 * time.Hour),
		UpdatedAt:   now.Add(-9 * 24 * time.Hour),
		CompletedAt: cloneTime(ptrTime(now.Add(-8 * 24 * time.Hour))),
		Failures:    []SessionImportJobFailure{},
	}
	require.NoError(t, store.withTxLocked(func(tx *sql.Tx) error {
		if err := store.persistSessionImportJobLocked(tx, "import-stale"); err != nil {
			return err
		}
		return store.persistSessionImportJobLocked(tx, "import-terminal")
	}))
	require.NoError(t, svc.cleanupExpiredSessionImportJobsLocked(now))
	store.mu.Unlock()

	loaded, err := NewAnalyticsStore(conf)
	require.NoError(t, err)

	loaded.mu.RLock()
	defer loaded.mu.RUnlock()

	stale := loaded.sessionImportJobs["import-stale"]
	require.NotNil(t, stale)
	require.Equal(t, sessionImportJobStatusCanceled, stale.Status)
	require.Contains(t, stale.LastError, "expired before completion")
	require.NotNil(t, stale.CompletedAt)

	_, exists := loaded.sessionImportJobs["import-terminal"]
	require.False(t, exists)
}

func TestSyncSkillSetVersionHistoryLockedDeletesTrimmedRows(t *testing.T) {
	conf := &configs.Config{}
	conf.App.StorePath = filepath.Join(t.TempDir(), "autoskills-store.json")

	store, err := NewAnalyticsStore(conf)
	require.NoError(t, err)

	svc := NewAnalyticsService(Options{
		Config:         conf,
		AnalyticsStore: store,
	})

	now := time.Now().UTC().Round(time.Second)
	v1 := &SkillSetVersion{
		ID:           "skver-1",
		ProjectID:    "project-1",
		OrgID:        "org-1",
		BundleName:   managedSkillBundleName,
		Version:      "v1",
		CompiledHash: "hash-1",
		CreatedAt:    now.Add(-2 * time.Hour),
		GeneratedAt:  now.Add(-2 * time.Hour),
	}
	v2 := &SkillSetVersion{
		ID:           "skver-2",
		ProjectID:    "project-1",
		OrgID:        "org-1",
		BundleName:   managedSkillBundleName,
		Version:      "v2",
		CompiledHash: "hash-2",
		CreatedAt:    now.Add(-time.Hour),
		GeneratedAt:  now.Add(-time.Hour),
	}
	v3 := &SkillSetVersion{
		ID:           "skver-3",
		ProjectID:    "project-1",
		OrgID:        "org-1",
		BundleName:   managedSkillBundleName,
		Version:      "v3",
		CompiledHash: "hash-3",
		CreatedAt:    now,
		GeneratedAt:  now,
	}

	store.mu.Lock()
	store.skillSetVersions["project-1"] = []*SkillSetVersion{v1, v2}
	require.NoError(t, store.withTxLocked(func(tx *sql.Tx) error {
		return store.persistSkillSetVersionsForProjectLocked(tx, "project-1", skillSetVersionIDs(store.skillSetVersions["project-1"]))
	}))

	previousIDs := skillSetVersionIDs(store.skillSetVersions["project-1"])
	store.skillSetVersions["project-1"] = []*SkillSetVersion{v2, v3}
	require.NoError(t, store.withTxLocked(func(tx *sql.Tx) error {
		return svc.syncSkillSetVersionHistoryLocked(tx, "project-1", previousIDs)
	}))
	store.mu.Unlock()

	loaded, err := NewAnalyticsStore(conf)
	require.NoError(t, err)

	loaded.mu.RLock()
	defer loaded.mu.RUnlock()

	history := loaded.skillSetVersions["project-1"]
	require.Len(t, history, 2)
	require.Nil(t, findSkillSetVersionLockedForTest(history, "skver-1"))
	require.NotNil(t, findSkillSetVersionLockedForTest(history, "skver-2"))
	require.NotNil(t, findSkillSetVersionLockedForTest(history, "skver-3"))
}

func newAnalyticsPersistenceFixture(t *testing.T) (*AnalyticsService, *AnalyticsStore, *configs.Config, context.Context, *Project) {
	t.Helper()

	conf := &configs.Config{}
	conf.App.StorePath = filepath.Join(t.TempDir(), "autoskills-store.json")

	store, err := NewAnalyticsStore(conf)
	require.NoError(t, err)

	now := time.Now().UTC().Round(time.Second)
	project := &Project{
		ID:          "project-1",
		OrgID:       "org-1",
		AgentID:     "agent-1",
		Name:        sharedWorkspaceName,
		DefaultTool: "codex",
		ConnectedAt: now,
	}

	store.mu.Lock()
	store.organizations["org-1"] = &Organization{ID: "org-1", Name: "Org 1"}
	store.users["user-1"] = &User{
		ID:        "user-1",
		OrgID:     "org-1",
		Email:     "user-1@example.com",
		Name:      "User 1",
		Role:      userRoleAdmin,
		Status:    userStatusActive,
		CreatedAt: now,
	}
	store.agents["agent-1"] = &Agent{
		ID:           "agent-1",
		OrgID:        "org-1",
		UserID:       "user-1",
		RegisteredAt: now,
	}
	store.projects[project.ID] = project
	require.NoError(t, store.withTxLocked(func(tx *sql.Tx) error {
		if err := store.persistOrganizationLocked(tx, "org-1"); err != nil {
			return err
		}
		if err := store.persistUserLocked(tx, "user-1"); err != nil {
			return err
		}
		if err := store.persistAgentLocked(tx, "agent-1"); err != nil {
			return err
		}
		return store.persistProjectLocked(tx, project.ID)
	}))
	store.mu.Unlock()

	svc := NewAnalyticsService(Options{
		Config:            conf,
		AnalyticsStore:    store,
		ReportMinSessions: 10,
	})
	ctx := WithAuthIdentity(context.Background(), AuthIdentity{
		TokenKind: TokenKindDeviceAccess,
		OrgID:     "org-1",
		UserID:    "user-1",
		AgentID:   "agent-1",
	})

	return svc, store, conf, ctx, project
}

func findSkillSetVersionLockedForTest(history []*SkillSetVersion, id string) *SkillSetVersion {
	for _, item := range history {
		if item != nil && item.ID == id {
			return item
		}
	}
	return nil
}
