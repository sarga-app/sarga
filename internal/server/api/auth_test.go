//nolint:testpackage // tests use package-private fakeAuthStore + dummyHash.
package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sarga-app/sarga/internal/auth"
	"github.com/sarga-app/sarga/internal/db/queries"
	"github.com/sarga-app/sarga/openapi"
)

type fakeAuthStore struct {
	mu       sync.Mutex
	users    map[string]*queries.User
	byName   map[string]string
	tokens   map[string]*queries.RefreshToken
	tokenIDs map[string]string
}

func newFakeAuthStore() *fakeAuthStore {
	return &fakeAuthStore{
		users:    map[string]*queries.User{},
		byName:   map[string]string{},
		tokens:   map[string]*queries.RefreshToken{},
		tokenIDs: map[string]string{},
	}
}

func (s *fakeAuthStore) addUser(u queries.User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := u
	s.users[u.ID] = &cp
	s.byName[u.Username] = u.ID
}

func (s *fakeAuthStore) GetUserByUsername(_ context.Context, username string) (queries.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.byName[username]
	if !ok {
		return queries.User{}, sql.ErrNoRows
	}
	return *s.users[id], nil
}

func (s *fakeAuthStore) GetUserByID(_ context.Context, id string) (queries.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	if !ok {
		return queries.User{}, sql.ErrNoRows
	}
	return *u, nil
}

func (s *fakeAuthStore) UpdateUserPassword(_ context.Context, p queries.UpdateUserPasswordParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[p.ID]
	if !ok {
		return errors.New("not found")
	}
	u.PasswordHash = p.PasswordHash
	now := time.Now().UTC().Format(time.RFC3339)
	u.PasswordChangedAt = &now
	return nil
}

func (s *fakeAuthStore) CreateRefreshToken(_ context.Context, p queries.CreateRefreshTokenParams) (queries.RefreshToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rt := queries.RefreshToken{
		ID:        p.ID,
		UserID:    p.UserID,
		TokenHash: p.TokenHash,
		ExpiresAt: p.ExpiresAt,
		ParentID:  p.ParentID,
		CreatedAt: p.CreatedAt,
	}
	key := string(p.TokenHash)
	s.tokens[key] = &rt
	s.tokenIDs[rt.ID] = key
	return rt, nil
}

func (s *fakeAuthStore) GetRefreshTokenByHash(_ context.Context, hash []byte) (queries.RefreshToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rt, ok := s.tokens[string(hash)]
	if !ok {
		return queries.RefreshToken{}, sql.ErrNoRows
	}
	return *rt, nil
}

func (s *fakeAuthStore) RevokeRefreshToken(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key, ok := s.tokenIDs[id]
	if !ok {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	s.tokens[key].RevokedAt = &now
	return nil
}

func (s *fakeAuthStore) RevokeAllUserRefreshTokens(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339)
	for _, rt := range s.tokens {
		if rt.UserID == userID && rt.RevokedAt == nil {
			rt.RevokedAt = &now
		}
	}
	return nil
}

func newTestAPI(t *testing.T) (*API, *fakeAuthStore) {
	t.Helper()
	store := newFakeAuthStore()
	jwt := auth.JWTConfig{Secret: []byte("test-secret-32-bytes-long-padding!!")}
	a := &API{
		jwt:       jwt,
		authStore: store,
	}
	return a, store
}

func seedUser(t *testing.T, store *fakeAuthStore, username, password string, isAdmin bool) string {
	t.Helper()
	hash, err := auth.Hash(password)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)
	store.addUser(queries.User{
		ID:           id,
		Username:     username,
		PasswordHash: &hash,
		IsAdmin:      isAdmin,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	return id
}

func TestLogin_Success(t *testing.T) {
	t.Parallel()
	a, store := newTestAPI(t)
	seedUser(t, store, "alice", "hunter2!", true)

	body := strings.NewReader(`{"username":"alice","password":"hunter2!"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login", body)
	rr := httptest.NewRecorder()
	a.handleLogin(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var resp openapi.LoginResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.AccessToken == "" {
		t.Error("missing access_token")
	}
	if resp.ExpiresIn != int(auth.AccessTTL.Seconds()) {
		t.Errorf("ExpiresIn = %d, want %d", resp.ExpiresIn, int(auth.AccessTTL.Seconds()))
	}
	if refreshCookieFrom(t, rr) == nil {
		t.Error("refresh cookie not set")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	t.Parallel()
	a, store := newTestAPI(t)
	seedUser(t, store, "alice", "hunter2!", false)

	body := strings.NewReader(`{"username":"alice","password":"WRONG"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login", body)
	rr := httptest.NewRecorder()
	a.handleLogin(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}
}

func TestLogin_NoSuchUser(t *testing.T) {
	t.Parallel()
	a, _ := newTestAPI(t)
	body := strings.NewReader(`{"username":"ghost","password":"x"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login", body)
	rr := httptest.NewRecorder()
	a.handleLogin(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}
}

func TestRefresh_Rotates(t *testing.T) {
	t.Parallel()
	a, store := newTestAPI(t)
	seedUser(t, store, "alice", "hunter2!", false)

	loginRR := doLogin(t, a, "alice", "hunter2!")
	cookie := refreshCookieFrom(t, loginRR)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/refresh", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	a.handleRefresh(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("refresh status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if refreshCookieFrom(t, rr).Value == cookie.Value {
		t.Error("refresh did not rotate cookie")
	}
}

func TestRefresh_ReplayDetection(t *testing.T) {
	t.Parallel()
	a, store := newTestAPI(t)
	uid := seedUser(t, store, "alice", "hunter2!", false)

	loginRR := doLogin(t, a, "alice", "hunter2!")
	first := refreshCookieFrom(t, loginRR)

	req1 := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/refresh", nil)
	req1.AddCookie(first)
	rr1 := httptest.NewRecorder()
	a.handleRefresh(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("first refresh failed: %d", rr1.Code)
	}

	req2 := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/refresh", nil)
	req2.AddCookie(first)
	rr2 := httptest.NewRecorder()
	a.handleRefresh(rr2, req2)
	if rr2.Code != http.StatusUnauthorized {
		t.Errorf("replay status = %d, want 401", rr2.Code)
	}
	for _, rt := range store.tokens {
		if rt.UserID == uid && rt.RevokedAt == nil {
			t.Error("expected all of alice's tokens revoked after replay")
		}
	}
}

func TestLogout_RevokesAndClears(t *testing.T) {
	t.Parallel()
	a, store := newTestAPI(t)
	seedUser(t, store, "alice", "hunter2!", false)

	loginRR := doLogin(t, a, "alice", "hunter2!")
	cookie := refreshCookieFrom(t, loginRR)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/logout", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	a.handleLogout(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rr.Code)
	}
	for _, rt := range store.tokens {
		if rt.RevokedAt == nil {
			t.Error("token not revoked after logout")
		}
	}
}

func doLogin(t *testing.T, a *API, username, password string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(openapi.LoginRequest{Username: username, Password: password})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	a.handleLogin(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", rr.Code, rr.Body.String())
	}
	return rr
}

func refreshCookieFrom(t *testing.T, rr *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rr.Result().Cookies() {
		if c.Name == refreshCookieName {
			return c
		}
	}
	t.Fatal("refresh cookie not found")
	return nil
}
