package arturia

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

//go:embed master_parameter_specs.json
var rawMasterParameterSpecs []byte

type masterParameterSpecFile struct {
	Specs []ParameterSpec `json:"specs"`
}

var engineTypeValues = map[string]string{
	"analog": "0", "wavetable": "0.25", "sample": "0.5", "granular": "0.5", "harmonic": "0.75", "modal": "1",
}

var filterTypeValues = map[string]string{
	"none": "0", "ladder": "0.14285715", "multimode": "0.2", "classic": "0.2", "sem": "0.2857143", "formant": "0.42857143", "comb": "0.5714286", "phaser": "0.71428573",
}

var fxTypeValues = map[string]string{
	"none": "0", "chorus": "0.07692308", "distortion": "0.15384616", "compressor": "0.23076923", "flanger": "0.30769232", "phaser": "0.38461539", "delay": "0.46153846", "tape echo": "0.53846157", "tapeecho": "0.53846157", "reverb": "0.61538464", "shimmer": "0.69230771", "filter": "0.76923078", "super unison": "0.84615386", "superunison": "0.84615386", "erosion": "0.92307693", "bitcrusher": "1", "bit crusher": "1",
}

var sampleShaperTypeValues = map[string]string{
	"unison": "0", "chord": "0.2", "super": "0.4", "resonator": "0.6", "bit crush": "0.8", "bitcrush": "0.8", "modulation": "1",
}

var lfoWaveValues = map[string]string{
	"sine": "0", "triangle": "0.33333331", "saw": "0.66666669", "sawtooth": "0.66666669", "square": "1",
}

// KnownParameterSpecs is intentionally curated. The editor can still preserve and edit
// any existing raw parameter ID; these specs add human aliases and unit conversions.
var KnownParameterSpecs = []ParameterSpec{
	{ID: "Engine1_ModuleType", Friendly: "Engine 1 type", Aliases: []string{"engine1 type", "engine 1 type", "oscillator 1 engine"}, Unit: "enum", EnumValues: engineTypeValues, Description: "Selects Analog, Wavetable, Sample/Granular, Harmonic, or Modal engine."},
	{ID: "Engine2_ModuleType", Friendly: "Engine 2 type", Aliases: []string{"engine2 type", "engine 2 type", "oscillator 2 engine"}, Unit: "enum", EnumValues: engineTypeValues, Description: "Selects the second synthesis engine."},
	{ID: "Engine1_Bypass", Friendly: "Engine 1 bypass", Aliases: []string{"engine1 bypass", "disable engine 1"}, Unit: "boolean", Min: 0, Max: 1, Description: "Bypasses Engine 1 when enabled."},
	{ID: "Engine2_Bypass", Friendly: "Engine 2 bypass", Aliases: []string{"engine2 bypass", "disable engine 2"}, Unit: "boolean", Min: 0, Max: 1, Description: "Bypasses Engine 2 when enabled."},
	{ID: "FilterMix_Engine1Volume", Friendly: "Engine 1 output volume", Aliases: []string{"engine1 volume", "engine 1 volume", "engine1 gain", "engine 1 gain"}, Unit: "db", Min: -70, Max: 12, Curve: "linear", Description: "Main Engine 1 level before filtering."},
	{ID: "FilterMix_Engine2Volume", Friendly: "Engine 2 output volume", Aliases: []string{"engine2 volume", "engine 2 volume", "engine2 gain", "engine 2 gain"}, Unit: "db", Min: -70, Max: 12, Curve: "linear", Description: "Main Engine 2 level before filtering."},
	{ID: "FilterMix_Engine1FilterMix", Friendly: "Engine 1 filter mix", Aliases: []string{"engine1 filter mix", "engine 1 routing", "engine1 filter routing"}, Unit: "percent", Min: 0, Max: 100, Description: "Routes Engine 1 between Filter 1 and Filter 2."},
	{ID: "FilterMix_Engine2FilterMix", Friendly: "Engine 2 filter mix", Aliases: []string{"engine2 filter mix", "engine 2 routing", "engine2 filter routing"}, Unit: "percent", Min: 0, Max: 100, Description: "Routes Engine 2 between Filter 1 and Filter 2."},
	{ID: "MasterVolume", Friendly: "Master volume", Aliases: []string{"master volume", "main volume", "output level"}, Unit: "db", Min: -70, Max: 12, Curve: "linear", Description: "Pigments master output level."},

	{ID: "Engine1_WTOsc_FrameIndex", Friendly: "Engine 1 wavetable position", Aliases: []string{"engine1 wavetable position", "engine 1 wavetable position", "wt1 frame", "wavetable 1 position", "wave position"}, Unit: "percent", Min: 0, Max: 100, Description: "Position within Engine 1 wavetable."},
	{ID: "Engine2_WTOsc_FrameIndex", Friendly: "Engine 2 wavetable position", Aliases: []string{"engine2 wavetable position", "engine 2 wavetable position", "wt2 frame", "wavetable 2 position"}, Unit: "percent", Min: 0, Max: 100, Description: "Position within Engine 2 wavetable."},
	{ID: "Engine1_WTOsc_MainVolume", Friendly: "Engine 1 wavetable volume", Aliases: []string{"wt1 volume", "wavetable 1 volume"}, Unit: "percent", Min: 0, Max: 100, Description: "Wavetable oscillator level inside Engine 1."},
	{ID: "Engine2_WTOsc_MainVolume", Friendly: "Engine 2 wavetable volume", Aliases: []string{"wt2 volume", "wavetable 2 volume"}, Unit: "percent", Min: 0, Max: 100, Description: "Wavetable oscillator level inside Engine 2."},
	{ID: "Engine1_WTOsc_FMAmount", Friendly: "Engine 1 frequency/ring modulation amount", Aliases: []string{"engine1 fm", "engine 1 fm", "ring mod 1", "frequency mod 1", "wt1 fm"}, Unit: "percent", Min: 0, Max: 100, Description: "Frequency or ring modulation amount for Wavetable Engine 1."},
	{ID: "Engine2_WTOsc_FMAmount", Friendly: "Engine 2 frequency/ring modulation amount", Aliases: []string{"engine2 fm", "engine 2 fm", "ring mod 2", "frequency mod 2", "wt2 fm"}, Unit: "percent", Min: 0, Max: 100, Description: "Frequency or ring modulation amount for Wavetable Engine 2."},
	{ID: "Engine1_WTOsc_FMType", Friendly: "Engine 1 FM/ring-mod type", Aliases: []string{"engine1 fm type", "engine 1 ring mod type"}, Unit: "normalized", Min: 0, Max: 1, Description: "Selects frequency modulation versus ring modulation behavior."},
	{ID: "Engine2_WTOsc_FMType", Friendly: "Engine 2 FM/ring-mod type", Aliases: []string{"engine2 fm type", "engine 2 ring mod type"}, Unit: "normalized", Min: 0, Max: 1, Description: "Selects frequency modulation versus ring modulation behavior."},
	{ID: "Engine1_WTOsc_PMAmount", Friendly: "Engine 1 phase modulation", Aliases: []string{"engine1 phase mod", "engine 1 phase modulation", "wt1 phase mod"}, Unit: "percent", Min: 0, Max: 100, Description: "Phase modulation amount for Wavetable Engine 1."},
	{ID: "Engine2_WTOsc_PMAmount", Friendly: "Engine 2 phase modulation", Aliases: []string{"engine2 phase mod", "engine 2 phase modulation", "wt2 phase mod"}, Unit: "percent", Min: 0, Max: 100, Description: "Phase modulation amount for Wavetable Engine 2."},
	{ID: "Engine1_WTOsc_Phase", Friendly: "Engine 1 wavetable phase", Aliases: []string{"engine1 phase", "engine 1 phase", "wt1 phase"}, Unit: "percent", Min: 0, Max: 100, Description: "Starting oscillator phase."},
	{ID: "Engine2_WTOsc_Phase", Friendly: "Engine 2 wavetable phase", Aliases: []string{"engine2 phase", "engine 2 phase", "wt2 phase"}, Unit: "percent", Min: 0, Max: 100, Description: "Starting oscillator phase."},
	{ID: "Engine1_WTOsc_PhaseDist", Friendly: "Engine 1 phase transform", Aliases: []string{"engine1 phase transform", "engine 1 pulse width", "wt1 phase distortion"}, Unit: "percent", Min: 0, Max: 100, Description: "Phase-transform or phase-distortion amount."},
	{ID: "Engine2_WTOsc_PhaseDist", Friendly: "Engine 2 phase transform", Aliases: []string{"engine2 phase transform", "engine 2 pulse width", "wt2 phase distortion"}, Unit: "percent", Min: 0, Max: 100, Description: "Phase-transform or phase-distortion amount."},
	{ID: "Engine1_WTOsc_Fold", Friendly: "Engine 1 wavefold amount", Aliases: []string{"engine1 fold", "engine 1 wavefold", "wt1 fold", "wave folding 1"}, Unit: "percent", Min: 0, Max: 100, Description: "Wavefolding amount for Engine 1."},
	{ID: "Engine2_WTOsc_Fold", Friendly: "Engine 2 wavefold amount", Aliases: []string{"engine2 fold", "engine 2 wavefold", "wt2 fold", "wave folding 2"}, Unit: "percent", Min: 0, Max: 100, Description: "Wavefolding amount for Engine 2."},
	{ID: "Engine1_WTOsc_UnisonDetune", Friendly: "Engine 1 unison detune", Aliases: []string{"engine1 detune", "engine 1 unison detune", "wt1 detune"}, Unit: "percent", Min: 0, Max: 100, Description: "Unison detune amount."},
	{ID: "Engine2_WTOsc_UnisonDetune", Friendly: "Engine 2 unison detune", Aliases: []string{"engine2 detune", "engine 2 unison detune", "wt2 detune"}, Unit: "percent", Min: 0, Max: 100, Description: "Unison detune amount."},
	{ID: "Engine1_WTOsc_UnisonStereo", Friendly: "Engine 1 unison stereo", Aliases: []string{"engine1 stereo", "engine 1 unison stereo", "wt1 stereo"}, Unit: "percent", Min: 0, Max: 100, Description: "Unison stereo spread."},
	{ID: "Engine2_WTOsc_UnisonStereo", Friendly: "Engine 2 unison stereo", Aliases: []string{"engine2 stereo", "engine 2 unison stereo", "wt2 stereo"}, Unit: "percent", Min: 0, Max: 100, Description: "Unison stereo spread."},

	{ID: "Engine1_VA3Osc_Osc1Wave", Friendly: "Analog Engine 1 oscillator 1 waveform", Aliases: []string{"analog oscillator 1 waveform", "engine1 osc1 waveform", "engine 1 oscillator waveform"}, Unit: "normalized", Min: 0, Max: 1, Description: "Analog Engine 1 oscillator-1 waveform selector/morph."},
	{ID: "Engine1_VA3Osc_Osc1Volume", Friendly: "Analog Engine 1 oscillator 1 volume", Aliases: []string{"engine1 osc1 volume", "analog oscillator 1 volume"}, Unit: "percent", Min: 0, Max: 100, Description: "Analog oscillator 1 level."},
	{ID: "Engine1_VA3Osc_Osc2Wave", Friendly: "Analog Engine 1 oscillator 2 waveform", Aliases: []string{"analog oscillator 2 waveform", "engine1 osc2 waveform"}, Unit: "normalized", Min: 0, Max: 1, Description: "Analog Engine 1 oscillator-2 waveform selector/morph."},
	{ID: "Engine1_VA3Osc_Osc2Volume", Friendly: "Analog Engine 1 oscillator 2 volume", Aliases: []string{"engine1 osc2 volume", "analog oscillator 2 volume"}, Unit: "percent", Min: 0, Max: 100, Description: "Analog oscillator 2 level."},
	{ID: "Engine1_VA3Osc_Osc3Wave", Friendly: "Analog Engine 1 oscillator 3 waveform", Aliases: []string{"analog oscillator 3 waveform", "engine1 osc3 waveform"}, Unit: "normalized", Min: 0, Max: 1, Description: "Analog Engine 1 oscillator-3 waveform selector/morph."},
	{ID: "Engine1_VA3Osc_Osc3Volume", Friendly: "Analog Engine 1 oscillator 3 volume", Aliases: []string{"engine1 osc3 volume", "analog oscillator 3 volume"}, Unit: "percent", Min: 0, Max: 100, Description: "Analog oscillator 3 level."},

	{ID: "Filter1_ModuleType", Friendly: "Filter 1 type", Aliases: []string{"filter1 type", "filter 1 type"}, Unit: "enum", EnumValues: filterTypeValues, Description: "Selects Filter 1 model."},
	{ID: "Filter2_ModuleType", Friendly: "Filter 2 type", Aliases: []string{"filter2 type", "filter 2 type"}, Unit: "enum", EnumValues: filterTypeValues, Description: "Selects Filter 2 model."},
	{ID: "Filter1_Cutoff", Friendly: "Filter 1 cutoff", Aliases: []string{"filter1 cutoff", "filter 1 cutoff", "filter1 frequency", "filter 1 frequency", "f1 cutoff"}, Unit: "hz", Min: 20, Max: 20000, Curve: "log", Description: "Filter 1 cutoff frequency. Hz conversion is an approximation of the normalized Pigments curve."},
	{ID: "Filter2_Cutoff", Friendly: "Filter 2 cutoff", Aliases: []string{"filter2 cutoff", "filter 2 cutoff", "filter2 frequency", "filter 2 frequency", "f2 cutoff"}, Unit: "hz", Min: 20, Max: 20000, Curve: "log", Description: "Filter 2 cutoff frequency. Hz conversion is an approximation of the normalized Pigments curve."},
	{ID: "Filter1_Resonance", Friendly: "Filter 1 resonance", Aliases: []string{"filter1 resonance", "filter 1 resonance", "f1 resonance"}, Unit: "percent", Min: 0, Max: 100, Description: "Filter 1 resonance."},
	{ID: "Filter2_Resonance", Friendly: "Filter 2 resonance", Aliases: []string{"filter2 resonance", "filter 2 resonance", "f2 resonance"}, Unit: "percent", Min: 0, Max: 100, Description: "Filter 2 resonance."},
	{ID: "Filter1_Bypass", Friendly: "Filter 1 bypass", Aliases: []string{"filter1 bypass", "disable filter 1"}, Unit: "boolean", Min: 0, Max: 1, Description: "Bypasses Filter 1."},
	{ID: "Filter2_Bypass", Friendly: "Filter 2 bypass", Aliases: []string{"filter2 bypass", "disable filter 2"}, Unit: "boolean", Min: 0, Max: 1, Description: "Bypasses Filter 2."},
	{ID: "Filter1_Volume", Friendly: "Filter 1 output volume", Aliases: []string{"filter1 volume", "filter 1 volume"}, Unit: "db", Min: -70, Max: 12, Curve: "linear", Description: "Filter 1 output level."},
	{ID: "Filter2_Volume", Friendly: "Filter 2 output volume", Aliases: []string{"filter2 volume", "filter 2 volume"}, Unit: "db", Min: -70, Max: 12, Curve: "linear", Description: "Filter 2 output level."},

	{ID: "Env1_Attack", Friendly: "Envelope 1 attack", Aliases: []string{"env1 attack", "envelope 1 attack", "amp attack"}, Unit: "percent", Min: 0, Max: 100, Description: "Envelope 1 attack normalized across Pigments' time curve."},
	{ID: "Env1_Decay", Friendly: "Envelope 1 decay", Aliases: []string{"env1 decay", "envelope 1 decay", "amp decay"}, Unit: "percent", Min: 0, Max: 100, Description: "Envelope 1 decay normalized across Pigments' time curve."},
	{ID: "Env1_Sustain", Friendly: "Envelope 1 sustain", Aliases: []string{"env1 sustain", "envelope 1 sustain", "amp sustain"}, Unit: "percent", Min: 0, Max: 100, Description: "Envelope 1 sustain level."},
	{ID: "Env1_Release", Friendly: "Envelope 1 release", Aliases: []string{"env1 release", "envelope 1 release", "amp release"}, Unit: "percent", Min: 0, Max: 100, Description: "Envelope 1 release normalized across Pigments' time curve."},
	{ID: "Env2_Attack", Friendly: "Envelope 2 attack", Aliases: []string{"env2 attack", "envelope 2 attack"}, Unit: "percent", Min: 0, Max: 100, Description: "Envelope 2 attack."},
	{ID: "Env2_Decay", Friendly: "Envelope 2 decay", Aliases: []string{"env2 decay", "envelope 2 decay"}, Unit: "percent", Min: 0, Max: 100, Description: "Envelope 2 decay."},
	{ID: "Env2_Sustain", Friendly: "Envelope 2 sustain", Aliases: []string{"env2 sustain", "envelope 2 sustain"}, Unit: "percent", Min: 0, Max: 100, Description: "Envelope 2 sustain."},
	{ID: "Env2_Release", Friendly: "Envelope 2 release", Aliases: []string{"env2 release", "envelope 2 release"}, Unit: "percent", Min: 0, Max: 100, Description: "Envelope 2 release."},
	{ID: "Env3_Attack", Friendly: "Envelope 3 attack", Aliases: []string{"env3 attack", "envelope 3 attack"}, Unit: "percent", Min: 0, Max: 100, Description: "Envelope 3 attack."},
	{ID: "Env3_Decay", Friendly: "Envelope 3 decay", Aliases: []string{"env3 decay", "envelope 3 decay"}, Unit: "percent", Min: 0, Max: 100, Description: "Envelope 3 decay."},
	{ID: "Env3_Sustain", Friendly: "Envelope 3 sustain", Aliases: []string{"env3 sustain", "envelope 3 sustain"}, Unit: "percent", Min: 0, Max: 100, Description: "Envelope 3 sustain."},
	{ID: "Env3_Release", Friendly: "Envelope 3 release", Aliases: []string{"env3 release", "envelope 3 release"}, Unit: "percent", Min: 0, Max: 100, Description: "Envelope 3 release."},

	{ID: "LFO1_Setting", Friendly: "LFO 1 enabled", Aliases: []string{"enable lfo1", "lfo 1 on"}, Unit: "boolean", Min: 0, Max: 1, Description: "Activates LFO 1."},
	{ID: "LFO2_Setting", Friendly: "LFO 2 enabled", Aliases: []string{"enable lfo2", "lfo 2 on"}, Unit: "boolean", Min: 0, Max: 1, Description: "Activates LFO 2."},
	{ID: "LFO3_Setting", Friendly: "LFO 3 enabled", Aliases: []string{"enable lfo3", "lfo 3 on"}, Unit: "boolean", Min: 0, Max: 1, Description: "Activates LFO 3."},
	{ID: "LFO1_RateUnSynced", Friendly: "LFO 1 rate", Aliases: []string{"lfo1 rate", "lfo 1 rate"}, Unit: "percent", Min: 0, Max: 100, Description: "Free-running LFO 1 rate across Pigments' nonlinear range."},
	{ID: "LFO2_RateUnSynced", Friendly: "LFO 2 rate", Aliases: []string{"lfo2 rate", "lfo 2 rate"}, Unit: "percent", Min: 0, Max: 100, Description: "Free-running LFO 2 rate across Pigments' nonlinear range."},
	{ID: "LFO3_RateUnSynced", Friendly: "LFO 3 rate", Aliases: []string{"lfo3 rate", "lfo 3 rate"}, Unit: "percent", Min: 0, Max: 100, Description: "Free-running LFO 3 rate across Pigments' nonlinear range."},
	{ID: "LFO1_Waveform", Friendly: "LFO 1 waveform", Aliases: []string{"lfo1 waveform", "lfo 1 shape"}, Unit: "enum", EnumValues: lfoWaveValues, Description: "LFO 1 waveform."},
	{ID: "LFO2_Waveform", Friendly: "LFO 2 waveform", Aliases: []string{"lfo2 waveform", "lfo 2 shape"}, Unit: "enum", EnumValues: lfoWaveValues, Description: "LFO 2 waveform."},
	{ID: "LFO3_Waveform", Friendly: "LFO 3 waveform", Aliases: []string{"lfo3 waveform", "lfo 3 shape"}, Unit: "enum", EnumValues: lfoWaveValues, Description: "LFO 3 waveform."},

	{ID: "Random1_ModuleType", Friendly: "Random 1 mode", Aliases: []string{"random1 mode", "random 1 type"}, Unit: "normalized", Min: 0, Max: 1, Description: "Random modulation mode selector."},
	{ID: "Random1_RnH_Distance", Friendly: "Random 1 rate/distance", Aliases: []string{"random1 rate", "random 1 rate", "random 1 distance"}, Unit: "percent", Min: 0, Max: 100, Description: "Random modulation distance/rate control."},
	{ID: "Random1_RnH_Jitter", Friendly: "Random 1 jitter", Aliases: []string{"random1 jitter", "random 1 jitter"}, Unit: "percent", Min: 0, Max: 100, Description: "Random modulation jitter."},
	{ID: "Random1_RnH_Smooth", Friendly: "Random 1 smoothing", Aliases: []string{"random1 smooth", "random 1 smoothing"}, Unit: "percent", Min: 0, Max: 100, Description: "Random modulation smoothing."},

	{ID: "FilterMix_UtilitySOVolume", Friendly: "Utility oscillator volume", Aliases: []string{"utility volume", "sub oscillator volume", "sub volume"}, Unit: "db", Min: -70, Max: 12, Curve: "linear", Description: "Utility/sub oscillator output level."},
	{ID: "FilterMix_UtilityN1Volume", Friendly: "Utility noise 1 volume", Aliases: []string{"noise1 volume", "utility noise 1 volume"}, Unit: "db", Min: -70, Max: 12, Curve: "linear", Description: "Utility Noise 1 output level."},
	{ID: "FilterMix_UtilityN2Volume", Friendly: "Utility noise 2 volume", Aliases: []string{"noise2 volume", "utility noise 2 volume"}, Unit: "db", Min: -70, Max: 12, Curve: "linear", Description: "Utility Noise 2 output level."},
	{ID: "Utility_Osc_OnOff", Friendly: "Utility oscillator enabled", Aliases: []string{"enable utility oscillator", "sub oscillator on"}, Unit: "boolean", Min: 0, Max: 1, Description: "Enables utility oscillator."},
	{ID: "Utility_Noise1_OnOff", Friendly: "Utility noise 1 enabled", Aliases: []string{"enable noise1", "noise 1 on"}, Unit: "boolean", Min: 0, Max: 1, Description: "Enables Utility Noise 1."},
	{ID: "Utility_Noise2_OnOff", Friendly: "Utility noise 2 enabled", Aliases: []string{"enable noise2", "noise 2 on"}, Unit: "boolean", Min: 0, Max: 1, Description: "Enables Utility Noise 2."},

	{ID: "Macro1", Friendly: "Macro 1 value", Aliases: []string{"macro1", "macro 1"}, Unit: "percent", Min: 0, Max: 100, Description: "Saved Macro 1 position."},
	{ID: "Macro2", Friendly: "Macro 2 value", Aliases: []string{"macro2", "macro 2"}, Unit: "percent", Min: 0, Max: 100, Description: "Saved Macro 2 position."},
	{ID: "Macro3", Friendly: "Macro 3 value", Aliases: []string{"macro3", "macro 3"}, Unit: "percent", Min: 0, Max: 100, Description: "Saved Macro 3 position."},
	{ID: "Macro4", Friendly: "Macro 4 value", Aliases: []string{"macro4", "macro 4"}, Unit: "percent", Min: 0, Max: 100, Description: "Saved Macro 4 position."},
	{ID: "VST3_CtrlModWheel", Friendly: "Mod wheel saved position", Aliases: []string{"mod wheel", "modwheel"}, Unit: "percent", Min: 0, Max: 100, Description: "Saved modulation-wheel position, not a routing by itself."},
}

func init() {
	for slot := 1; slot <= 6; slot++ {
		KnownParameterSpecs = append(KnownParameterSpecs,
			ParameterSpec{ID: fmt.Sprintf("FX%d_ModuleType", slot), Friendly: fmt.Sprintf("FX %d type", slot), Aliases: []string{fmt.Sprintf("fx%d type", slot), fmt.Sprintf("effect %d type", slot)}, Unit: "enum", EnumValues: fxTypeValues, Description: fmt.Sprintf("Selects the effect loaded in FX slot %d.", slot)},
			ParameterSpec{ID: fmt.Sprintf("FX%d_Dry_Wet", slot), Friendly: fmt.Sprintf("FX %d dry/wet", slot), Aliases: []string{fmt.Sprintf("fx%d amount", slot), fmt.Sprintf("fx %d mix", slot), fmt.Sprintf("effect %d wet", slot)}, Unit: "percent", Min: 0, Max: 100, Description: fmt.Sprintf("Wet mix for FX slot %d.", slot)},
			ParameterSpec{ID: fmt.Sprintf("FX%d_Bypass", slot), Friendly: fmt.Sprintf("FX %d bypass", slot), Aliases: []string{fmt.Sprintf("fx%d bypass", slot), fmt.Sprintf("disable effect %d", slot)}, Unit: "boolean", Min: 0, Max: 1, Description: fmt.Sprintf("Bypasses FX slot %d.", slot)},
		)
	}
	for engine := 1; engine <= 2; engine++ {
		prefix := fmt.Sprintf("Engine%d_SampleGranularOsc_", engine)
		label := fmt.Sprintf("Sample Engine %d", engine)
		KnownParameterSpecs = append(KnownParameterSpecs,
			ParameterSpec{ID: prefix + "KeyTrack", Friendly: label + " keyboard tracking", Aliases: []string{fmt.Sprintf("engine %d sample keytrack", engine), fmt.Sprintf("sample %d keyboard tracking", engine)}, Unit: "boolean", Min: 0, Max: 1, Description: "Controls whether sample pitch follows incoming notes."},
			ParameterSpec{ID: prefix + "Quantize", Friendly: label + " coarse-mod quantize", Aliases: []string{fmt.Sprintf("sample %d quantize", engine), fmt.Sprintf("engine %d sample pitch class", engine)}, Unit: "normalized", Min: 0, Max: 1, Description: "Pitch-class selector C through B. The exact enum direction still requires controlled preset-diff calibration."},
			ParameterSpec{ID: prefix + "Coarse", Friendly: label + " coarse tune", Aliases: []string{fmt.Sprintf("sample %d coarse", engine), fmt.Sprintf("engine %d sample transpose", engine)}, Unit: "semitones", Min: -36, Max: 36, Curve: "linear", Description: "Sample-engine coarse transposition."},
			ParameterSpec{ID: prefix + "Fine", Friendly: label + " fine tune", Aliases: []string{fmt.Sprintf("sample %d fine", engine), fmt.Sprintf("engine %d sample fine tune", engine)}, Unit: "semitones", Min: -1, Max: 1, Curve: "linear", Description: "Sample-engine fine pitch adjustment."},
			ParameterSpec{ID: prefix + "Filter", Friendly: label + " sample filter", Aliases: []string{fmt.Sprintf("sample %d filter", engine), fmt.Sprintf("engine %d lp hp sample filter", engine)}, Unit: "normalized", Min: 0, Max: 1, Description: "Bipolar sample filter: approximately 0=LP, 0.5=no filter, 1=HP."},
			ParameterSpec{ID: prefix + "Enable Effect", Friendly: label + " shaper enabled", Aliases: []string{fmt.Sprintf("sample %d shaper on", engine), fmt.Sprintf("engine %d sample effect on", engine)}, Unit: "boolean", Min: 0, Max: 1, Description: "Enables the Sample Engine shaper section."},
			ParameterSpec{ID: prefix + "Effect Type", Friendly: label + " shaper mode", Aliases: []string{fmt.Sprintf("sample %d shaper mode", engine), fmt.Sprintf("engine %d sample effect type", engine)}, Unit: "enum", EnumValues: sampleShaperTypeValues, Description: "Selects Unison, Chord, Super, Resonator, Bit Crush, or Modulation."},
			ParameterSpec{ID: prefix + "BitCrushDecimate", Friendly: label + " bit-crush decimate", Aliases: []string{fmt.Sprintf("sample %d decimate", engine), fmt.Sprintf("engine %d sample decimation", engine)}, Unit: "percent", Min: 0, Max: 100, Description: "Sample-engine Bit Crush decimation amount."},
			ParameterSpec{ID: prefix + "BitCrushMode", Friendly: label + " bit-crush mode", Aliases: []string{fmt.Sprintf("sample %d decimate mode", engine), fmt.Sprintf("engine %d classic smooth", engine)}, Unit: "normalized", Min: 0, Max: 1, Description: "Classic/Smooth selector; enum direction needs controlled audition and preset-diff calibration."},
			ParameterSpec{ID: prefix + "BitCrushBitDepth", Friendly: label + " bit depth", Aliases: []string{fmt.Sprintf("sample %d bit depth", engine), fmt.Sprintf("engine %d sample bits", engine)}, Unit: "bits", Min: 16, Max: 1.5, Curve: "linear", Description: "Bit depth. The serialized direction is reversed: raw 0 is approximately 16 bits and raw 1 is approximately 1.5 bits."},
			ParameterSpec{ID: prefix + "BitCrushPitchFollow", Friendly: label + " bit-crush pitch follow", Aliases: []string{fmt.Sprintf("sample %d pitch follow", engine), fmt.Sprintf("engine %d bitcrush pitch follow", engine)}, Unit: "boolean", Min: 0, Max: 1, Description: "Makes the Bit Crush processing follow pitch."},
			ParameterSpec{ID: prefix + "Start", Friendly: label + " sample start", Aliases: []string{fmt.Sprintf("sample %d start", engine), fmt.Sprintf("engine %d sample start time", engine)}, Unit: "percent", Min: 0, Max: 100, Description: "Start position as a percentage of the loaded sample duration. It is not globally limited to seven seconds."},
			ParameterSpec{ID: prefix + "UnisonDetune", Friendly: label + " shaper unison/super detune", Aliases: []string{fmt.Sprintf("sample %d shaper detune", engine), fmt.Sprintf("sample %d unison detune", engine), fmt.Sprintf("sample %d super detune", engine)}, Unit: "percent", Min: 0, Max: 100, Description: "Detune amount shared by the Sample Shaper Unison and Super modes."},
			ParameterSpec{ID: prefix + "UnisonStereo", Friendly: label + " shaper stereo", Aliases: []string{fmt.Sprintf("sample %d shaper stereo", engine), fmt.Sprintf("sample %d unison stereo", engine), fmt.Sprintf("sample %d super stereo", engine)}, Unit: "percent", Min: 0, Max: 100, Description: "Stereo spread used by Sample Shaper voice-stacking modes."},
			ParameterSpec{ID: prefix + "Unison_PhaseControl", Friendly: label + " shaper unison phase", Aliases: []string{fmt.Sprintf("sample %d unison phase", engine), fmt.Sprintf("sample %d shaper phase", engine)}, Unit: "percent", Min: 0, Max: 100, Description: "Phase control for the Sample Shaper Unison mode."},
			ParameterSpec{ID: prefix + "UnisonMix", Friendly: label + " shaper super mix", Aliases: []string{fmt.Sprintf("sample %d super mix", engine), fmt.Sprintf("sample %d shaper mix", engine)}, Unit: "percent", Min: 0, Max: 100, Description: "Mix amount for the Sample Shaper Super mode."},
			ParameterSpec{ID: prefix + "ResonatorFcCoarse", Friendly: label + " shaper resonator coarse", Aliases: []string{fmt.Sprintf("sample %d resonator coarse", engine), fmt.Sprintf("sample %d resonator pitch", engine)}, Unit: "semitones", Min: -36, Max: 36, Curve: "linear", Description: "Coarse tuning for the Sample Shaper Resonator mode. Display conversion is treated as approximate until controlled calibration."},
			ParameterSpec{ID: prefix + "ResonatorDryWet", Friendly: label + " shaper resonator dry/wet", Aliases: []string{fmt.Sprintf("sample %d resonator mix", engine), fmt.Sprintf("sample %d resonator dry wet", engine)}, Unit: "percent", Min: 0, Max: 100, Description: "Dry/wet mix for the Sample Shaper Resonator mode."},
			ParameterSpec{ID: prefix + "ResonatorInharmonicity", Friendly: label + " shaper resonator inharmonicity", Aliases: []string{fmt.Sprintf("sample %d resonator inharmonicity", engine)}, Unit: "normalized", Min: 0, Max: 1, Description: "Bipolar inharmonicity control. Raw 0.5 is the documented neutral center; exact display conversion remains to be calibrated."},
			ParameterSpec{ID: prefix + "ResonatorQ", Friendly: label + " shaper resonator resonance", Aliases: []string{fmt.Sprintf("sample %d resonator resonance", engine), fmt.Sprintf("sample %d resonator q", engine)}, Unit: "percent", Min: 0, Max: 100, Description: "Resonance amount for the Sample Shaper Resonator mode."},
			ParameterSpec{ID: prefix + "FMAmount", Friendly: label + " shaper frequency modulation", Aliases: []string{fmt.Sprintf("sample %d frequency modulation", engine), fmt.Sprintf("sample %d shaper fm", engine)}, Unit: "percent", Min: 0, Max: 100, Description: "Frequency-modulation amount for the Sample Shaper Modulation mode."},
			ParameterSpec{ID: prefix + "RingModAmount", Friendly: label + " shaper ring modulation", Aliases: []string{fmt.Sprintf("sample %d ring modulation", engine), fmt.Sprintf("sample %d shaper ring mod", engine)}, Unit: "percent", Min: 0, Max: 100, Description: "Ring-modulation amount for the Sample Shaper Modulation mode."},
			ParameterSpec{ID: prefix + "ModOscVolume", Friendly: label + " modulator volume", Aliases: []string{fmt.Sprintf("sample %d modulator volume", engine)}, Unit: "db", Min: -70, Max: 0, Curve: "linear", Description: "Direct level of the Sample Engine modulation oscillator. Display conversion is approximate until controlled calibration."},
			ParameterSpec{ID: prefix + "ModOscRatio", Friendly: label + " modulator tune ratio", Aliases: []string{fmt.Sprintf("sample %d modulator ratio", engine), fmt.Sprintf("sample %d modulator tune", engine)}, Unit: "normalized", Min: 0, Max: 1, Description: "Stored tuning-ratio control for the Sample Engine modulator. The visible 0.25-48 ratio curve is not yet calibrated."},
			ParameterSpec{ID: prefix + "ModOscFine", Friendly: label + " modulator fine tune", Aliases: []string{fmt.Sprintf("sample %d modulator fine", engine)}, Unit: "semitones", Min: -1, Max: 1, Curve: "linear", Description: "Fine tuning of the Sample Engine modulator. Display conversion is approximate until controlled calibration."},
		)
		for sampleSlot := 1; sampleSlot <= 6; sampleSlot++ {
			KnownParameterSpecs = append(KnownParameterSpecs,
				ParameterSpec{ID: fmt.Sprintf("%sSlot%d_TrimStart", prefix, sampleSlot), Friendly: fmt.Sprintf("%s slot %c trim start", label, 'A'+rune(sampleSlot-1)), Aliases: []string{fmt.Sprintf("sample %d slot %c trim start", engine, 'A'+rune(sampleSlot-1))}, Unit: "percent", Min: 0, Max: 100, Description: "Per-slot sample trim start as a percentage of sample duration."},
				ParameterSpec{ID: fmt.Sprintf("%sSlot%d_TrimEnd", prefix, sampleSlot), Friendly: fmt.Sprintf("%s slot %c trim end", label, 'A'+rune(sampleSlot-1)), Aliases: []string{fmt.Sprintf("sample %d slot %c trim end", engine, 'A'+rune(sampleSlot-1))}, Unit: "percent", Min: 0, Max: 100, Description: "Per-slot sample trim end as a percentage of sample duration."},
			)
		}
	}
}

func init() {
	var payload masterParameterSpecFile
	if err := json.Unmarshal(rawMasterParameterSpecs, &payload); err != nil {
		return
	}
	existing := make(map[string]bool, len(KnownParameterSpecs))
	for _, spec := range KnownParameterSpecs {
		existing[spec.ID] = true
	}
	for _, spec := range payload.Specs {
		if strings.TrimSpace(spec.ID) == "" || existing[spec.ID] {
			continue
		}
		if spec.Curve == "" {
			spec.Curve = "linear"
		}
		KnownParameterSpecs = append(KnownParameterSpecs, spec)
		existing[spec.ID] = true
	}
}

var simplifiedAliases = map[string]string{
	"Engine1_Type":          "Engine1_ModuleType",
	"Engine2_Type":          "Engine2_ModuleType",
	"Engine1_Wvt_Waveform":  "Engine1_WTOsc_FrameIndex",
	"Engine2_Wvt_Waveform":  "Engine2_WTOsc_FrameIndex",
	"Filter1_Type":          "Filter1_ModuleType",
	"Filter2_Type":          "Filter2_ModuleType",
	"Lfo1_Rate":             "LFO1_RateUnSynced",
	"Lfo2_Rate":             "LFO2_RateUnSynced",
	"Lfo3_Rate":             "LFO3_RateUnSynced",
	"Lfo1_Waveform":         "LFO1_Waveform",
	"Lfo2_Waveform":         "LFO2_Waveform",
	"Lfo3_Waveform":         "LFO3_Waveform",
	"Utility_Noise1_Volume": "FilterMix_UtilityN1Volume",
	"Utility_Noise2_Volume": "FilterMix_UtilityN2Volume",
	"Utility_Volume":        "FilterMix_UtilitySOVolume",
	"Mod_Random1_Type":      "Random1_ModuleType",
	"Mod_Random1_Rate":      "Random1_RnH_Distance",
	"Fx1_Type":              "FX1_ModuleType", "Fx2_Type": "FX2_ModuleType", "Fx3_Type": "FX3_ModuleType", "Fx4_Type": "FX4_ModuleType", "Fx5_Type": "FX5_ModuleType", "Fx6_Type": "FX6_ModuleType",
	"Fx1_Amount": "FX1_Dry_Wet", "Fx2_Amount": "FX2_Dry_Wet", "Fx3_Amount": "FX3_Dry_Wet", "Fx4_Amount": "FX4_Dry_Wet", "Fx5_Amount": "FX5_Dry_Wet", "Fx6_Amount": "FX6_Dry_Wet",
}

func CanonicalParameterID(id string) string {
	trimmed := strings.TrimSpace(id)
	if mapped, ok := simplifiedAliases[trimmed]; ok {
		return mapped
	}
	for _, spec := range KnownParameterSpecs {
		if strings.EqualFold(trimmed, spec.ID) || strings.EqualFold(trimmed, spec.Friendly) {
			return spec.ID
		}
		for _, alias := range spec.Aliases {
			if strings.EqualFold(trimmed, alias) {
				return spec.ID
			}
		}
	}
	return trimmed
}

func SpecFor(id string) (ParameterSpec, bool) {
	canonical := CanonicalParameterID(id)
	for _, spec := range KnownParameterSpecs {
		if spec.ID == canonical {
			return spec, true
		}
	}
	return ParameterSpec{}, false
}

func ResolveEnum(spec ParameterSpec, value string) (string, bool) {
	v := strings.ToLower(strings.TrimSpace(value))
	if mapped, ok := spec.EnumValues[v]; ok {
		return mapped, true
	}
	return "", false
}

func CatalogPrompt() string {
	return CatalogPromptFiltered(nil)
}

// CatalogPromptFiltered builds the model-facing catalog while allowing the
// caller to omit calibration-locked IDs from natural-language planning. The
// deterministic editor may still inspect or directly modify an exact existing
// parameter when a separate trusted workflow permits it.
func CatalogPromptFiltered(allowed func(string) bool) string {
	var specs []ParameterSpec
	for _, spec := range KnownParameterSpecs {
		if allowed == nil || allowed(spec.ID) {
			specs = append(specs, spec)
		}
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].ID < specs[j].ID })
	var b strings.Builder
	b.WriteString("KNOWN SAFE PIGMENTS PARAMETERS (canonical ID | unit | purpose):\n")
	for _, spec := range specs {
		fmt.Fprintf(&b, "- %s | %s | %s", spec.ID, spec.Unit, spec.Friendly)
		if spec.Description != "" {
			fmt.Fprintf(&b, ": %s", spec.Description)
		}
		if spec.Unit == "enum" {
			keys := make([]string, 0, len(spec.EnumValues))
			for k := range spec.EnumValues {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			fmt.Fprintf(&b, " Values: %s.", strings.Join(keys, ", "))
		}
		b.WriteByte('\n')
	}
	b.WriteString("- Modulations_<Target>_<Source>_Amount | raw | Existing/addable modulation-routing IDs. Use allow_add=true only for IDs beginning Modulations_.\n")
	return b.String()
}
