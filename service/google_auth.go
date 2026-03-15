package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Royaltyprogram/aiops/dto/response"
	"github.com/Royaltyprogram/aiops/pkg/ecode"
)

const (
	GoogleOAuthStateCookieName = "crux_google_oauth_state"

	googleOAuthStateTTL = 10 * time.Minute

	defaultGoogleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	defaultGoogleTokenURL    = "https://oauth2.googleapis.com/token"
	defaultGoogleUserInfoURL = "https://openidconnect.googleapis.com/v1/userinfo"
)

type GoogleAuthStart struct {
	State       string
	RedirectURL string
}

type googleTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	IDToken     string `json:"id_token"`
}

type googleUserInfo struct {
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
}

func (s *AnalyticsService) BeginGoogleAuth(redirectURL string) (*GoogleAuthStart, error) {
	conf, err := s.googleAuthConfig()
	if err != nil {
		return nil, err
	}

	state := randomHex(16)
	params := url.Values{}
	params.Set("client_id", conf.ClientID)
	params.Set("redirect_uri", redirectURL)
	params.Set("response_type", "code")
	params.Set("scope", "openid email profile")
	params.Set("state", state)
	params.Set("access_type", "offline")
	params.Set("prompt", "select_account")

	return &GoogleAuthStart{
		State:       state,
		RedirectURL: conf.authURL() + "?" + params.Encode(),
	}, nil
}

func (s *AnalyticsService) CompleteGoogleAuth(ctx context.Context, redirectURL, code string) (*response.LoginResp, error) {
	conf, err := s.googleAuthConfig()
	if err != nil {
		return nil, err
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, ecode.InvalidParams.WithCause(ecode.NewInvalidParamsErr("missing Google auth code"))
	}

	tokenResp, err := exchangeGoogleCode(ctx, conf, redirectURL, code)
	if err != nil {
		return nil, err
	}
	info, err := fetchGoogleUserInfo(ctx, conf, tokenResp.AccessToken)
	if err != nil {
		return nil, err
	}
	if !info.EmailVerified {
		return nil, ecode.Forbidden(1015, "google account email is not verified")
	}
	if !googleEmailAllowed(conf.AllowedDomains, info.Email) {
		return nil, ecode.Forbidden(1016, "google account domain is not allowed")
	}

	now := time.Now().UTC()

	s.AnalyticsStore.mu.Lock()
	defer s.AnalyticsStore.mu.Unlock()

	user, org, err := s.ensureGoogleUserLocked(info, now)
	if err != nil {
		return nil, err
	}
	tokenValue, tokenRecord, err := s.AnalyticsStore.issueAccessTokenLocked(TokenKindWebSession, user.OrgID, user.ID, "dashboard session", defaultSessionTokenTTL, now)
	if err != nil {
		return nil, err
	}

	user.LastLoginAt = cloneTime(&now)
	s.appendAuditLocked(ctx, user.OrgID, auditEventInput{
		Type:        "auth.login",
		Message:     "dashboard session created",
		ActorUserID: user.ID,
		ActorRole:   normalizeUserRole(user.Role),
		Result:      "success",
		Reason:      "user authenticated with Google OAuth",
	})
	if err := s.AnalyticsStore.persistLocked(); err != nil {
		return nil, err
	}

	return &response.LoginResp{
		SessionToken:     tokenValue,
		SessionExpiresAt: tokenRecord.ExpiresAt,
		User:             toSessionUser(user),
		Organization:     toSessionOrganization(org),
	}, nil
}

func (s *AnalyticsService) googleAuthConfig() (googleAuthConfig, error) {
	if s == nil || s.Config == nil {
		return googleAuthConfig{}, ecode.New(1012, 503, "google sign-in is not configured")
	}
	conf := googleAuthConfig{
		ClientID:       strings.TrimSpace(s.Config.Auth.Google.ClientID),
		ClientSecret:   strings.TrimSpace(s.Config.Auth.Google.ClientSecret),
		AllowedDomains: append([]string(nil), s.Config.Auth.Google.AllowedDomains...),
		AuthURL:        strings.TrimSpace(s.Config.Auth.Google.AuthURL),
		TokenURL:       strings.TrimSpace(s.Config.Auth.Google.TokenURL),
		UserInfoURL:    strings.TrimSpace(s.Config.Auth.Google.UserInfoURL),
	}
	if conf.ClientID == "" || conf.ClientSecret == "" {
		return googleAuthConfig{}, ecode.New(1012, 503, "google sign-in is not configured")
	}
	return conf, nil
}

type googleAuthConfig struct {
	ClientID       string
	ClientSecret   string
	AllowedDomains []string
	AuthURL        string
	TokenURL       string
	UserInfoURL    string
}

func (c googleAuthConfig) authURL() string {
	if c.AuthURL != "" {
		return c.AuthURL
	}
	return defaultGoogleAuthURL
}

func (c googleAuthConfig) tokenURL() string {
	if c.TokenURL != "" {
		return c.TokenURL
	}
	return defaultGoogleTokenURL
}

func (c googleAuthConfig) userInfoURL() string {
	if c.UserInfoURL != "" {
		return c.UserInfoURL
	}
	return defaultGoogleUserInfoURL
}

func exchangeGoogleCode(ctx context.Context, conf googleAuthConfig, redirectURL, code string) (*googleTokenResponse, error) {
	form := url.Values{}
	form.Set("client_id", conf.ClientID)
	form.Set("client_secret", conf.ClientSecret)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", redirectURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, conf.tokenURL(), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, ecode.InternalServerErr.WithCause(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, ecode.InternalServerErr.WithCause(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, ecode.InternalServerErr.WithCause(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ecode.Unauthorized(1013, "failed to exchange Google auth code")
	}

	var payload googleTokenResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, ecode.InternalServerErr.WithCause(err)
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return nil, ecode.Unauthorized(1013, "failed to exchange Google auth code")
	}

	return &payload, nil
}

func fetchGoogleUserInfo(ctx context.Context, conf googleAuthConfig, accessToken string) (*googleUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, conf.userInfoURL(), nil)
	if err != nil {
		return nil, ecode.InternalServerErr.WithCause(err)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, ecode.InternalServerErr.WithCause(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, ecode.InternalServerErr.WithCause(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ecode.Unauthorized(1014, "failed to read Google account profile")
	}

	var payload googleUserInfo
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, ecode.InternalServerErr.WithCause(err)
	}
	if strings.TrimSpace(payload.Subject) == "" || normalizeEmail(payload.Email) == "" {
		return nil, ecode.Unauthorized(1014, "failed to read Google account profile")
	}

	return &payload, nil
}

func googleEmailAllowed(allowedDomains []string, email string) bool {
	if len(allowedDomains) == 0 {
		return true
	}
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return false
	}
	domain := strings.ToLower(strings.TrimSpace(email[at+1:]))
	if domain == "" {
		return false
	}
	for _, item := range allowedDomains {
		if domain == strings.ToLower(strings.TrimSpace(item)) {
			return true
		}
	}
	return false
}

func (s *AnalyticsService) ensureGoogleUserLocked(info *googleUserInfo, now time.Time) (*User, *Organization, error) {
	subject := strings.TrimSpace(info.Subject)
	email := normalizeEmail(info.Email)
	name := strings.TrimSpace(info.Name)

	user := s.AnalyticsStore.findUserByAuthSubjectLocked(authProviderGoogle, subject)
	if user == nil {
		user = s.AnalyticsStore.findUserByEmailLocked(email)
	}
	if user != nil {
		if !userCanAuthenticate(user) {
			return nil, nil, ecode.Forbidden(1017, "user account cannot sign in")
		}
		if existing := s.AnalyticsStore.findUserByAuthSubjectLocked(authProviderGoogle, subject); existing != nil && existing.ID != user.ID {
			return nil, nil, ecode.Forbidden(1018, "google account is already linked to another user")
		}
		if provider := normalizeAuthProvider(user.AuthProvider); provider != "" && provider != authProviderGoogle {
			return nil, nil, ecode.Forbidden(1019, "user account is linked to a different sign-in provider")
		}
		if provider := normalizeAuthProvider(user.AuthProvider); provider == authProviderGoogle && strings.TrimSpace(user.AuthSubject) != "" && strings.TrimSpace(user.AuthSubject) != subject {
			return nil, nil, ecode.Forbidden(1020, "google account does not match the existing user link")
		}
		if email != "" && normalizeEmail(user.Email) != email {
			if existing := s.AnalyticsStore.findUserByEmailLocked(email); existing != nil && existing.ID != user.ID {
				return nil, nil, ecode.Forbidden(1021, "google account email is already linked to another user")
			}
			user.Email = email
		}
		if strings.TrimSpace(user.Name) == "" && name != "" {
			user.Name = name
		}
		user.AuthProvider = authProviderGoogle
		user.AuthSubject = subject

		org, ok := s.AnalyticsStore.organizations[user.OrgID]
		if !ok {
			return nil, nil, ecode.NotFound.WithCause(ecode.NewInvalidParamsErr("unknown org_id"))
		}
		return user, org, nil
	}

	orgName := defaultGoogleOrganizationName(name, email)
	org := &Organization{
		ID:   s.AnalyticsStore.nextID("org"),
		Name: orgName,
	}
	s.AnalyticsStore.organizations[org.ID] = org

	user = &User{
		ID:           s.AnalyticsStore.nextID("user"),
		OrgID:        org.ID,
		Email:        email,
		Name:         firstNonEmptyTrimmed(name, email),
		Source:       userSourceGoogle,
		AuthProvider: authProviderGoogle,
		AuthSubject:  subject,
		Role:         userRoleAdmin,
		Status:       userStatusActive,
		CreatedAt:    now,
	}
	s.AnalyticsStore.users[user.ID] = user

	return user, org, nil
}

func defaultGoogleOrganizationName(name, email string) string {
	label := firstNonEmptyTrimmed(strings.TrimSpace(name), normalizeEmail(email))
	if label == "" {
		label = "Personal"
	}
	return fmt.Sprintf("%s Workspace", label)
}

func firstNonEmptyTrimmed(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
