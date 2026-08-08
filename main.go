package main

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/audioprompters/pigments-web-mvp/internal/arturia"
	"github.com/audioprompters/pigments-web-mvp/internal/knowledge"
	"github.com/audioprompters/pigments-web-mvp/internal/planner"
)

//go:embed web/index.html
var indexHTML []byte

const (
	version               = "0.3.0"
	defaultAddr           = "127.0.0.1:8080"
	defaultRetention      = 24 * time.Hour
	defaultMaxUpload      = int64(128 << 20)
	defaultRequestLimit   = int64(272 << 20)
	maxInstructionRunes   = 12000
	maxPatchNameRunes     = 80
	maxAuthorRunes        = 96
	maxCategoryRunes      = 80
	maxTags               = 20
	maxTagRunes           = 48
	maxParameterQuerySize = 500
)

type requestedMetadata struct {
	Author   string   `json:"author,omitempty"`
	Category string   `json:"category,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

type generatedJob struct {
	ID          string                    `json:"id"`
	Mode        string                    `json:"mode"`
	Filename    string                    `json:"filename"`
	DownloadURL string                    `json:"download_url"`
	ReportURL   string                    `json:"report_url"`
	Plan        arturia.PresetPlan        `json:"plan"`
	Report      *arturia.ApplyReport      `json:"report"`
	Inspection  *arturia.PresetInspection `json:"inspection,omitempty"`
	Metadata    requestedMetadata         `json:"requested_metadata,omitempty"`
	Planner     planner.Status            `json:"planner"`
	ExpiresAt   time.Time                 `json:"expires_at"`
	Path        string                    `json:"-"`
}

type appServer struct {
	planner     planner.Planner
	dataDir     string
	uploadDir   string
	outputDir   string
	retention   time.Duration
	maxUpload   int64
	jobs        map[string]*generatedJob
	jobsMu      sync.RWMutex
	rateLimiter *fixedWindowLimiter
}

type fixedWindowEntry struct {
	WindowStart time.Time
	Count       int
}

type fixedWindowLimiter struct {
	mu      sync.Mutex
	entries map[string]fixedWindowEntry
	limit   int
	window  time.Duration
}

func newFixedWindowLimiter(limit int, window time.Duration) *fixedWindowLimiter {
	return &fixedWindowLimiter{entries: map[string]fixedWindowEntry{}, limit: limit, window: window}
}

func (l *fixedWindowLimiter) Allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entries[key]
	if entry.WindowStart.IsZero() || now.Sub(entry.WindowStart) >= l.window {
		entry = fixedWindowEntry{WindowStart: now, Count: 0}
	}
	entry.Count++
	l.entries[key] = entry
	if len(l.entries) > 5000 {
		for candidate, value := range l.entries {
			if now.Sub(value.WindowStart) > 2*l.window {
				delete(l.entries, candidate)
			}
		}
	}
	return entry.Count <= l.limit
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	command := "serve"
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		command = os.Args[1]
		os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
	}

	var err error
	switch command {
	case "serve":
		err = runServe()
	case "inspect":
		err = runInspect()
	case "diff":
		err = runDiff()
	case "generate-plan":
		err = runGeneratePlan()
	case "modify-plan":
		err = runModifyPlan()
	case "version", "--version", "-version":
		fmt.Println(version)
	case "help", "--help", "-h":
		printUsage()
	default:
		printUsage()
		err = fmt.Errorf("unknown command %q", command)
	}
	if err != nil {
		log.Printf("error: %v", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`Audio Prompters — Pigments Web MVP %s

Usage:
  pigments-web serve [--addr %s] [--data-dir PATH] [--mock] [--open]
  pigments-web inspect --preset FILE.pgtx [--query TEXT] [--limit 100]
  pigments-web diff --before FILE.pgtx --after FILE.pgtx [--limit 500]
  pigments-web generate-plan --plan FILE.json [--output-dir PATH]
  pigments-web modify-plan --preset FILE.pgtx --plan FILE.json [--output-dir PATH]

Production AI mode reads OPENAI_API_KEY from the server environment and calls
the OpenAI Responses API. ChatGPT Pro and Codex are not runtime dependencies.
Use --mock for a deterministic local demonstration with no network request.
`, version, defaultAddr)
}

func runServe() error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", envOr("APP_ADDR", defaultAddr), "listen address")
	dataDir := fs.String("data-dir", envOr("DATA_DIR", "./data"), "private application data directory")
	mockMode := fs.Bool("mock", strings.EqualFold(os.Getenv("PLANNER_MODE"), "mock"), "use deterministic demo planner")
	open := fs.Bool("open", false, "open the website after startup")
	retention := fs.Duration("retention", defaultRetention, "generated-file retention")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	absoluteData, err := filepath.Abs(*dataDir)
	if err != nil {
		return err
	}
	app := &appServer{
		dataDir:     absoluteData,
		uploadDir:   filepath.Join(absoluteData, "uploads"),
		outputDir:   filepath.Join(absoluteData, "outputs"),
		retention:   *retention,
		maxUpload:   defaultMaxUpload,
		jobs:        map[string]*generatedJob{},
		rateLimiter: newFixedWindowLimiter(30, time.Minute),
	}
	if *mockMode {
		app.planner = planner.Mock{}
	} else {
		app.planner = planner.NewOpenAIFromEnv()
	}
	for _, dir := range []string{app.uploadDir, app.outputDir} {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return err
		}
	}
	app.cleanupExpiredFiles(time.Now())

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", app.handleIndex)
	mux.HandleFunc("GET /healthz", app.handleHealth)
	mux.HandleFunc("GET /api/status", app.handleStatus)
	mux.HandleFunc("GET /api/knowledge", app.handleKnowledge)
	mux.HandleFunc("GET /api/parameters", app.handleParameters)
	mux.HandleFunc("POST /api/generate", app.handleGenerate)
	mux.HandleFunc("POST /api/modify", app.handleModify)
	mux.HandleFunc("POST /api/inspect", app.handleInspect)
	mux.HandleFunc("POST /api/research/diff", app.handleDiff)
	mux.HandleFunc("GET /api/download/{id}", app.handleDownload)
	mux.HandleFunc("GET /api/report/{id}", app.handleReport)

	server := &http.Server{
		Addr:              *addr,
		Handler:           app.securityMiddleware(app.rateLimitMiddleware(loggingMiddleware(mux))),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       6 * time.Minute,
		WriteTimeout:      6 * time.Minute,
		IdleTimeout:       75 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	baseURL := "http://" + *addr
	log.Printf("Audio Prompters Pigments Web MVP %s listening on %s", version, baseURL)
	log.Printf("Planner: %+v", app.planner.Status(context.Background()))
	log.Printf("Private data directory: %s", absoluteData)
	if *open {
		go func() {
			time.Sleep(350 * time.Millisecond)
			_ = openBrowser(baseURL)
		}()
	}
	return server.ListenAndServe()
}

func runInspect() error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	presetPath := fs.String("preset", "", "source .pgtx file")
	query := fs.String("query", "", "parameter search")
	limit := fs.Int("limit", 100, "maximum parameters")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}
	if strings.TrimSpace(*presetPath) == "" {
		return errors.New("--preset is required")
	}
	inspection, err := arturia.InspectPresetFile(*presetPath, *query, *limit)
	if err != nil {
		return err
	}
	return encodePretty(os.Stdout, inspection)
}

func runDiff() error {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	before := fs.String("before", "", "baseline .pgtx")
	after := fs.String("after", "", "changed .pgtx")
	limit := fs.Int("limit", 500, "maximum differences")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}
	if strings.TrimSpace(*before) == "" || strings.TrimSpace(*after) == "" {
		return errors.New("--before and --after are required")
	}
	report, err := arturia.DiffPresetFiles(*before, *after, *limit)
	if err != nil {
		return err
	}
	return encodePretty(os.Stdout, report)
}

func runGeneratePlan() error {
	fs := flag.NewFlagSet("generate-plan", flag.ContinueOnError)
	planPath := fs.String("plan", "", "structured plan JSON")
	outputDir := fs.String("output-dir", ".", "output directory")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}
	plan, err := loadPlan(*planPath, "generate")
	if err != nil {
		return err
	}
	report, err := arturia.GeneratePreset(*plan, *outputDir)
	if err != nil {
		return err
	}
	return encodePretty(os.Stdout, report)
}

func runModifyPlan() error {
	fs := flag.NewFlagSet("modify-plan", flag.ContinueOnError)
	presetPath := fs.String("preset", "", "source .pgtx file")
	planPath := fs.String("plan", "", "structured plan JSON")
	outputDir := fs.String("output-dir", ".", "output directory")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}
	if strings.TrimSpace(*presetPath) == "" {
		return errors.New("--preset is required")
	}
	plan, err := loadPlan(*planPath, "modify")
	if err != nil {
		return err
	}
	report, err := arturia.ModifyPreset(*presetPath, *plan, *outputDir)
	if err != nil {
		return err
	}
	return encodePretty(os.Stdout, report)
}

func loadPlan(path, mode string) (*arturia.PresetPlan, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("--plan is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var plan arturia.PresetPlan
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&plan); err != nil {
		return nil, err
	}
	if err := planner.ValidatePlan(&plan, mode); err != nil {
		return nil, err
	}
	return &plan, nil
}

func (a *appServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(indexHTML)
}

func (a *appServer) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "version": version})
}

func (a *appServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	catalog := knowledge.Load()
	status := map[string]any{
		"version":                       version,
		"planner":                       a.planner.Status(r.Context()),
		"target_pigments_version":       catalog.TargetVersion,
		"knowledge_schema":              catalog.SchemaVersion,
		"master_database":               knowledge.MasterDatabaseSummary(),
		"compiler_safe_parameter_count": writeSafeParameterCount(),
		"max_upload_bytes":              a.maxUpload,
		"retention_hours":               int(a.retention.Hours()),
		"features": []string{
			"generate_pgtx", "modify_pgtx", "inspect_pgtx", "parameter_diff", "knowledge_search", "master_database_search",
		},
	}
	writeJSON(w, http.StatusOK, status)
}

func (a *appServer) handleKnowledge(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(knowledge.JSON())
}

type parameterSearchResult struct {
	Kind        string   `json:"kind"`
	ID          string   `json:"id"`
	Friendly    string   `json:"friendly"`
	Aliases     []string `json:"aliases,omitempty"`
	Unit        string   `json:"unit"`
	Minimum     float64  `json:"minimum"`
	Maximum     float64  `json:"maximum"`
	Description string   `json:"description,omitempty"`
	WriteSafe   bool     `json:"write_safe"`
}

func writeSafeParameterCount() int {
	count := 0
	for _, spec := range arturia.KnownParameterSpecs {
		if knowledge.AutomaticEditAllowed(spec.ID) {
			count++
		}
	}
	return count
}

func (a *appServer) handleParameters(w http.ResponseWriter, r *http.Request) {
	query := cleanText(r.URL.Query().Get("q"), maxParameterQuerySize)
	limit := parseBoundedInt(r.URL.Query().Get("limit"), 80, 1, 300)
	q := strings.ToLower(strings.TrimSpace(query))
	tokens := strings.Fields(q)
	type scored struct {
		Result parameterSearchResult
		Score  int
	}
	all := make([]scored, 0)
	for _, spec := range arturia.KnownParameterSpecs {
		if !knowledge.AutomaticEditAllowed(spec.ID) {
			continue
		}
		text := strings.ToLower(spec.ID + " " + spec.Friendly + " " + strings.Join(spec.Aliases, " ") + " " + spec.Description)
		score := 1
		if q != "" {
			score = 0
			if strings.EqualFold(q, spec.ID) {
				score += 1000
			}
			if strings.Contains(text, q) {
				score += 100
			}
			for _, token := range tokens {
				if strings.Contains(text, token) {
					score += 10
				}
			}
		}
		if score > 0 {
			all = append(all, scored{Result: parameterSearchResult{Kind: "curated_write_safe_parameter", ID: spec.ID, Friendly: spec.Friendly, Aliases: spec.Aliases, Unit: spec.Unit, Minimum: spec.Min, Maximum: spec.Max, Description: spec.Description, WriteSafe: true}, Score: score})
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Score != all[j].Score {
			return all[i].Score > all[j].Score
		}
		return all[i].Result.ID < all[j].Result.ID
	})
	if len(all) > limit {
		all = all[:limit]
	}
	results := make([]parameterSearchResult, len(all))
	for i := range all {
		results[i] = all[i].Result
	}
	masterResults := knowledge.SearchMaster(query, limit)
	writeSafeIDs := make(map[string]bool, len(results))
	for _, result := range results {
		writeSafeIDs[result.ID] = true
	}
	filteredMaster := masterResults[:0]
	for _, result := range masterResults {
		if result.Kind == "observed_internal_parameter" && writeSafeIDs[result.ID] {
			continue
		}
		filteredMaster = append(filteredMaster, result)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"query":          query,
		"results":        results,
		"ui_knowledge":   knowledge.Search(query, limit),
		"master_results": filteredMaster,
		"master_summary": knowledge.GetMasterSummary(),
		"trust_note":     "Write-safe parameters are curated mappings. Documented UI controls and observed internal IDs remain research evidence until their mapping and conversion are verified.",
	})
}

func (a *appServer) handleGenerate(w http.ResponseWriter, r *http.Request) {
	if !a.requireSameOrigin(w, r) {
		return
	}
	if err := a.parseMultipart(w, r); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	instruction := cleanText(r.FormValue("instruction"), maxInstructionRunes)
	if instruction == "" {
		writeError(w, http.StatusBadRequest, errors.New("sound description is required"))
		return
	}
	requestedName := cleanText(r.FormValue("patch_name"), maxPatchNameRunes)
	metadata := requestedMetadata{
		Author:   cleanText(r.FormValue("author"), maxAuthorRunes),
		Category: cleanText(r.FormValue("category"), maxCategoryRunes),
		Tags:     cleanTags(r.FormValue("tags")),
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	plan, err := a.planner.Plan(ctx, planner.Request{
		Mode:        "generate",
		Instruction: instruction,
		Author:      metadata.Author,
		Category:    metadata.Category,
		Tags:        metadata.Tags,
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Errorf("sound-design planning failed: %w", err))
		return
	}
	if requestedName != "" {
		plan.PatchName = requestedName
	}
	plan.BankOverride = "User"
	plan.AuthorOverride = metadata.Author
	if err := planner.ValidatePlan(plan, "generate"); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	report, err := arturia.GeneratePreset(*plan, a.outputDir)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("preset generation failed: %w", err))
		return
	}
	job, err := a.registerJob("generate", *plan, report, metadata)
	if err != nil {
		_ = os.Remove(report.OutputPath)
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (a *appServer) handleModify(w http.ResponseWriter, r *http.Request) {
	if !a.requireSameOrigin(w, r) {
		return
	}
	if err := a.parseMultipart(w, r); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !formBool(r.FormValue("rights_confirmed")) {
		writeError(w, http.StatusBadRequest, errors.New("confirm that you have the right to modify the uploaded preset"))
		return
	}
	instruction := cleanText(r.FormValue("instruction"), maxInstructionRunes)
	if instruction == "" {
		writeError(w, http.StatusBadRequest, errors.New("modification instruction is required"))
		return
	}
	path, originalName, err := a.saveUploadedFile(r, "preset")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	defer os.Remove(path)

	if _, err := arturia.InspectPresetFile(path, "", 500); err != nil {
		writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("invalid Pigments preset: %w", err))
		return
	}
	contextJSON, err := buildPresetContext(path, instruction)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	metadata := requestedMetadata{
		Category: cleanText(r.FormValue("category"), maxCategoryRunes),
		Tags:     cleanTags(r.FormValue("tags")),
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	plan, err := a.planner.Plan(ctx, planner.Request{
		Mode:          "modify",
		Instruction:   instruction,
		PresetContext: contextJSON,
		Category:      metadata.Category,
		Tags:          metadata.Tags,
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Errorf("sound-design planning failed: %w", err))
		return
	}
	// Preserve the existing visible identity by default. Only an explicit value
	// in the dedicated rename field is allowed to rename the uploaded preset.
	plan.PatchName = cleanText(r.FormValue("new_name"), maxPatchNameRunes)
	lowerInstruction := strings.ToLower(instruction)
	if !strings.Contains(lowerInstruction, "macro") || (!strings.Contains(lowerInstruction, "name") && !strings.Contains(lowerInstruction, "label")) {
		plan.Macros = arturia.MacroNames{}
	}
	if err := planner.ValidatePlan(plan, "modify"); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	report, err := arturia.ModifyPreset(path, *plan, a.outputDir)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("preset modification failed: %w", err))
		return
	}
	report.SourcePath = originalName
	job, err := a.registerJob("modify", *plan, report, metadata)
	if err != nil {
		_ = os.Remove(report.OutputPath)
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (a *appServer) handleInspect(w http.ResponseWriter, r *http.Request) {
	if !a.requireSameOrigin(w, r) {
		return
	}
	if err := a.parseMultipart(w, r); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	path, originalName, err := a.saveUploadedFile(r, "preset")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	defer os.Remove(path)
	query := cleanText(r.FormValue("query"), maxParameterQuerySize)
	limit := parseBoundedInt(r.FormValue("limit"), 100, 1, 500)
	inspection, err := arturia.InspectPresetFile(path, query, limit)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	inspection.Path = originalName
	writeJSON(w, http.StatusOK, inspection)
}

func (a *appServer) handleDiff(w http.ResponseWriter, r *http.Request) {
	if !a.requireSameOrigin(w, r) {
		return
	}
	if err := a.parseMultipart(w, r); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	beforePath, _, err := a.saveUploadedFile(r, "before")
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("baseline preset: %w", err))
		return
	}
	defer os.Remove(beforePath)
	afterPath, _, err := a.saveUploadedFile(r, "after")
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("changed preset: %w", err))
		return
	}
	defer os.Remove(afterPath)
	limit := parseBoundedInt(r.FormValue("limit"), 500, 1, 5000)
	report, err := arturia.DiffPresetFiles(beforePath, afterPath, limit)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (a *appServer) handleDownload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job := a.lookupJob(id)
	if job == nil || time.Now().After(job.ExpiresAt) {
		http.NotFound(w, r)
		return
	}
	file, err := os.Open(job.Path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", job.Filename))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, job.Filename, info.ModTime(), file)
}

func (a *appServer) handleReport(w http.ResponseWriter, r *http.Request) {
	job := a.lookupJob(r.PathValue("id"))
	if job == nil || time.Now().After(job.ExpiresAt) {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (a *appServer) parseMultipart(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, defaultRequestLimit)
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		return fmt.Errorf("invalid multipart request: %w", err)
	}
	return nil
}

func (a *appServer) saveUploadedFile(r *http.Request, field string) (path string, originalName string, err error) {
	file, header, err := r.FormFile(field)
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return "", "", fmt.Errorf("%s .pgtx file is required", field)
		}
		return "", "", err
	}
	defer file.Close()
	originalName = filepath.Base(strings.ReplaceAll(header.Filename, "\\", "/"))
	if !strings.EqualFold(filepath.Ext(originalName), ".pgtx") {
		return "", "", errors.New("only .pgtx files are accepted")
	}
	id, err := randomID(16)
	if err != nil {
		return "", "", err
	}
	path = filepath.Join(a.uploadDir, id+".pgtx")
	out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", "", err
	}
	written, copyErr := io.Copy(out, io.LimitReader(file, a.maxUpload+1))
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(path)
		return "", "", copyErr
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return "", "", closeErr
	}
	if written <= 0 || written > a.maxUpload {
		_ = os.Remove(path)
		return "", "", fmt.Errorf("preset must be between 1 byte and %d MB", a.maxUpload>>20)
	}
	return path, originalName, nil
}

func buildPresetContext(path, instruction string) (string, error) {
	preset, err := arturia.LoadPGTX(path)
	if err != nil {
		return "", err
	}
	base := preset.Inspect("", 500)
	relevant := preset.Inspect(instruction, 250)
	seen := map[string]bool{}
	parameters := make([]arturia.ParameterView, 0, len(base.Parameters)+len(relevant.Parameters))
	for _, list := range [][]arturia.ParameterView{relevant.Parameters, base.Parameters} {
		for _, parameter := range list {
			if !seen[parameter.ID] {
				seen[parameter.ID] = true
				parameters = append(parameters, parameter)
			}
		}
	}
	contextData := map[string]any{
		"metadata":          preset.Metadata,
		"inner_path":        preset.InnerPath,
		"parameter_count":   base.ParameterCount,
		"parameters":        parameters,
		"preservation_rule": "All parameters and safe archive entries not listed in the returned change plan must remain byte-for-byte equivalent where possible.",
	}
	data, err := json.Marshal(contextData)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *appServer) registerJob(mode string, plan arturia.PresetPlan, report *arturia.ApplyReport, metadata requestedMetadata) (*generatedJob, error) {
	id, err := randomID(16)
	if err != nil {
		return nil, err
	}
	path := report.OutputPath
	inspection, err := arturia.InspectPresetFile(path, "", 120)
	if err != nil {
		return nil, fmt.Errorf("generated preset did not pass final inspection: %w", err)
	}
	filename := filepath.Base(path)
	inspection.Path = filename
	publicReport := *report
	publicReport.OutputPath = filename
	if publicReport.SourcePath != "" {
		publicReport.SourcePath = filepath.Base(publicReport.SourcePath)
	}
	job := &generatedJob{
		ID:          id,
		Mode:        mode,
		Filename:    filename,
		DownloadURL: "/api/download/" + id,
		ReportURL:   "/api/report/" + id,
		Plan:        plan,
		Report:      &publicReport,
		Inspection:  inspection,
		Metadata:    metadata,
		Planner:     a.planner.Status(context.Background()),
		ExpiresAt:   time.Now().Add(a.retention),
		Path:        path,
	}
	a.jobsMu.Lock()
	a.jobs[id] = job
	a.jobsMu.Unlock()
	return job, nil
}

func (a *appServer) lookupJob(id string) *generatedJob {
	if len(id) != 32 {
		return nil
	}
	a.jobsMu.RLock()
	job := a.jobs[id]
	a.jobsMu.RUnlock()
	return job
}

func (a *appServer) cleanupExpiredFiles(now time.Time) {
	entries, err := os.ReadDir(a.outputDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			info, infoErr := entry.Info()
			if infoErr == nil && now.Sub(info.ModTime()) > a.retention {
				_ = os.Remove(filepath.Join(a.outputDir, entry.Name()))
			}
		}
	}
	entries, err = os.ReadDir(a.uploadDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			info, infoErr := entry.Info()
			if infoErr == nil && now.Sub(info.ModTime()) > time.Hour {
				_ = os.Remove(filepath.Join(a.uploadDir, entry.Name()))
			}
		}
	}
}

func (a *appServer) requireSameOrigin(w http.ResponseWriter, r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || !strings.EqualFold(parsed.Host, r.Host) {
		writeError(w, http.StatusForbidden, errors.New("cross-origin write request rejected"))
		return false
	}
	return true
}

func (a *appServer) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			key := clientIP(r)
			if !a.rateLimiter.Allow(key, time.Now()) {
				w.Header().Set("Retry-After", "60")
				writeError(w, http.StatusTooManyRequests, errors.New("request limit reached; retry shortly"))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (a *appServer) securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

func cleanText(value string, maxRunes int) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\x00", ""))
	runes := []rune(value)
	if len(runes) > maxRunes {
		value = string(runes[:maxRunes])
	}
	return value
}

func cleanTags(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' || r == '\n' })
	seen := map[string]bool{}
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = cleanText(part, maxTagRunes)
		key := strings.ToLower(part)
		if part != "" && !seen[key] {
			seen[key] = true
			result = append(result, part)
			if len(result) >= maxTags {
				break
			}
		}
	}
	return result
}

func formBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on", "confirmed":
		return true
	default:
		return false
	}
}

func parseBoundedInt(value string, fallback, min, max int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	if parsed < min {
		return min
	}
	if parsed > max {
		return max
	}
	return parsed
}

func randomID(bytesCount int) (string, error) {
	data := make([]byte, bytesCount)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(true)
	_ = encoder.Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"error": err.Error(), "status": status})
}

func encodePretty(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func openBrowser(target string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", target)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		command = exec.Command("xdg-open", target)
	}
	return command.Start()
}
