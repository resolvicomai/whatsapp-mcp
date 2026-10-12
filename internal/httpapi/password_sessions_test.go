package httpapi

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
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
