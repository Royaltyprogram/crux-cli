package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Royaltyprogram/aiops/configs"
	"github.com/Royaltyprogram/aiops/dto/request"
)

func TestIssueAuthenticateRevokeCLITokenPersistsRoundTrip(t *testing.T) {
	svc, store, conf, webCtx, webIdentity := newAuthPersistenceFixture(t)

	issued, err := svc.IssueCLIToken(webCtx, &request.IssueCLITokenReq{Label: "CLI enrollment"})
	require.NoError(t, err)

	cliCtx := WithRequestMetadata(WithAuthIdentity(context.Background(), AuthIdentity{
		TokenID:   issued.TokenID,
		TokenKind: TokenKindCLIEnrollment,
		OrgID:     webIdentity.OrgID,
		UserID:    webIdentity.UserID,
		UserRole:  webIdentity.UserRole,
	}), RequestMetadata{
		SourceIP:  "127.0.0.1",
		UserAgent: "autoskills-cli/test",
	})

	loginResp, err := svc.AuthenticateCLI(cliCtx, &request.CLILoginReq{
		DeviceID:   "device-1",
		DeviceName: "Test Laptop",
		Hostname:   "test-host",
		Platform:   "darwin",
		CLIVersion: "1.0.0",
		Tools:      []string{"codex"},
	})
	require.NoError(t, err)
	require.Equal(t, "device-1", loginResp.AgentID)

	_, err = svc.RevokeCLIToken(webCtx, &request.RevokeCLITokenReq{TokenID: issued.TokenID})
	require.NoError(t, err)

	loaded, err := NewAnalyticsStore(conf)
	require.NoError(t, err)

	loaded.mu.RLock()
	defer loaded.mu.RUnlock()

	enrollment := loaded.accessTokens[issued.TokenID]
	require.NotNil(t, enrollment)
	require.NotNil(t, enrollment.ConsumedAt)
	require.NotNil(t, enrollment.RevokedAt)
	require.Equal(t, TokenKindCLIEnrollment, enrollment.Kind)

	agent := loaded.agents[loginResp.AgentID]
	require.NotNil(t, agent)
	require.Equal(t, webIdentity.UserID, agent.UserID)

	deviceTokenCount := 0
	for _, token := range loaded.accessTokens {
		if token == nil || token.AgentID != loginResp.AgentID {
			continue
		}
		deviceTokenCount++
		require.NotNil(t, token.RevokedAt)
	}
	require.Equal(t, 2, deviceTokenCount)

	require.ElementsMatch(t, []string{
		"auth.cli_token_issued",
		"device.registered",
		"auth.cli_token_revoked",
	}, auditTypesForTest(loaded.audits))

	store.Close()
}

func TestLogoutPersistsRevokedSessionAndAudit(t *testing.T) {
	svc, _, conf, webCtx, webIdentity := newAuthPersistenceFixture(t)

	_, err := svc.Logout(webCtx)
	require.NoError(t, err)

	loaded, err := NewAnalyticsStore(conf)
	require.NoError(t, err)

	loaded.mu.RLock()
	defer loaded.mu.RUnlock()

	record := loaded.accessTokens[webIdentity.TokenID]
	require.NotNil(t, record)
	require.NotNil(t, record.RevokedAt)
	require.Contains(t, auditTypesForTest(loaded.audits), "auth.logout")
}

func TestCompleteGoogleAuthPersistsRoundTrip(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			require.Equal(t, http.MethodPost, r.Method)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"access_token": "google-access-token",
				"token_type":   "Bearer",
				"id_token":     "id-token",
			})
		case "/userinfo":
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "Bearer google-access-token", r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sub":            "google-subject-1",
				"email":          "google-user@example.com",
				"email_verified": true,
				"name":           "Google User",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	conf := &configs.Config{}
	conf.App.Mode = "prod"
	conf.App.StorePath = filepath.Join(t.TempDir(), "autoskills-store.json")
	conf.Auth.Google.ClientID = "client-id"
	conf.Auth.Google.ClientSecret = "client-secret"
	conf.Auth.Google.AllowedDomains = []string{"example.com"}
	conf.Auth.Google.TokenURL = server.URL + "/token"
	conf.Auth.Google.UserInfoURL = server.URL + "/userinfo"

	store, err := NewAnalyticsStore(conf)
	require.NoError(t, err)

	svc := NewAnalyticsService(Options{
		Config:         conf,
		AnalyticsStore: store,
	})

	ctx := WithRequestMetadata(context.Background(), RequestMetadata{
		SourceIP:  "203.0.113.10",
		UserAgent: "browser-test",
	})
	resp, err := svc.CompleteGoogleAuth(ctx, "https://example.com/callback", "google-code")
	require.NoError(t, err)
	require.Equal(t, "google-user@example.com", resp.User.Email)
	require.Equal(t, userRoleMember, resp.User.Role)

	loaded, err := NewAnalyticsStore(conf)
	require.NoError(t, err)

	loaded.mu.RLock()
	defer loaded.mu.RUnlock()

	var googleUser *User
	for _, user := range loaded.users {
		if user != nil && user.AuthProvider == authProviderGoogle {
			googleUser = user
			break
		}
	}
	require.NotNil(t, googleUser)
	require.Equal(t, "google-subject-1", googleUser.AuthSubject)
	require.Equal(t, userRoleMember, normalizeUserRole(googleUser.Role))
	require.NotNil(t, googleUser.LastLoginAt)

	org := loaded.organizations[googleUser.OrgID]
	require.NotNil(t, org)
	require.Equal(t, resp.Organization.ID, org.ID)

	tokenCount := 0
	for _, token := range loaded.accessTokens {
		if token != nil && token.UserID == googleUser.ID && token.Kind == TokenKindWebSession {
			tokenCount++
		}
	}
	require.Equal(t, 1, tokenCount)

	require.Contains(t, auditTypesForTest(loaded.audits), "auth.login")
	require.Equal(t, "203.0.113.10", loaded.audits[0].SourceIP)
	require.Equal(t, "browser-test", loaded.audits[0].UserAgent)
}

func TestDevLoginPersistsSessionTokenAndLastLogin(t *testing.T) {
	conf := &configs.Config{}
	conf.App.Mode = "debug"
	conf.App.StorePath = filepath.Join(t.TempDir(), "autoskills-store.json")

	store, err := NewAnalyticsStore(conf)
	require.NoError(t, err)

	svc := NewAnalyticsService(Options{
		Config:         conf,
		AnalyticsStore: store,
	})

	resp, err := svc.DevLogin()
	require.NoError(t, err)
	require.NotEmpty(t, resp.SessionToken)

	loaded, err := NewAnalyticsStore(conf)
	require.NoError(t, err)

	loaded.mu.RLock()
	defer loaded.mu.RUnlock()

	user := loaded.users[defaultDemoUserID]
	require.NotNil(t, user)
	require.NotNil(t, user.LastLoginAt)

	sessionCount := 0
	for _, token := range loaded.accessTokens {
		if token != nil && token.UserID == defaultDemoUserID && token.Kind == TokenKindWebSession {
			sessionCount++
		}
	}
	require.Equal(t, 1, sessionCount)
}

func newAuthPersistenceFixture(t *testing.T) (*AnalyticsService, *AnalyticsStore, *configs.Config, context.Context, AuthIdentity) {
	t.Helper()

	conf := &configs.Config{}
	conf.App.Mode = "prod"
	conf.App.StorePath = filepath.Join(t.TempDir(), "autoskills-store.json")

	store, err := NewAnalyticsStore(conf)
	require.NoError(t, err)

	now := time.Now().UTC().Round(time.Second)

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
	sessionTokenValue, sessionTokenRecord, err := store.issueAccessTokenLocked(TokenKindWebSession, "org-1", "user-1", "dashboard session", defaultSessionTokenTTL, now)
	require.NoError(t, err)
	require.NotEmpty(t, sessionTokenValue)
	require.NoError(t, store.withTxLocked(func(tx *sql.Tx) error {
		if err := store.persistOrganizationLocked(tx, "org-1"); err != nil {
			return err
		}
		if err := store.persistUserLocked(tx, "user-1"); err != nil {
			return err
		}
		return store.persistAccessTokenLocked(tx, sessionTokenRecord.ID)
	}))
	store.mu.Unlock()

	svc := NewAnalyticsService(Options{
		Config:         conf,
		AnalyticsStore: store,
	})

	identity := AuthIdentity{
		TokenID:   sessionTokenRecord.ID,
		TokenKind: TokenKindWebSession,
		OrgID:     "org-1",
		UserID:    "user-1",
		UserRole:  userRoleAdmin,
	}
	ctx := WithRequestMetadata(WithAuthIdentity(context.Background(), identity), RequestMetadata{
		SourceIP:  "127.0.0.1",
		UserAgent: "dashboard-test",
	})

	return svc, store, conf, ctx, identity
}

func auditTypesForTest(audits []*AuditEvent) []string {
	items := make([]string, 0, len(audits))
	for _, audit := range audits {
		if audit != nil {
			items = append(items, audit.Type)
		}
	}
	return items
}
