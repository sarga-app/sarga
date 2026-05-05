//nolint:testpackage // tests cover unexported writeJSON.
package server

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteJSON_SetsContentTypeStatusAndBody(t *testing.T) {
	t.Parallel()
	rr := httptest.NewRecorder()
	writeJSON(rr, 201, map[string]string{"ok": "yes"})

	if rr.Code != 201 {
		t.Errorf("status = %d, want 201", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	body := rr.Body.String()
	if !strings.HasSuffix(body, "\n") {
		t.Errorf("expected trailing newline (json.Encoder appends one), got %q", body)
	}
	if !strings.Contains(body, `"ok":"yes"`) {
		t.Errorf("body %q missing field", body)
	}
}
