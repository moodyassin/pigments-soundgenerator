package knowledge

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestMasterDatabaseSummaryMatchesImportedResearch(t *testing.T) {
	summary := GetMasterSummary()
	if summary.DocumentedControlCount != 125 {
		t.Fatalf("documented controls=%d", summary.DocumentedControlCount)
	}
	if summary.InternalParameterCount != 3525 {
		t.Fatalf("internal parameters=%d", summary.InternalParameterCount)
	}
	if summary.PresetParameterValueRows != 15975 {
		t.Fatalf("preset parameter rows=%d", summary.PresetParameterValueRows)
	}
	if summary.PresetCount != 5 {
		t.Fatalf("preset count=%d", summary.PresetCount)
	}
	if summary.MappedUIControlCount != 119 {
		t.Fatalf("mapped UI controls=%d", summary.MappedUIControlCount)
	}
	if summary.AutomaticEditControlCount != 79 {
		t.Fatalf("automatic-edit controls=%d", summary.AutomaticEditControlCount)
	}
	if summary.UniqueMappedInternalIDCount != 260 {
		t.Fatalf("unique mapped internal IDs=%d", summary.UniqueMappedInternalIDCount)
	}
	if summary.ObservedPluginBuild != "7.0.1.6772" {
		t.Fatalf("observed build=%q", summary.ObservedPluginBuild)
	}
	if summary.ControlsWithNumericRange == 0 || summary.ControlsAwaitingCalibration == 0 {
		t.Fatalf("range/calibration summary is empty: %+v", summary)
	}
}

func TestMasterSearchSeparatesUIEvidenceFromSerializationEvidence(t *testing.T) {
	results := MasterSearch("bitcrush", 100)
	if len(results) == 0 {
		t.Fatal("no bitcrush results")
	}
	var foundUI, foundInternal bool
	for _, result := range results {
		if result.Kind == "documented_ui_control" && strings.Contains(result.ID, "sample.shaper.bitcrush") {
			foundUI = true
		}
		if result.Kind == "observed_internal_parameter" && strings.Contains(result.ID, "BitCrush") {
			foundInternal = true
		}
	}
	if !foundUI || !foundInternal {
		t.Fatalf("expected both knowledge layers, ui=%v internal=%v", foundUI, foundInternal)
	}
}

func TestExactInternalIDSearchRanksExactMatchFirst(t *testing.T) {
	id := "Engine1_SampleGranularOsc_BitCrushDecimate"
	results := MasterSearch(id, 10)
	if len(results) == 0 || results[0].ID != id || results[0].Kind != "observed_internal_parameter" {
		t.Fatalf("unexpected exact search result: %+v", results)
	}
}

func TestPublicKnowledgeDoesNotBulkExposeInternalInventory(t *testing.T) {
	payload := JSON()
	if len(payload) > 2<<20 {
		t.Fatalf("public knowledge payload is unexpectedly large: %d bytes", len(payload))
	}
	if bytes.Contains(payload, []byte("AfterTouchCurve_LastActivePointIndex")) {
		t.Fatal("public knowledge endpoint bulk-exposed internal inventory")
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	master, ok := decoded["master_database"].(map[string]any)
	if !ok || master["summary"] == nil || master["documented_controls"] == nil {
		t.Fatalf("master public payload is incomplete: %#v", decoded["master_database"])
	}
}

func TestGeneratedMappingOverlayCountsAndSafety(t *testing.T) {
	summary := GetMasterSummary()
	if summary.MappedUIControlCount != 119 {
		t.Fatalf("mapped controls=%d", summary.MappedUIControlCount)
	}
	if summary.AutomaticEditControlCount != 79 {
		t.Fatalf("automatic edit controls=%d", summary.AutomaticEditControlCount)
	}
	if summary.UniqueMappedInternalIDCount != 260 {
		t.Fatalf("mapped internal IDs=%d", summary.UniqueMappedInternalIDCount)
	}

	results := MasterSearch("sample bit depth", 50)
	var bitDepth *MasterSearchResult
	for i := range results {
		if results[i].ID == "sample.shaper.bitcrush.bit_depth" {
			bitDepth = &results[i]
			break
		}
	}
	if bitDepth == nil {
		t.Fatal("bit-depth UI mapping not found")
	}
	if !bitDepth.AutomaticEdit || bitDepth.MappingConfidence != "high" {
		t.Fatalf("unexpected bit-depth mapping safety: %+v", bitDepth)
	}
	if len(bitDepth.CanonicalIDs) != 2 {
		t.Fatalf("bit-depth canonical IDs=%v", bitDepth.CanonicalIDs)
	}

	browser, ok := MappingForControl("sample.browser.sample_select")
	if !ok {
		t.Fatal("sample browser mapping not found")
	}
	if browser.AutomaticEdit {
		t.Fatalf("asset selector must not be automatic-edit: %+v", browser)
	}
	if browser.MappingStatus != "serialized_audio_object_mapping_required" {
		t.Fatalf("sample browser mapping status=%q", browser.MappingStatus)
	}
}

func TestMasterParameterEditPolicy(t *testing.T) {
	allowed, ok := PolicyForParameter("Engine1_SampleGranularOsc_BitCrushDecimate")
	if !ok || !allowed.AutomaticEdit {
		t.Fatalf("expected BitCrushDecimate to be governed and automatic-edit safe: %#v, ok=%v", allowed, ok)
	}
	locked, ok := PolicyForParameter("Engine1_SampleGranularOsc_BitCrushMode")
	if !ok || locked.AutomaticEdit {
		t.Fatalf("expected BitCrushMode to be calibration-locked: %#v, ok=%v", locked, ok)
	}
	if !AutomaticEditAllowed("Engine1_Bypass") {
		t.Fatal("direct Engine1_Bypass semantics should remain governed by the curated compiler; transformed UI Power mapping must not lock it")
	}
	if !AutomaticEditAllowed("Filter1_Cutoff") {
		t.Fatal("parameters outside the master overlay should retain curated compiler policy")
	}
}
