package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Royaltyprogram/aiops/configs"
	"github.com/Royaltyprogram/aiops/service"
)

type googleAuthRouteTestUser struct {
	Code     string
	Subject  string
	Email    string
	Name     string
	Verified bool
}

func configureGoogleAuthRoutesTest(t *testing.T, conf *configs.Config, users ...googleAuthRouteTestUser) func() {
	t.Helper()

	byCode := make(map[string]googleAuthRouteTestUser, len(users))
	for _, user := range users {
		byCode[user.Code] = user
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			require.Equal(t, http.MethodPost, r.Method)
			require.NoError(t, r.ParseForm())
			code := r.Form.Get("code")
			user, ok := byCode[code]
			require.True(t, ok, "unexpected google auth code %q", code)
			require.Equal(t, "test-google-client-id", r.Form.Get("client_id"))
			require.Equal(t, "test-google-client-secret", r.Form.Get("client_secret"))
			require.Equal(t, "authorization_code", r.Form.Get("grant_type"))
			require.Equal(t, "http://example.com/api/v1/auth/google/callback", r.Form.Get("redirect_uri"))
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(`{"access_token":"token-` + user.Code + `","token_type":"Bearer"}`))
			require.NoError(t, err)
		case "/userinfo":
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			code := strings.TrimPrefix(token, "token-")
			user, ok := byCode[code]
			require.True(t, ok, "unexpected google bearer token %q", token)
			body, err := json.Marshal(map[string]any{
				"sub":            user.Subject,
				"email":          user.Email,
				"email_verified": user.Verified,
				"name":           user.Name,
			})
			require.NoError(t, err)
			w.Header().Set("Content-Type", "application/json")
			_, err = w.Write(body)
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))

	conf.Auth.Google.ClientID = "test-google-client-id"
	conf.Auth.Google.ClientSecret = "test-google-client-secret"
	conf.Auth.Google.AuthURL = server.URL + "/auth"
	conf.Auth.Google.TokenURL = server.URL + "/token"
	conf.Auth.Google.UserInfoURL = server.URL + "/userinfo"

	return server.Close
}

func loginWithGoogleRoutesTest(t *testing.T, handler http.Handler, code string) *http.Cookie {
	t.Helper()

	startReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/start", nil)
	startReq = startReq.WithContext(context.Background())
	startRec := httptest.NewRecorder()
	handler.ServeHTTP(startRec, startReq)
	require.Equal(t, http.StatusSeeOther, startRec.Code)

	var stateCookie *http.Cookie
	for _, cookie := range startRec.Result().Cookies() {
		if cookie.Name == service.GoogleOAuthStateCookieName {
			stateCookie = cookie
			break
		}
	}
	require.NotNil(t, stateCookie)

	startURL, err := url.Parse(startRec.Header().Get("Location"))
	require.NoError(t, err)
	callbackReq := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/auth/google/callback?code="+url.QueryEscape(code)+"&state="+url.QueryEscape(startURL.Query().Get("state")),
		nil,
	)
	callbackReq = callbackReq.WithContext(context.Background())
	callbackReq.AddCookie(stateCookie)
	callbackRec := httptest.NewRecorder()
	handler.ServeHTTP(callbackRec, callbackReq)
	require.Equal(t, http.StatusSeeOther, callbackRec.Code)
	require.Equal(t, "/dashboard", callbackRec.Header().Get("Location"))

	for _, cookie := range callbackRec.Result().Cookies() {
		if cookie.Name == service.WebSessionCookieName {
			return cookie
		}
	}

	require.FailNowf(t, "cookie missing", "expected cookie %q on response", service.WebSessionCookieName)
	return nil
}
