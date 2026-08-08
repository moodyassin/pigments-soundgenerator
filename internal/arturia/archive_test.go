package arturia

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestDefaultTemplateParsesAndBuildsConservativePGTX(t *testing.T) {
	preset, err := NewFromDefault()
	if err != nil {
		t.Fatalf("NewFromDefault: %v", err)
	}
	if got, want := len(preset.block.params), 3335; got != want {
		t.Fatalf("parameter count = %d, want %d", got, want)
	}
	for _, id := range []string{
		"Engine1_ModuleType",
		"Engine2_ModuleType",
		"Filter1_Cutoff",
		"FilterMix_Engine1Volume",
		"MasterVolume",
	} {
		if _, ok := preset.block.params[id]; !ok {
			t.Errorf("default template is missing required parameter %q", id)
		}
	}

	data, err := preset.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	reparsed, err := ParsePGTX(data)
	if err != nil {
		t.Fatalf("ParsePGTX(round trip): %v", err)
	}
	if got, want := len(reparsed.block.params), len(preset.block.params); got != want {
		t.Fatalf("round-trip parameter count = %d, want %d", got, want)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	if got, want := len(zr.File), 2; got != want {
		t.Fatalf("archive entry count = %d, want %d", got, want)
	}
	for _, file := range zr.File {
		if file.Method != zip.Store {
			t.Errorf("entry %q compression method = %d, want Store", file.Name, file.Method)
		}
		if file.CreatorVersion>>8 != 0 {
			t.Errorf("entry %q creator system = %d, want FAT/0", file.Name, file.CreatorVersion>>8)
		}
		if file.Flags != 0 {
			t.Errorf("entry %q flags = %d, want 0", file.Name, file.Flags)
		}
		if file.ExternalAttrs != 0 {
			t.Errorf("entry %q external attrs = %d, want 0", file.Name, file.ExternalAttrs)
		}
		if len(file.Extra) != 0 {
			t.Errorf("entry %q has unexpected extra data", file.Name)
		}
	}
}

func TestModifyPresetPreservesUnspecifiedParametersAndAssets(t *testing.T) {
	tempDir := t.TempDir()
	preset, err := NewFromDefault()
	if err != nil {
		t.Fatalf("NewFromDefault: %v", err)
	}
	preset.entries = append(preset.entries, archiveEntry{
		Name: "Pigments/User/User/Embedded/custom-data.bin",
		Data: []byte{0, 1, 2, 3, 4, 5, 0xff},
	})
	sourcePath := filepath.Join(tempDir, "source.pgtx")
	if err := preset.Save(sourcePath); err != nil {
		t.Fatalf("save source: %v", err)
	}
	sourceBytesBefore, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	sourceHash := sha256.Sum256(sourceBytesBefore)
	before, err := LoadPGTX(sourcePath)
	if err != nil {
		t.Fatalf("load source: %v", err)
	}
	beforeParams := cloneStringMap(before.block.params)
	beforeAssets := cloneEntryData(before.entries)
	beforeMetadata := before.Metadata
	beforeInnerPath := before.InnerPath
	beforeOriginalName := originalPresetName(t, before.presetData)

	plan := PresetPlan{
		Summary: "Raise only Filter 1 resonance by ten percentage points.",
		Changes: []ParameterChange{{
			ParameterID: "Filter1_Resonance",
			Operation:   "add",
			Value:       "10",
			Unit:        "percent",
			Reason:      "Targeted edit requested by the user.",
		}},
	}
	report, err := ModifyPreset(sourcePath, plan, tempDir)
	if err != nil {
		t.Fatalf("ModifyPreset: %v", err)
	}
	if report.OutputPath == sourcePath {
		t.Fatal("modifier returned the source path")
	}
	if got := report.Metadata; got != beforeMetadata {
		t.Fatalf("metadata changed unexpectedly: got %+v, want %+v", got, beforeMetadata)
	}
	if len(report.Changes) != 1 || report.Changes[0].ParameterID != "Filter1_Resonance" {
		t.Fatalf("unexpected change report: %+v", report.Changes)
	}

	sourceBytesAfter, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(sourceBytesAfter) != sourceHash {
		t.Fatal("source preset was modified")
	}

	after, err := LoadPGTX(report.OutputPath)
	if err != nil {
		t.Fatalf("load modified output: %v", err)
	}
	if after.InnerPath != beforeInnerPath {
		t.Fatalf("inner path changed without a rename: got %q, want %q", after.InnerPath, beforeInnerPath)
	}
	if got := originalPresetName(t, after.presetData); got != beforeOriginalName {
		t.Fatalf("OriginalPresetName changed without a rename: got %q, want %q", got, beforeOriginalName)
	}
	if got, want := after.block.params["Filter1_Resonance"], "0.1"; got != want {
		t.Fatalf("Filter1_Resonance = %q, want %q", got, want)
	}
	for id, beforeValue := range beforeParams {
		if id == "Filter1_Resonance" {
			continue
		}
		if got := after.block.params[id]; got != beforeValue {
			t.Fatalf("unrelated parameter %q changed: got %q, want %q", id, got, beforeValue)
		}
	}
	if got, want := len(after.block.params), len(beforeParams); got != want {
		t.Fatalf("parameter count changed: got %d, want %d", got, want)
	}

	afterAssets := cloneEntryData(after.entries)
	for name, wantData := range beforeAssets {
		if name == beforeInnerPath {
			continue // the serialized preset entry is expected to change.
		}
		gotData, ok := afterAssets[name]
		if !ok {
			t.Fatalf("archive asset %q was removed", name)
		}
		if !bytes.Equal(gotData, wantData) {
			t.Fatalf("archive asset %q changed", name)
		}
	}
}

func TestExplicitRenameUpdatesMetadataOriginalNameAndInnerPath(t *testing.T) {
	tempDir := t.TempDir()
	preset, err := NewFromDefault()
	if err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(tempDir, "source.pgtx")
	if err := preset.Save(sourcePath); err != nil {
		t.Fatal(err)
	}

	report, err := ModifyPreset(sourcePath, PresetPlan{
		PatchName: "Brighter Default",
		Changes: []ParameterChange{{
			ParameterID: "Filter1_Resonance",
			Operation:   "set",
			Value:       "15",
			Unit:        "percent",
		}},
	}, tempDir)
	if err != nil {
		t.Fatalf("ModifyPreset: %v", err)
	}
	after, err := LoadPGTX(report.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := after.Metadata.Name, "Brighter Default"; got != want {
		t.Fatalf("metadata name = %q, want %q", got, want)
	}
	if got, want := originalPresetName(t, after.presetData), "Brighter Default"; got != want {
		t.Fatalf("OriginalPresetName = %q, want %q", got, want)
	}
	if !strings.HasSuffix(after.InnerPath, "/Brighter Default") {
		t.Fatalf("inner path was not renamed: %q", after.InnerPath)
	}
}

func TestGeneratePresetRenamesMetadataMacrosAndFilename(t *testing.T) {
	tempDir := t.TempDir()
	plan := PresetPlan{
		PatchName: "Dark Space Lead",
		Summary:   "Test patch",
		Macros: MacroNames{
			Macro1: "Brightness",
			Macro2: "Fuzz Motion",
		},
		Changes: []ParameterChange{
			{ParameterID: "Engine1_ModuleType", Operation: "set", Value: "wavetable", Unit: "enum", Reason: "Use wavetable synthesis."},
			{ParameterID: "Engine1_WTOsc_FrameIndex", Operation: "set", Value: "25", Unit: "percent", Reason: "Move to a darker frame."},
			{ParameterID: "Filter1_Cutoff", Operation: "set", Value: "1200", Unit: "hz", Reason: "Darken the patch."},
		},
	}
	report, err := GeneratePreset(plan, tempDir)
	if err != nil {
		t.Fatalf("GeneratePreset: %v", err)
	}
	if report.Metadata.Name != plan.PatchName {
		t.Fatalf("name = %q, want %q", report.Metadata.Name, plan.PatchName)
	}
	if report.Metadata.Bank != "User" || report.Metadata.Author != "Audio Prompters" {
		t.Fatalf("unexpected generated metadata: %+v", report.Metadata)
	}
	matched, err := regexp.MatchString(`^Pigments_Preset_Dark_Space_Lead_\d{8}_\d{4}(?:_\d+)?\.pgtx$`, filepath.Base(report.OutputPath))
	if err != nil || !matched {
		t.Fatalf("unexpected filename %q", filepath.Base(report.OutputPath))
	}

	generated, err := LoadPGTX(report.OutputPath)
	if err != nil {
		t.Fatalf("LoadPGTX: %v", err)
	}
	if !strings.HasSuffix(generated.InnerPath, "/Dark Space Lead") {
		t.Fatalf("unexpected inner path %q", generated.InnerPath)
	}
	if got, want := generated.block.params["Engine1_ModuleType"], "0.25"; got != want {
		t.Fatalf("engine type = %q, want %q", got, want)
	}
	if got, want := generated.block.params["Engine1_WTOsc_FrameIndex"], "0.25"; got != want {
		t.Fatalf("wavetable position = %q, want %q", got, want)
	}
	if !containsWarning(report.Warnings, "approximate") {
		t.Fatalf("expected approximate Hz warning, got %v", report.Warnings)
	}
	if got := macroName(generated.presetData, 1); got != "Brightness" {
		t.Fatalf("Macro 1 name = %q, want Brightness", got)
	}
	if got := macroName(generated.presetData, 2); got != "Fuzz Motion" {
		t.Fatalf("Macro 2 name = %q, want Fuzz Motion", got)
	}
}

func TestRelativeDisplayUnitsAndSafetyRules(t *testing.T) {
	cutoffSpec, ok := SpecFor("Filter1_Cutoff")
	if !ok {
		t.Fatal("Filter1_Cutoff spec missing")
	}
	startRaw := formatRaw(displayNumberToRaw(1000, cutoffSpec))
	newRaw, _, newDisplay, approximate, err := calculateNewValue("Filter1_Cutoff", startRaw, ParameterChange{
		Operation: "add",
		Value:     "200",
		Unit:      "hz",
	})
	if err != nil {
		t.Fatalf("calculateNewValue: %v", err)
	}
	if !approximate || !strings.Contains(newDisplay, "1200.0 Hz") {
		t.Fatalf("relative Hz conversion produced raw=%s display=%q approximate=%v", newRaw, newDisplay, approximate)
	}

	preset, err := NewFromDefault()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := preset.ApplyPlan(PresetPlan{Changes: []ParameterChange{{
		ParameterID: "Definitely_Not_A_Real_Parameter",
		Operation:   "set",
		Value:       "0.5",
		Unit:        "normalized",
	}}}, false); err == nil {
		t.Fatal("expected unknown absent parameter to be rejected")
	}

	preset, err = NewFromDefault()
	if err != nil {
		t.Fatal(err)
	}
	beforeCount := len(preset.block.params)
	routeID := "Modulations_Filter1 Cutoff_Macro 1_Amount"
	changes, _, err := preset.ApplyPlan(PresetPlan{Changes: []ParameterChange{{
		ParameterID: routeID,
		Operation:   "set",
		Value:       "0.4",
		Unit:        "raw",
		AllowAdd:    true,
	}}}, false)
	if err != nil {
		t.Fatalf("add verified modulation route: %v", err)
	}
	if len(changes) != 1 || !changes[0].Added {
		t.Fatalf("route was not reported as added: %+v", changes)
	}
	if got, want := len(preset.block.params), beforeCount+1; got != want {
		t.Fatalf("parameter count = %d, want %d", got, want)
	}
}

func TestMetadataAndMacroStringsRemainValidUTF8(t *testing.T) {
	preset, err := NewFromDefault()
	if err != nil {
		t.Fatal(err)
	}
	longUnicode := strings.Repeat("✨", 20) + " Control\x00\n"
	_, warnings, err := preset.ApplyPlan(PresetPlan{
		PatchName: longUnicode,
		Macros:    MacroNames{Macro1: longUnicode},
		Changes: []ParameterChange{{
			ParameterID: "Macro1",
			Operation:   "set",
			Value:       "0",
			Unit:        "percent",
		}},
	}, false)
	if err != nil {
		t.Fatalf("ApplyPlan: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if !utf8.ValidString(preset.Metadata.Name) || len([]byte(preset.Metadata.Name)) > 96 {
		t.Fatalf("invalid/truncated metadata name %q", preset.Metadata.Name)
	}
	macro := macroName(preset.presetData, 1)
	if !utf8.ValidString(macro) || len([]byte(macro)) > 16 {
		t.Fatalf("invalid macro label %q", macro)
	}
	if _, err := preset.Build(); err != nil {
		t.Fatalf("Build after Unicode metadata: %v", err)
	}
}

func cloneStringMap(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func cloneEntryData(entries []archiveEntry) map[string][]byte {
	result := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		result[entry.Name] = append([]byte(nil), entry.Data...)
	}
	return result
}

func macroName(data []byte, index int) string {
	marker := []byte(fmt.Sprintf("11 Macro%d_Name 16 ", index))
	position := bytes.Index(data, marker)
	if position < 0 || position+len(marker)+16 > len(data) {
		return ""
	}
	field := data[position+len(marker) : position+len(marker)+16]
	return strings.TrimRight(string(field), "\x00")
}

func originalPresetName(t *testing.T, data []byte) string {
	t.Helper()
	marker := []byte("18 OriginalPresetName ")
	position := bytes.Index(data, marker)
	if position < 0 {
		t.Fatal("OriginalPresetName marker not found")
	}
	value, _, err := readLengthString(data, position+len(marker))
	if err != nil {
		t.Fatalf("read OriginalPresetName: %v", err)
	}
	return value
}

func containsWarning(warnings []string, text string) bool {
	for _, warning := range warnings {
		if strings.Contains(strings.ToLower(warning), strings.ToLower(text)) {
			return true
		}
	}
	return false
}

func TestSampleEngineBitDepthUsesReversedDisplayRange(t *testing.T) {
	preset, err := NewFromDefault()
	if err != nil {
		t.Fatal(err)
	}
	changes, warnings, err := preset.ApplyPlan(PresetPlan{Changes: []ParameterChange{{
		ParameterID: "Engine1_SampleGranularOsc_BitCrushBitDepth",
		Operation:   "set",
		Value:       "8",
		Unit:        "bits",
	}}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 {
		t.Fatalf("changes=%+v", changes)
	}
	if !strings.Contains(changes[0].NewDisplay, "8.00 bits") {
		t.Fatalf("new display=%q", changes[0].NewDisplay)
	}
	if len(warnings) == 0 {
		t.Fatal("expected approximate-conversion warning")
	}
	raw, err := strconv.ParseFloat(changes[0].NewRaw, 64)
	if err != nil {
		t.Fatal(err)
	}
	if raw < 0.54 || raw > 0.56 {
		t.Fatalf("raw=%f, expected reversed normalized value around 0.552", raw)
	}
}

func TestDiffPresetFilesFindsControlledChange(t *testing.T) {
	baseline, err := NewFromDefault()
	if err != nil {
		t.Fatal(err)
	}
	before := filepath.Join(t.TempDir(), "before.pgtx")
	if err := baseline.Save(before); err != nil {
		t.Fatal(err)
	}
	afterPreset, err := baseline.Clone()
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = afterPreset.ApplyPlan(PresetPlan{Changes: []ParameterChange{{
		ParameterID: "Engine1_SampleGranularOsc_Start",
		Operation:   "set",
		Value:       "25",
		Unit:        "percent",
	}}}, false)
	if err != nil {
		t.Fatal(err)
	}
	after := filepath.Join(t.TempDir(), "after.pgtx")
	if err := afterPreset.Save(after); err != nil {
		t.Fatal(err)
	}
	report, err := DiffPresetFiles(before, after, 100)
	if err != nil {
		t.Fatal(err)
	}
	if report.ChangeCount != 1 || report.Changes[0].ParameterID != "Engine1_SampleGranularOsc_Start" {
		t.Fatalf("report=%+v", report)
	}
}
