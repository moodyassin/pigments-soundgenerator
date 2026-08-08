package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/audioprompters/pigments-web-mvp/internal/arturia"
	"github.com/audioprompters/pigments-web-mvp/internal/planner"
)

func newTestApp(t *testing.T) *appServer {
	t.Helper()
	root := t.TempDir()
	uploadDir := filepath.Join(root, "uploads")
	outputDir := filepath.Join(root, "outputs")
	for _, dir := range []string{uploadDir, outputDir} {
		if err := os.MkdirAll(dir, 0750); err != nil {
			t.Fatal(err)
		}
	}
	return &appServer{
		planner:     planner.Mock{},
		dataDir:     root,
		uploadDir:   uploadDir,
		outputDir:   outputDir,
		retention:   time.Hour,
		maxUpload:   defaultMaxUpload,
		jobs:        map[string]*generatedJob{},
		rateLimiter: newFixedWindowLimiter(1000, time.Minute),
	}
}

func TestGenerateHTTPWorkflowCreatesDownloadableValidatedPreset(t *testing.T) {
	app := newTestApp(t)
	request := multipartRequest(t, "/api/generate", map[string]string{
		"instruction": "Create a dark bass with a controlled release.",
		"patch_name":  "Night Reactor",
		"author":      "Test Artist",
		"category":    "Bass",
		"tags":        "dark, test",
	}, nil)
	response := httptest.NewRecorder()
	app.handleGenerate(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("generate status=%d body=%s", response.Code, response.Body.String())
	}
	var job generatedJob
	if err := json.NewDecoder(response.Body).Decode(&job); err != nil {
		t.Fatal(err)
	}
	if job.ID == "" || job.Filename == "" || job.DownloadURL == "" {
		t.Fatalf("incomplete job: %+v", job)
	}
	stored := app.lookupJob(job.ID)
	if stored == nil {
		t.Fatal("job was not registered")
	}
	inspection, err := arturia.InspectPresetFile(stored.Path, "Filter1_Cutoff", 20)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Metadata.Name != "Night Reactor" || inspection.Metadata.Author != "Test Artist" {
		t.Fatalf("unexpected metadata: %+v", inspection.Metadata)
	}

	downloadRequest := httptest.NewRequest(http.MethodGet, job.DownloadURL, nil)
	downloadRequest.SetPathValue("id", job.ID)
	downloadResponse := httptest.NewRecorder()
	app.handleDownload(downloadResponse, downloadRequest)
	if downloadResponse.Code != http.StatusOK {
		t.Fatalf("download status=%d", downloadResponse.Code)
	}
	if !bytes.HasPrefix(downloadResponse.Body.Bytes(), []byte("PK")) {
		t.Fatal("download is not a ZIP-based .pgtx")
	}
}

func TestModifyHTTPWorkflowPreservesSourceAndMetadata(t *testing.T) {
	app := newTestApp(t)
	preset, err := arturia.NewFromDefault()
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = preset.ApplyPlan(arturia.PresetPlan{Changes: []arturia.ParameterChange{{
		ParameterID: "Filter1_Cutoff", Operation: "set", Value: "1000", Unit: "hz",
	}}}, false)
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "Owned Bass.pgtx")
	if err := preset.Save(source); err != nil {
		t.Fatal(err)
	}
	beforeBytes, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	beforeHash := sha256.Sum256(beforeBytes)
	before, err := arturia.InspectPresetFile(source, "Filter1_Cutoff", 10)
	if err != nil {
		t.Fatal(err)
	}

	request := multipartRequest(t, "/api/modify", map[string]string{
		"instruction":      "Raise Filter 1 cutoff by 200 Hz and preserve everything else.",
		"rights_confirmed": "true",
	}, map[string]string{"preset": source})
	response := httptest.NewRecorder()
	app.handleModify(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("modify status=%d body=%s", response.Code, response.Body.String())
	}
	var job generatedJob
	if err := json.NewDecoder(response.Body).Decode(&job); err != nil {
		t.Fatal(err)
	}
	afterBytes, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(afterBytes) != beforeHash {
		t.Fatal("source preset was overwritten")
	}
	stored := app.lookupJob(job.ID)
	if stored == nil {
		t.Fatal("modified job missing")
	}
	after, err := arturia.InspectPresetFile(stored.Path, "Filter1_Cutoff", 10)
	if err != nil {
		t.Fatal(err)
	}
	if after.Metadata != before.Metadata {
		t.Fatalf("metadata changed: got %+v want %+v", after.Metadata, before.Metadata)
	}
	if len(job.Report.Changes) != 1 || job.Report.Changes[0].ParameterID != "Filter1_Cutoff" {
		t.Fatalf("unexpected changes: %+v", job.Report.Changes)
	}
}

func TestModifyRequiresRightsConfirmation(t *testing.T) {
	app := newTestApp(t)
	preset, err := arturia.NewFromDefault()
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "source.pgtx")
	if err := preset.Save(source); err != nil {
		t.Fatal(err)
	}
	request := multipartRequest(t, "/api/modify", map[string]string{"instruction": "Darker."}, map[string]string{"preset": source})
	response := httptest.NewRecorder()
	app.handleModify(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "right to modify") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestPresetDiffHTTPFindsSingleControlledChange(t *testing.T) {
	app := newTestApp(t)
	baseline, err := arturia.NewFromDefault()
	if err != nil {
		t.Fatal(err)
	}
	beforePath := filepath.Join(t.TempDir(), "before.pgtx")
	if err := baseline.Save(beforePath); err != nil {
		t.Fatal(err)
	}
	afterPreset, err := baseline.Clone()
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = afterPreset.ApplyPlan(arturia.PresetPlan{Changes: []arturia.ParameterChange{{
		ParameterID: "Engine1_SampleGranularOsc_BitCrushDecimate", Operation: "set", Value: "40", Unit: "percent",
	}}}, false)
	if err != nil {
		t.Fatal(err)
	}
	afterPath := filepath.Join(t.TempDir(), "after.pgtx")
	if err := afterPreset.Save(afterPath); err != nil {
		t.Fatal(err)
	}

	request := multipartRequest(t, "/api/research/diff", nil, map[string]string{"before": beforePath, "after": afterPath})
	response := httptest.NewRecorder()
	app.handleDiff(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("diff status=%d body=%s", response.Code, response.Body.String())
	}
	var report arturia.PresetDiff
	if err := json.NewDecoder(response.Body).Decode(&report); err != nil {
		t.Fatal(err)
	}
	if report.ChangeCount != 1 {
		t.Fatalf("change count=%d changes=%+v", report.ChangeCount, report.Changes)
	}
	if report.Changes[0].ParameterID != "Engine1_SampleGranularOsc_BitCrushDecimate" {
		t.Fatalf("unexpected diff: %+v", report.Changes[0])
	}
}

func TestInspectHTTPRejectsWrongExtension(t *testing.T) {
	app := newTestApp(t)
	path := filepath.Join(t.TempDir(), "not-a-preset.txt")
	if err := os.WriteFile(path, []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	request := multipartRequest(t, "/api/inspect", nil, map[string]string{"preset": path})
	response := httptest.NewRecorder()
	app.handleInspect(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestSecurityMiddlewareAndOriginCheck(t *testing.T) {
	app := newTestApp(t)
	request := httptest.NewRequest(http.MethodGet, "http://example.test/healthz", nil)
	response := httptest.NewRecorder()
	app.securityMiddleware(http.HandlerFunc(app.handleHealth)).ServeHTTP(response, request)
	if got := response.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options=%q", got)
	}

	crossOrigin := httptest.NewRequest(http.MethodPost, "http://example.test/api/generate", strings.NewReader(""))
	crossOrigin.Header.Set("Origin", "https://evil.example")
	crossResponse := httptest.NewRecorder()
	if app.requireSameOrigin(crossResponse, crossOrigin) {
		t.Fatal("cross-origin request was allowed")
	}
	if crossResponse.Code != http.StatusForbidden {
		t.Fatalf("cross-origin status=%d", crossResponse.Code)
	}
}

func multipartRequest(t *testing.T, target string, fields map[string]string, files map[string]string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	for field, path := range files {
		part, err := writer.CreateFormFile(field, filepath.Base(path))
		if err != nil {
			t.Fatal(err)
		}
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		_, copyErr := io.Copy(part, file)
		_ = file.Close()
		if copyErr != nil {
			t.Fatal(copyErr)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "http://example.test"+target, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func TestParameterSearchExposesMasterKnowledgeLayers(t *testing.T) {
	app := newTestApp(t)
	request := httptest.NewRequest(http.MethodGet, "http://example.test/api/parameters?q=bitcrush&limit=120", nil)
	response := httptest.NewRecorder()
	app.handleParameters(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Results       []parameterSearchResult `json:"results"`
		MasterResults []map[string]any        `json:"master_results"`
		MasterSummary map[string]any          `json:"master_summary"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Results) == 0 || len(payload.MasterResults) == 0 {
		t.Fatalf("missing search layers: safe=%d master=%d", len(payload.Results), len(payload.MasterResults))
	}
	for _, result := range payload.Results {
		if result.ID == "Engine1_SampleGranularOsc_BitCrushMode" || result.ID == "Engine2_SampleGranularOsc_BitCrushMode" {
			t.Fatalf("calibration-locked BitCrushMode leaked into write-safe API results: %+v", result)
		}
	}
	if got := int(payload.MasterSummary["internal_parameter_count"].(float64)); got != 3525 {
		t.Fatalf("internal parameter summary=%d", got)
	}
}

func TestParameterSearchIncludesZeroMinimum(t *testing.T) {
	app := newTestApp(t)
	request := httptest.NewRequest(http.MethodGet, "http://example.test/api/parameters?q=Engine1_SampleGranularOsc_EnvelopeParam&limit=5", nil)
	response := httptest.NewRecorder()
	app.handleParameters(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"minimum":0`) {
		t.Fatalf("zero minimum was omitted from parameter JSON: %s", response.Body.String())
	}
}
