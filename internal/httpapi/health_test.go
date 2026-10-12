package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/BrOrlandi/whatsapp-mcp/internal/health"
)

func TestProbesDoNotDiscloseTheAccount(t *testing.T) {
	state := health.NewState()
	state.SetDependencies(true, true, true)
	state.SetWhatsApp("connected", "", "5500000000000:7@s.whatsapp.net", "Dummy Owner")
	state.MarkMessage(time.Date(2026, 10, 12, 3, 4, 5, 0, time.UTC))
	handler := FullHandler(state, time.Minute, http.NotFoundHandler(), nil, nil)
	for _, path := range []string{"/healthz", "/readyz"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		body := recorder.Body.String()
		for _, leak := range []string{"5500000000000", "Dummy Owner", "push_name", "last_message_at"} {
			if strings.Contains(body, leak) {
				t.Fatalf("%s discloses %q: %s", path, leak, body)
			}
		}
		if !strings.Contains(body, `"state":"connected"`) || !strings.Contains(body, `"version"`) {
			t.Fatalf("%s lost the state or version: %s", path, body)
		}
	}
}

func TestHealthIsLiveWhileReadinessReportsStale(t *testing.T) {
	state := health.NewState()
	state.SetDependencies(false, true, true)
	handler := Handler(state, time.Minute)
	for _, tc := range []struct {
		path string
		want int
	}{{"/healthz", http.StatusOK}, {"/readyz", http.StatusServiceUnavailable}} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if recorder.Code != tc.want || !strings.Contains(recorder.Body.String(), "freshness is not trustworthy") {
			t.Fatalf("%s: %d %s", tc.path, recorder.Code, recorder.Body.String())
		}
	}
}
