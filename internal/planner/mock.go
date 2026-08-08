package planner

import (
	"context"
	"strings"

	"github.com/audioprompters/pigments-web-mvp/internal/arturia"
)

// Mock is a deterministic, no-network demo planner. It is intentionally simple
// and must never be used as the production AI provider.
type Mock struct{}

func (Mock) Status(context.Context) Status {
	return Status{Provider: "mock", Ready: true, Model: "deterministic-demo", Mock: true, Message: "Demo mode; no OpenAI request will be made"}
}

func (Mock) Plan(_ context.Context, req Request) (*arturia.PresetPlan, error) {
	text := strings.ToLower(req.Instruction)
	name := "Audio Prompters Demo"
	changes := []arturia.ParameterChange{
		{ParameterID: "Engine1_ModuleType", Operation: "set", Value: "wavetable", Unit: "enum", Reason: "Use a flexible wavetable source.", AllowAdd: false},
		{ParameterID: "Engine1_Bypass", Operation: "set", Value: "off", Unit: "boolean", Reason: "Keep Engine 1 active.", AllowAdd: false},
		{ParameterID: "FilterMix_Engine1Volume", Operation: "set", Value: "-5", Unit: "db", Reason: "Audible but conservative engine gain.", AllowAdd: false},
		{ParameterID: "MasterVolume", Operation: "set", Value: "-3", Unit: "db", Reason: "Maintain output headroom.", AllowAdd: false},
		{ParameterID: "Filter1_Cutoff", Operation: "set", Value: "2400", Unit: "hz", Reason: "Establish a balanced tonal contour.", AllowAdd: false},
	}
	if strings.Contains(text, "dark") || strings.Contains(text, "warm") {
		name = "Dark Demo"
		changes[4].Value = "900"
	}
	if strings.Contains(text, "bright") || strings.Contains(text, "spark") {
		name = "Bright Demo"
		changes[4].Value = "6800"
	}
	if strings.Contains(text, "bass") {
		name = "Demo Bass"
		changes = append(changes,
			arturia.ParameterChange{ParameterID: "Env1_Attack", Operation: "set", Value: "2", Unit: "percent", Reason: "Fast bass attack.", AllowAdd: false},
			arturia.ParameterChange{ParameterID: "Env1_Release", Operation: "set", Value: "18", Unit: "percent", Reason: "Keep the bass release controlled.", AllowAdd: false},
		)
	}
	if strings.Contains(text, "sample") || strings.Contains(text, "bit crush") || strings.Contains(text, "bitcrush") {
		name = "Sample Crush Demo"
		changes[0] = arturia.ParameterChange{ParameterID: "Engine1_ModuleType", Operation: "set", Value: "sample", Unit: "enum", Reason: "Use the Sample Engine.", AllowAdd: false}
		changes = append(changes,
			arturia.ParameterChange{ParameterID: "Engine1_SampleGranularOsc_Enable Effect", Operation: "set", Value: "on", Unit: "boolean", Reason: "Enable the Sample Engine shaper.", AllowAdd: false},
			arturia.ParameterChange{ParameterID: "Engine1_SampleGranularOsc_Effect Type", Operation: "set", Value: "bit crush", Unit: "enum", Reason: "Choose Bit Crush shaper mode.", AllowAdd: false},
			arturia.ParameterChange{ParameterID: "Engine1_SampleGranularOsc_BitCrushDecimate", Operation: "set", Value: "32", Unit: "percent", Reason: "Add moderate decimation.", AllowAdd: false},
			arturia.ParameterChange{ParameterID: "Engine1_SampleGranularOsc_BitCrushBitDepth", Operation: "set", Value: "8", Unit: "bits", Reason: "Use approximately eight-bit depth.", AllowAdd: false},
			arturia.ParameterChange{ParameterID: "Engine1_SampleGranularOsc_BitCrushPitchFollow", Operation: "set", Value: "on", Unit: "boolean", Reason: "Keep the effect musically tracked.", AllowAdd: false},
		)
	}
	if strings.EqualFold(req.Mode, "modify") {
		// Keep demo modifications narrow and avoid renaming.
		name = ""
		changes = []arturia.ParameterChange{{ParameterID: "Filter1_Cutoff", Operation: "add", Value: "200", Unit: "hz", Reason: "Demo relative cutoff change.", AllowAdd: false}}
	}
	return &arturia.PresetPlan{
		PatchName: name,
		Summary:   "Deterministic demo plan. Configure OPENAI_API_KEY for AI sound design.",
		Macros:    arturia.MacroNames{Macro1: "Tone"},
		Changes:   changes,
		Warnings:  []string{"Planner is running in mock mode; this is not an AI-generated sound design."},
	}, nil
}
