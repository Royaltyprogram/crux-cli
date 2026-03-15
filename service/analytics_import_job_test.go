package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Royaltyprogram/aiops/configs"
	"github.com/Royaltyprogram/aiops/dto/request"
)

func TestCreateSessionImportJobCancelsExpiredActiveJob(t *testing.T) {
	svc, store, ctx, project := newSessionImportJobTestFixture(t)

	job, err := svc.CreateSessionImportJob(ctx, &request.SessionImportJobCreateReq{
		ProjectID:     project.ID,
		TotalSessions: 2,
	})
	require.NoError(t, err)

	_, err = svc.AppendSessionImportJobChunk(ctx, job.JobID, &request.SessionImportJobChunkReq{
		Sessions: []request.SessionSummaryReq{{
			Tool:      "codex",
			Timestamp: time.Now().UTC(),
		}},
	})
	require.NoError(t, err)

	store.mu.Lock()
	stale := store.sessionImportJobs[job.JobID]
	require.NotNil(t, stale)
	stale.UpdatedAt = time.Now().UTC().Add(-48 * time.Hour)
	store.mu.Unlock()

	replacement, err := svc.CreateSessionImportJob(ctx, &request.SessionImportJobCreateReq{
		ProjectID: project.ID,
	})
	require.NoError(t, err)
	require.NotEqual(t, job.JobID, replacement.JobID)
	require.False(t, replacement.Reused)

	store.mu.RLock()
	defer store.mu.RUnlock()
	expired := store.sessionImportJobs[job.JobID]
	require.NotNil(t, expired)
	require.Equal(t, sessionImportJobStatusCanceled, expired.Status)
	require.Contains(t, expired.LastError, "expired before completion")
	require.NotNil(t, expired.CompletedAt)
	require.Nil(t, expired.Sessions)
}

func TestCleanupExpiredSessionImportJobsDeletesOldTerminalJobs(t *testing.T) {
	svc, store, _, _ := newSessionImportJobTestFixture(t)

	now := time.Now().UTC()
	store.mu.Lock()
	store.sessionImportJobs["import-old"] = &SessionImportJob{
		ID:          "import-old",
		ProjectID:   "project-1",
		OrgID:       "org-1",
		AgentID:     "agent-1",
		Status:      sessionImportJobStatusSucceeded,
		CreatedAt:   now.Add(-10 * 24 * time.Hour),
		UpdatedAt:   now.Add(-9 * 24 * time.Hour),
		StartedAt:   cloneTime(ptrTime(now.Add(-9 * 24 * time.Hour))),
		CompletedAt: cloneTime(ptrTime(now.Add(-8 * 24 * time.Hour))),
	}
	require.NoError(t, svc.cleanupExpiredSessionImportJobsLocked(now))
	_, exists := store.sessionImportJobs["import-old"]
	store.mu.Unlock()
	require.False(t, exists)
}

func TestBuildSessionImportJobMetricsLocked(t *testing.T) {
	svc, store, _, _ := newSessionImportJobTestFixture(t)

	now := time.Now().UTC()
	job1Started := now.Add(-5 * time.Minute)
	job1Completed := now.Add(-4 * time.Minute)
	job2Started := now.Add(-3 * time.Minute)
	job2Completed := now.Add(-2 * time.Minute)

	store.mu.Lock()
	store.sessionImportJobs["import-1"] = &SessionImportJob{
		ID:                "import-1",
		ProjectID:         "project-1",
		OrgID:             "org-1",
		AgentID:           "agent-1",
		Status:            sessionImportJobStatusSucceeded,
		CreatedAt:         now.Add(-6 * time.Minute),
		UpdatedAt:         job1Completed,
		StartedAt:         cloneTime(&job1Started),
		CompletedAt:       cloneTime(&job1Completed),
		ProcessedSessions: 4,
		UploadedSessions:  4,
	}
	store.sessionImportJobs["import-2"] = &SessionImportJob{
		ID:                "import-2",
		ProjectID:         "project-1",
		OrgID:             "org-1",
		AgentID:           "agent-1",
		Status:            sessionImportJobStatusFailed,
		CreatedAt:         now.Add(-4 * time.Minute),
		UpdatedAt:         job2Completed,
		StartedAt:         cloneTime(&job2Started),
		CompletedAt:       cloneTime(&job2Completed),
		ProcessedSessions: 2,
		FailedSessions:    2,
	}
	store.sessionImportJobs["import-3"] = &SessionImportJob{
		ID:                "import-3",
		ProjectID:         "project-1",
		OrgID:             "org-1",
		AgentID:           "agent-1",
		Status:            sessionImportJobStatusRunning,
		CreatedAt:         now.Add(-time.Minute),
		UpdatedAt:         now,
		StartedAt:         cloneTime(ptrTime(now.Add(-time.Minute))),
		ProcessedSessions: 1,
	}

	metrics := svc.buildSessionImportJobMetricsLocked("org-1")
	store.mu.Unlock()

	require.NotNil(t, metrics)
	require.Equal(t, 3, metrics.CreatedJobs)
	require.Equal(t, 1, metrics.RunningJobs)
	require.Equal(t, 1, metrics.SucceededJobs)
	require.Equal(t, 1, metrics.FailedJobs)
	require.Equal(t, 7, metrics.ProcessedSessions)
	require.Equal(t, 4, metrics.UploadedSessions)
	require.Equal(t, 2, metrics.FailedSessions)
	require.InDelta(t, 0.29, metrics.FailureRate, 0.0001)
	require.Equal(t, 60_000, metrics.AvgDurationMS)
	require.InDelta(t, 3.0, metrics.ThroughputPerMinute, 0.0001)
	require.NotNil(t, metrics.LastCompletedAt)
	require.True(t, metrics.LastCompletedAt.Equal(job2Completed))
}

func TestListSessionImportJobsSupportsFailedOnlyAgentFilterAndCursor(t *testing.T) {
	svc, store, ctx, project := newSessionImportJobTestFixture(t)

	now := time.Now().UTC()
	store.mu.Lock()
	store.agents["agent-2"] = &Agent{
		ID:           "agent-2",
		OrgID:        "org-1",
		UserID:       "user-1",
		RegisteredAt: now,
	}
	store.sessionImportJobs["import-1"] = &SessionImportJob{
		ID:        "import-1",
		ProjectID: project.ID,
		OrgID:     "org-1",
		AgentID:   "agent-1",
		Status:    sessionImportJobStatusSucceeded,
		CreatedAt: now.Add(-4 * time.Minute),
		UpdatedAt: now.Add(-4 * time.Minute),
	}
	store.sessionImportJobs["import-2"] = &SessionImportJob{
		ID:             "import-2",
		ProjectID:      project.ID,
		OrgID:          "org-1",
		AgentID:        "agent-2",
		Status:         sessionImportJobStatusPartial,
		CreatedAt:      now.Add(-3 * time.Minute),
		UpdatedAt:      now.Add(-3 * time.Minute),
		FailedSessions: 1,
	}
	store.sessionImportJobs["import-3"] = &SessionImportJob{
		ID:             "import-3",
		ProjectID:      project.ID,
		OrgID:          "org-1",
		AgentID:        "agent-1",
		Status:         sessionImportJobStatusFailed,
		CreatedAt:      now.Add(-2 * time.Minute),
		UpdatedAt:      now.Add(-2 * time.Minute),
		FailedSessions: 2,
	}
	store.sessionImportJobs["import-4"] = &SessionImportJob{
		ID:        "import-4",
		ProjectID: project.ID,
		OrgID:     "org-1",
		AgentID:   "agent-1",
		Status:    sessionImportJobStatusRunning,
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now.Add(-time.Minute),
	}
	require.NoError(t, store.persistLocked())
	store.mu.Unlock()

	firstPage, err := svc.ListSessionImportJobs(ctx, &request.SessionImportJobListReq{
		ProjectID:  project.ID,
		FailedOnly: true,
		Limit:      1,
	})
	require.NoError(t, err)
	require.Len(t, firstPage.Items, 1)
	require.Equal(t, "import-3", firstPage.Items[0].JobID)
	require.Equal(t, "import-3", firstPage.NextCursor)

	secondPage, err := svc.ListSessionImportJobs(ctx, &request.SessionImportJobListReq{
		ProjectID:  project.ID,
		FailedOnly: true,
		Cursor:     firstPage.NextCursor,
		Limit:      1,
	})
	require.NoError(t, err)
	require.Len(t, secondPage.Items, 1)
	require.Equal(t, "import-2", secondPage.Items[0].JobID)
	require.Empty(t, secondPage.NextCursor)

	agentFiltered, err := svc.ListSessionImportJobs(ctx, &request.SessionImportJobListReq{
		ProjectID:  project.ID,
		AgentID:    "agent-1",
		FailedOnly: true,
		Limit:      10,
	})
	require.NoError(t, err)
	require.Len(t, agentFiltered.Items, 1)
	require.Equal(t, "import-3", agentFiltered.Items[0].JobID)

	_, err = svc.ListSessionImportJobs(ctx, &request.SessionImportJobListReq{
		ProjectID: project.ID,
		Cursor:    "missing-job",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown session_import_job cursor")
}

func newSessionImportJobTestFixture(t *testing.T) (*AnalyticsService, *AnalyticsStore, context.Context, *Project) {
	t.Helper()

	conf := &configs.Config{}
	conf.App.StorePath = filepath.Join(t.TempDir(), "crux-store.json")

	store, err := NewAnalyticsStore(conf)
	require.NoError(t, err)

	now := time.Now().UTC()
	store.mu.Lock()
	store.organizations["org-1"] = &Organization{
		ID:   "org-1",
		Name: "Org 1",
	}
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
	project := &Project{
		ID:          "project-1",
		OrgID:       "org-1",
		AgentID:     "agent-1",
		Name:        "Shared workspace",
		DefaultTool: "codex",
		ConnectedAt: now,
	}
	store.projects[project.ID] = project
	require.NoError(t, store.persistLocked())
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
	return svc, store, ctx, project
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
