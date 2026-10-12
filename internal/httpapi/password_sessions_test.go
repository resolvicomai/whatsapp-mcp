package httpapi

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/BrOrlandi/whatsapp-mcp/internal/health"
)

// Changing the password is how an owner evicts someone who got into the panel,
// so a session opened before the change must stop working after it, while the
// browser that made the change stays signed in.
func TestPasswordChangeEndsOtherSessions(t *testing.T) {
	server := httptest.NewServer(NewWebHandler(newRepo(), &fakeEvolution{}, health.NewState(), testSessionKey(), "https://mcp.example", "", false, "brorlandi.xyz", 3*time.Minute, nil))
	defer server.Close()
	client := func() *http.Client {
		jar, _ := cookiejar.New(nil)
		return &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	status := func(c *http.Client, path string) int {
		response, err := c.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		return response.StatusCode
	}
	post := func(c *http.Client, path string, form url.Values) {
		response, err := c.PostForm(server.URL+path, form)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
	}
	owner, intruder := client(), client()
	post(owner, "/setup", url.Values{"email": {"owner@example.com"}, "password": {"senha antiga 123"}})
	post(intruder, "/login", url.Values{"username": {"owner@example.com"}, "password": {"senha antiga 123"}})
	if got := status(intruder, "/configuracoes"); got != http.StatusOK {
		t.Fatalf("intruder before the change: %d", got)
	}
	post(owner, "/senha", url.Values{"current_password": {"senha antiga 123"}, "password": {"nova senha 456"}, "confirm_password": {"nova senha 456"}})
	if got := status(intruder, "/configuracoes"); got != http.StatusSeeOther {
		t.Fatalf("a session from before the password change still works: %d", got)
	}
	if got := status(owner, "/configuracoes"); got != http.StatusOK {
		t.Fatalf("the owner lost their own session: %d", got)
	}
}

// With no setup token, the loopback binding is what keeps strangers out of
// the first-run form, and a page in the operator's browser can still post to
// 127.0.0.1. The browser labels that post as cross-site; it must not create
// an administrator.
func TestSetupRefusesACrossSitePost(t *testing.T) {
	repo := newRepo()
	handler := NewWebHandler(repo, &fakeEvolution{}, health.NewState(), testSessionKey(), "https://mcp.example", "", false, "brorlandi.xyz", 3*time.Minute, nil)
	for _, headers := range []map[string]string{{"Sec-Fetch-Site": "cross-site"}, {"Origin": "https://attacker.example"}} {
		request := httptest.NewRequest(http.MethodPost, "/setup", strings.NewReader(url.Values{"email": {"attacker@example.com"}, "password": {"senha 123456"}}.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for k, v := range headers {
			request.Header.Set(k, v)
		}
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusForbidden || len(recorder.Result().Cookies()) != 0 {
			t.Fatalf("%v: %d, cookies %v", headers, recorder.Code, recorder.Result().Cookies())
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/setup", strings.NewReader(url.Values{"email": {"owner@example.com"}, "password": {"senha 123456"}}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("same-origin setup: %d %s", recorder.Code, recorder.Body.String())
	}
}
