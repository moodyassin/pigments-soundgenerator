package planner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOpenAIPlanUsesResponsesStructuredOutput(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("path=%q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("authorization=%q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		plan := `{"patch_name":"Schema Bass","summary":"A controlled bass.","macro_names":{"macro1":"Tone","macro2":"Motion","macro3":"Space","macro4":"Drive"},"changes":[{"parameter_id":"Filter1_Cutoff","operation":"set","value":"1200","unit":"hz","reason":"Darken the tone.","allow_add":false}],"warnings":[]}`
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "completed",
			"output": []any{map[string]any{
				"type":    "message",
				"content": []any{map[string]any{"type": "output_text", "text": plan}},
			}},
		})
	}))
	defer server.Close()

	client := &OpenAI{
		APIKey:       "test-key",
		BaseURL:      server.URL + "/v1",
		DefaultModel: "gpt-5.6-terra",
		Client:       &http.Client{Timeout: 5 * time.Second},
	}
	plan, err := client.Plan(context.Background(), Request{Mode: "generate", Instruction: "Create a dark bass."})
	if err != nil {
		t.Fatal(err)
	}
	if plan.PatchName != "Schema Bass" || len(plan.Changes) != 1 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	if received["model"] != "gpt-5.6-terra" {
		t.Fatalf("model=%v", received["model"])
	}
	if received["store"] != false {
		t.Fatalf("store=%v", received["store"])
	}
	text, ok := received["text"].(map[string]any)
	if !ok {
		t.Fatalf("missing text config: %#v", received["text"])
	}
	format, ok := text["format"].(map[string]any)
	if !ok || format["type"] != "json_schema" || format["strict"] != true {
		t.Fatalf("invalid structured-output format: %#v", text["format"])
	}
	instructions, _ := received["instructions"].(string)
	if !strings.Contains(instructions, "deterministic server") || !strings.Contains(instructions, "VERIFIED_PARAMETER_CATALOG") {
		t.Fatalf("instructions missing safety rules")
	}
}

func TestOpenAIPlanRequiresAPIKey(t *testing.T) {
	client := &OpenAI{DefaultModel: "gpt-5.6"}
	_, err := client.Plan(context.Background(), Request{Mode: "generate", Instruction: "Pad"})
	if err == nil || !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Fatalf("error=%v", err)
	}
	status := client.Status(context.Background())
	if status.Ready {
		t.Fatal("planner reported ready without API key")
	}
}
