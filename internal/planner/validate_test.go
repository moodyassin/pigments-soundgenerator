package planner

import (
	"strings"
	"testing"

	"github.com/audioprompters/pigments-web-mvp/internal/arturia"
)

func TestValidatePlanRejectsCalibrationLockedParameter(t *testing.T) {
	plan := &arturia.PresetPlan{
		PatchName: "Locked Test",
		Summary:   "test",
		Changes: []arturia.ParameterChange{{
			ParameterID: "Engine1_SampleGranularOsc_BitCrushMode",
			Operation:   "set",
			Value:       "1",
			Unit:        "normalized",
		}},
	}
	err := ValidatePlan(plan, "generate")
	if err == nil || !strings.Contains(err.Error(), "calibration-locked") {
		t.Fatalf("expected calibration-lock error, got %v", err)
	}
}

func TestValidatePlanAllowsMasterApprovedParameter(t *testing.T) {
	plan := &arturia.PresetPlan{
		PatchName: "Allowed Test",
		Summary:   "test",
		Changes: []arturia.ParameterChange{{
			ParameterID: "Engine1_SampleGranularOsc_BitCrushDecimate",
			Operation:   "set",
			Value:       "32",
			Unit:        "percent",
		}},
	}
	if err := ValidatePlan(plan, "generate"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifiedCatalogExcludesCalibrationLockedTarget(t *testing.T) {
	catalog := verifiedParameterCatalogPrompt()
	if strings.Contains(catalog, "Engine1_SampleGranularOsc_BitCrushMode") {
		t.Fatal("calibration-locked BitCrushMode leaked into verified planner catalog")
	}
	if !strings.Contains(catalog, "Engine1_SampleGranularOsc_BitCrushDecimate") {
		t.Fatal("approved BitCrushDecimate missing from verified planner catalog")
	}
}
