package rhizome_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ybordag/cambium/internal/rhizome"
)

// fakeRhizome spins up a test server that stands in for Rhizome's internal API.
func fakeRhizome(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *rhizome.Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	// Override RHIZOME_INTERNAL_URL via env so Client.New() picks it up.
	t.Setenv("RHIZOME_INTERNAL_URL", srv.URL)
	return srv, rhizome.New()
}

func TestRunAgent_Success(t *testing.T) {
	_, client := fakeRhizome(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/agent" || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"thread_id": "thread-1",
			"response":  "Water your tomatoes today.",
		})
	})

	resp, err := client.RunAgent(rhizome.AgentRequest{
		UserID:   "user-1",
		ThreadID: "thread-1",
		Message:  "What should I do today?",
	})
	if err != nil {
		t.Fatalf("RunAgent: %v", err)
	}
	if resp.Response != "Water your tomatoes today." {
		t.Errorf("got response %q", resp.Response)
	}
	if resp.ThreadID != "thread-1" {
		t.Errorf("got thread_id %q", resp.ThreadID)
	}
}

func TestRunAgent_RhizomeError(t *testing.T) {
	_, client := fakeRhizome(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal error"}`))
	})

	_, err := client.RunAgent(rhizome.AgentRequest{UserID: "u", ThreadID: "t", Message: "hi"})
	if err == nil {
		t.Error("expected error for non-200, got nil")
	}
}

func TestStreamAgent_ForwardsBody(t *testing.T) {
	ssePayload := "data: {\"type\":\"token\",\"content\":\"Hello\"}\n\ndata: {\"type\":\"done\"}\n\n"

	_, client := fakeRhizome(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/agent/stream" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(ssePayload))
	})

	body, err := client.StreamAgent(rhizome.AgentRequest{UserID: "u", ThreadID: "t", Message: "hi"})
	if err != nil {
		t.Fatalf("StreamAgent: %v", err)
	}
	defer body.Close()

	got, _ := io.ReadAll(body)
	if string(got) != ssePayload {
		t.Errorf("got SSE body %q, want %q", got, ssePayload)
	}
}

func TestDataGet_ForwardsUserID(t *testing.T) {
	_, client := fakeRhizome(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("user_id") != "user-42" {
			t.Errorf("user_id not forwarded: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	})

	body, status, err := client.DataGet("alerts", "user-42", nil)
	if err != nil {
		t.Fatalf("DataGet: %v", err)
	}
	defer body.Close()
	if status != http.StatusOK {
		t.Errorf("got status %d", status)
	}
}

func TestDataPost_SendsJSON(t *testing.T) {
	_, client := fakeRhizome(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	body, status, err := client.DataPost("alerts/alert-1/dismiss", "user-1", map[string]any{})
	if err != nil {
		t.Fatalf("DataPost: %v", err)
	}
	defer body.Close()
	if status != http.StatusOK {
		t.Errorf("got status %d", status)
	}
}

func TestDataDelete_SendsDeleteMethod(t *testing.T) {
	_, client := fakeRhizome(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Query().Get("user_id") != "user-3" {
			t.Errorf("user_id not forwarded: %s", r.URL.RawQuery)
		}
		if r.ContentLength > 0 {
			t.Errorf("expected no body on DELETE, got content-length %d", r.ContentLength)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"deleted"}`))
	})

	body, status, err := client.DataDelete("threads/thread-1/context/plant/p1", "user-3")
	if err != nil {
		t.Fatalf("DataDelete: %v", err)
	}
	defer body.Close()
	if status != http.StatusOK {
		t.Errorf("got status %d", status)
	}
}

func TestDataRequest_PatchSendsPatchMethod(t *testing.T) {
	_, client := fakeRhizome(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH, got %s", r.Method)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["status"] != "resolved" {
			t.Errorf("body not forwarded: %v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	body, status, err := client.DataRequest(http.MethodPatch, "incidents/inc-1", "user-1", nil, map[string]any{"status": "resolved"})
	if err != nil {
		t.Fatalf("DataRequest PATCH: %v", err)
	}
	defer body.Close()
	if status != http.StatusOK {
		t.Errorf("got status %d", status)
	}
}
