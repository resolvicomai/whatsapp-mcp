package mcp

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/BrOrlandi/whatsapp-mcp/internal/store"
)

// perInstanceIndex answers EachMessage with each instance's own messages.
type perInstanceIndex struct {
	*fakeIndex
	byInstance map[string][]store.Message
}

func (p *perInstanceIndex) EachMessage(_ context.Context, instanceID string, _ store.Filter, fn func(store.Message) error) error {
	for _, m := range p.byInstance[instanceID] {
		if err := fn(m); err != nil {
			return err
		}
	}
	return nil
}

// Two instances share the export folder. An export one of them makes in the
// same second as the other must not take over the other's file, or a link
// handed to one key would serve another instance's messages.
func TestExportLinkServesOnlyItsOwnInstance(t *testing.T) {
	index := &perInstanceIndex{
		fakeIndex: &fakeIndex{tokens: map[string]string{"inst-A": "tok-A", "inst-B": "tok-B"}, selected: "inst-A"},
		byInstance: map[string][]store.Message{
			"inst-B": {{InstanceID: "inst-B", MessageID: "b1", ChatJID: "5511@s.whatsapp.net", Text: "only-for-B", SentAt: time.Now()}},
		},
	}
	server := New(index, &fakeLive{}, readyState(), time.Minute).WithFiles(t.TempDir(), t.TempDir()).WithPublicURL("https://mcp.example")
	export := func(instance string) map[string]any {
		encoded, _ := json.Marshal(map[string]any{"name": "export_messages", "arguments": map[string]any{}})
		ctx := WithSession(context.Background(), Session{InstanceID: instance, Token: "tok-" + strings.TrimPrefix(instance, "inst-")})
		var response map[string]any
		if err := json.Unmarshal(server.Handle(ctx, []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":`+string(encoded)+`}`)), &response); err != nil {
			t.Fatal(err)
		}
		result := response["result"].(map[string]any)
		var payload map[string]any
		_ = json.Unmarshal([]byte(result["content"].([]any)[0].(map[string]any)["text"].(string)), &payload)
		if result["isError"] == true {
			t.Fatalf("export as %s failed: %v", instance, payload)
		}
		return payload
	}
	// Start at the top of a second so both exports share it.
	time.Sleep(time.Until(time.Now().Truncate(time.Second).Add(time.Second)))
	a, b := export("inst-A"), export("inst-B")
	if a["file"] == b["file"] {
		t.Fatalf("both instances wrote %v", a["file"])
	}
	rec := httptest.NewRecorder()
	server.MediaHandler().ServeHTTP(rec, httptest.NewRequest("GET", "/media/"+strings.TrimPrefix(a["url"].(string), "https://mcp.example/media/"), nil))
	if strings.Contains(rec.Body.String(), "only-for-B") {
		t.Fatalf("instance A's link served instance B's export: %s", rec.Body.String())
	}
}
