package arturia

import "testing"

func TestMasterDatabaseCompilerSpecsAreLoadedConservatively(t *testing.T) {
	bitDepth, ok := SpecFor("Engine1_SampleGranularOsc_BitCrushBitDepth")
	if !ok {
		t.Fatal("generated bit-depth spec is missing")
	}
	if bitDepth.Unit != "bits" || bitDepth.Min != 16 || bitDepth.Max != 1.5 {
		t.Fatalf("bit-depth spec=%+v", bitDepth)
	}

	analogCoarse, ok := SpecFor("Engine1_VA3Osc_Osc1Range")
	if !ok {
		t.Fatal("generated analog coarse spec is missing")
	}
	if analogCoarse.Unit != "percent" || analogCoarse.Min != 0 || analogCoarse.Max != 100 {
		t.Fatalf("uncalibrated analog coarse must use safe knob percentage, got %+v", analogCoarse)
	}

	if _, ok := SpecFor("Engine1_WTOsc_WavetableObject"); ok {
		t.Fatal("serialized wavetable asset object must not be compiler-write-safe")
	}
}
