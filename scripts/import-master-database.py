#!/usr/bin/env python3
"""Import the user-maintained Pigments 7 master database into compact runtime indexes.

The source database deliberately keeps UI documentation and raw .pgtx IDs separate.
This script adds a versioned, confidence-rated overlay without mutating the source.
"""
from __future__ import annotations

import argparse
import csv
import json
import re
from collections import Counter, defaultdict
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[1]
DEFAULT_SOURCE = ROOT / "knowledge" / "pigments7_master_database_v1_6.json"


def target(parameter_id: str, slot: str | None = None, role: str = "value", condition: str | None = None,
           legacy: bool = False, transform: str | None = None) -> dict[str, Any]:
    out: dict[str, Any] = {"parameter_id": parameter_id, "role": role}
    if slot:
        out["engine_slot"] = slot
    if condition:
        out["condition"] = condition
    if legacy:
        out["legacy"] = True
    if transform:
        out["transform"] = transform
    return out


def pair(prefix: str, suffix: str, **kwargs: Any) -> list[dict[str, Any]]:
    return [
        target(f"Engine1_{prefix}_{suffix}", "engine_1", **kwargs),
        target(f"Engine2_{prefix}_{suffix}", "engine_2", **kwargs),
    ]


def generic_pair(suffix: str, **kwargs: Any) -> list[dict[str, Any]]:
    return [target(f"Engine1_{suffix}", "engine_1", **kwargs), target(f"Engine2_{suffix}", "engine_2", **kwargs)]


def rule(targets: list[dict[str, Any]] | None = None, *, status: str = "high_confidence_name_match",
         confidence: str = "high", automatic: bool = True, conversion: str = "normalized_curve_unverified",
         notes: str = "", aliases: list[str] | None = None) -> dict[str, Any]:
    return {
        "targets": targets or [],
        "mapping_status": status,
        "confidence": confidence,
        "automatic_edit": automatic,
        "conversion_status": conversion,
        "mapping_notes": notes,
        "mapping_aliases": aliases or [],
    }


def build_rules() -> dict[str, dict[str, Any]]:
    r: dict[str, dict[str, Any]] = {}

    # Analog engine.
    r["analog.modulator.crossmod_source"] = rule(generic_pair("CrossModSource"), automatic=False,
        status="high_confidence_enum_candidate", conversion="enum_values_need_controlled_diff",
        notes="Exact generic CrossModSource IDs exist for both engine slots; selector values still need calibration.")
    r["analog.modulator.source_combination"] = rule(pair("VA3Osc", "ModMix"))
    r["analog.noise.color"] = rule(pair("VA3Osc", "NoiseType"))
    r["analog.noise.volume"] = rule(pair("VA3Osc", "NoiseGain"), conversion="display_db_curve_unverified")
    for osc in (1, 2, 3):
        r[f"analog.oscillator_{osc}.coarse"] = rule(pair("VA3Osc", f"Osc{osc}Range"), conversion="ui_semitone_curve_unverified")
        r[f"analog.oscillator_{osc}.volume"] = rule(pair("VA3Osc", f"Osc{osc}Volume"), conversion="display_db_curve_unverified")
        r[f"analog.oscillator_{osc}.wave_type"] = rule(pair("VA3Osc", f"Osc{osc}Wave"), automatic=False,
            status="high_confidence_enum_candidate", conversion="wave_enum_values_need_controlled_diff")
        r[f"analog.oscillator_{osc}.width"] = rule(pair("VA3Osc", f"Osc{osc}PulseWidth"))
    r["analog.oscillator_1.sync"] = rule(pair("VA3Osc", "HardSync"), conversion="binary")
    r["analog.oscillator_1.fm_switch"] = rule(pair("VA3Osc", "Osc1FM"), conversion="binary")
    r["analog.oscillator_2.fm_switch"] = rule(pair("VA3Osc", "Osc2FM"), conversion="binary")
    # One shared FMGain exists, so both per-oscillator UI labels require a controlled diff before assignment.
    for osc in (1, 2):
        r[f"analog.oscillator_{osc}.fm_amount"] = rule(pair("VA3Osc", "FMGain"), status="ambiguous_shared_parameter_candidate",
            confidence="medium", automatic=False, conversion="control_relationship_needs_diff",
            notes="The archive exposes one VA3Osc_FMGain plus Osc1FM/Osc2FM switches; a one-control preset pair is needed to confirm the visible amount relationship.")
    for osc in (2, 3):
        r[f"analog.oscillator_{osc}.fine"] = rule(
            pair("VA3Osc", f"Osc{osc}Detune", role="semitone_value", condition="Fine display mode is semitones")
            + pair("VA3Osc", f"Osc{osc}FreqOffset", role="hertz_value", condition="Fine display mode is hertz")
            + pair("VA3Osc", f"Osc{osc}FreqOffsetMode", role="mode"),
            status="conditional_multi_parameter_candidate", confidence="medium", automatic=False,
            conversion="dual_mode_requires_controlled_diff",
            notes="The visible dual-unit Fine control appears to use Detune, FreqOffset and FreqOffsetMode.")
        r[f"analog.oscillator_{osc}.key_follower"] = rule(pair("VA3Osc", f"Osc{osc}Key"), conversion="binary")
    r["analog.tune.coarse"] = rule(pair("VA3Osc", "CoarseTune"), conversion="ui_semitone_curve_unverified")
    r["analog.tune.drift"] = rule(pair("VA3Osc", "PitchDriftST"))
    r["analog.tune.fine"] = rule(pair("VA3Osc", "FineTune"), conversion="ui_semitone_curve_unverified")
    r["analog.tune.keyboard_tracking"] = rule(pair("VA3Osc", "KeyTrack"), conversion="binary")
    r["analog.tune.q"] = rule(
        pair("VA3Osc", "Quantize", role="enabled") + pair("VA3Osc", "Scale", role="scale_or_root"),
        status="conditional_multi_parameter_candidate", confidence="medium", automatic=False,
        conversion="pitch_class_mapping_needs_controlled_diff")
    r["analog.unison.detune"] = rule(pair("VA3Osc", "Unison_Detune"))
    r["analog.unison.stereo"] = rule(pair("VA3Osc", "Unison_Stereo"))
    r["analog.unison.voices"] = rule(
        pair("VA3Osc", "UnisonVoices") + [
            target("Engine1_VA3Osc_Unison_Voices", "engine_1", legacy=True),
            target("Engine2_VA3Osc_Unison_Voices", "engine_2", legacy=True),
        ], conversion="stepped_voice_curve_needs_calibration", automatic=False,
        status="high_confidence_with_legacy_id", notes="Both current and legacy voice-count spellings occur in the imported archives.")

    # Common engine header and output.
    r["common.engine.enabled"] = rule([
        target("Engine1_Bypass", "engine_1", transform="invert_boolean"),
        target("Engine2_Bypass", "engine_2", transform="invert_boolean"),
    ], status="confirmed_inverse_semantics", confidence="high", automatic=False, conversion="inverted_boolean",
       notes="Pigments stores Bypass; the visible Power state has inverse semantics. The current compiler does not expose an inverted UI alias.")
    r["common.engine_output.filter_mix"] = rule([
        target("FilterMix_Engine1FilterMix", "engine_1"), target("FilterMix_Engine2FilterMix", "engine_2"),
        target("FilterMix_UtilitySOFilterMix", "utility_engine", role="utility_oscillator"),
        target("FilterMix_UtilityN1FilterMix", "utility_engine", role="utility_noise_1"),
        target("FilterMix_UtilityN2FilterMix", "utility_engine", role="utility_noise_2"),
    ], status="platform_verified", conversion="normalized_linear")
    r["common.engine_output.volume"] = rule([
        target("FilterMix_Engine1Volume", "engine_1"), target("FilterMix_Engine2Volume", "engine_2"),
        target("FilterMix_UtilitySOVolume", "utility_engine", role="utility_oscillator"),
        target("FilterMix_UtilityN1Volume", "utility_engine", role="utility_noise_1"),
        target("FilterMix_UtilityN2Volume", "utility_engine", role="utility_noise_2"),
    ], status="platform_verified", conversion="display_db_linear_approximation")

    # Wavetable engine.
    r["wavetable.freq_ring_mod.amount"] = rule(pair("WTOsc", "FMAmount"), status="platform_verified")
    r["wavetable.freq_ring_mod.type"] = rule(pair("WTOsc", "FMType"), automatic=False,
        status="high_confidence_enum_candidate", conversion="enum_values_need_controlled_diff")
    r["wavetable.modulator.fine"] = rule(pair("WTOsc", "ModOscFine"), conversion="ui_semitone_curve_unverified")
    r["wavetable.modulator.source"] = rule(generic_pair("CrossModSource"), automatic=False,
        status="high_confidence_enum_candidate", conversion="enum_values_need_controlled_diff")
    r["wavetable.modulator.tune"] = rule(pair("WTOsc", "ModOscRatio"), automatic=False,
        status="high_confidence_ratio_candidate", conversion="ratio_curve_needs_controlled_diff")
    r["wavetable.modulator.volume"] = rule(pair("WTOsc", "ModOscVolume"), conversion="display_db_curve_unverified")
    r["wavetable.modulator.wave"] = rule(pair("WTOsc", "ModOscWf"), automatic=False,
        status="high_confidence_enum_candidate", conversion="wave_enum_values_need_controlled_diff")
    r["wavetable.phase_mod.amount"] = rule(pair("WTOsc", "PMAmount"), status="platform_verified")
    r["wavetable.phase_mod.phase"] = rule(pair("WTOsc", "Phase"), status="platform_verified", conversion="normalized_to_degrees")
    r["wavetable.phase_mod.retrig"] = rule(pair("WTOsc", "Sync"), automatic=False,
        status="high_confidence_enum_candidate", conversion="retrig_enum_values_need_controlled_diff")
    r["wavetable.phase_transform.amount"] = rule(pair("WTOsc", "PhaseDist"), status="platform_verified")
    r["wavetable.phase_transform.mod_amount"] = rule(pair("WTOsc", "PDAmount"))
    r["wavetable.phase_transform.type"] = rule(pair("WTOsc", "PDSourceIndex"), automatic=False,
        status="high_confidence_enum_candidate", conversion="phase_transform_enum_values_need_controlled_diff",
        notes="Observed values are discrete and align with the selector role; a complete option calibration is still required.")
    r["wavetable.tune.coarse"] = rule(pair("WTOsc", "Coarse"), conversion="ui_semitone_curve_unverified")
    r["wavetable.tune.drift"] = rule([], status="unmapped_no_wavetable_specific_id", confidence="low", automatic=False,
        conversion="unknown", notes="The imported inventory exposes Analog PitchDriftST but no clear WTOsc drift ID.")
    r["wavetable.tune.fine"] = rule(pair("WTOsc", "Fine"), conversion="ui_semitone_curve_unverified")
    r["wavetable.tune.keyboard_tracking"] = rule(pair("WTOsc", "KeyTrack"), conversion="binary")
    r["wavetable.tune.q"] = rule(
        pair("WTOsc", "Quantize", role="enabled") + pair("WTOsc", "Scale", role="scale") + pair("WTOsc", "Scales_RootNote", role="root_note"),
        status="conditional_multi_parameter_candidate", confidence="medium", automatic=False,
        conversion="pitch_class_mapping_needs_controlled_diff")
    r["wavetable.unison.detune"] = rule(pair("WTOsc", "UnisonDetune"), status="platform_verified")
    r["wavetable.unison.phase"] = rule(pair("WTOsc", "Unison_PhaseControl"))
    r["wavetable.unison.stereo"] = rule(pair("WTOsc", "UnisonStereo"), status="platform_verified")
    r["wavetable.unison.voices"] = rule(pair("WTOsc", "UnisonVoices"), automatic=False,
        status="high_confidence_stepped_candidate", conversion="voice_count_curve_needs_controlled_diff")
    r["wavetable.wave.morph"] = rule(pair("WTOsc", "Morph"), conversion="binary")
    r["wavetable.wave.position"] = rule(pair("WTOsc", "FrameIndex"), status="platform_verified")
    r["wavetable.wave.view_mode"] = rule([], status="ui_state_not_confirmed_in_preset", confidence="low", automatic=False,
        conversion="not_a_safe_preset_parameter")
    r["wavetable.wave.volume"] = rule(pair("WTOsc", "MainVolume"), status="platform_verified", conversion="display_db_curve_unverified")
    r["wavetable.wave.wavetable_select"] = rule([], status="serialized_object_mapping_required", confidence="high", automatic=False,
        conversion="asset_object_not_numeric", notes="Wavetable choice is represented in the serialized object layer, not as a safely calibrated numeric selector.")
    r["wavetable.wavefolding.fold"] = rule(pair("WTOsc", "Fold"), status="platform_verified")
    r["wavetable.wavefolding.mod_amount"] = rule(pair("WTOsc", "FoldAmount"))
    r["wavetable.wavefolding.shape"] = rule(pair("WTOsc", "FoldSourceIndex"), automatic=False,
        status="high_confidence_enum_candidate", conversion="fold_shape_enum_values_need_controlled_diff")

    # Sample engine tune, shaper, viewer, and granular controls.
    r["sample.tune.keyboard_tracking"] = rule(pair("SampleGranularOsc", "KeyTrack"), status="platform_verified", conversion="binary")
    r["sample.tune.q"] = rule(
        pair("SampleGranularOsc", "Quantize", role="enabled") + pair("SampleGranularOsc", "Scale", role="scale_or_root"),
        status="conditional_multi_parameter_candidate", confidence="medium", automatic=False,
        conversion="pitch_class_mapping_needs_controlled_diff")
    r["sample.tune.coarse"] = rule(pair("SampleGranularOsc", "Coarse"), status="platform_verified", conversion="ui_semitone_linear_approximation")
    r["sample.tune.fine"] = rule(pair("SampleGranularOsc", "Fine"), status="platform_verified", conversion="ui_semitone_linear_approximation")
    r["sample.tune.filter"] = rule(pair("SampleGranularOsc", "Filter"), status="platform_verified", conversion="normalized_bipolar")
    r["sample.shaper.mode"] = rule(pair("SampleGranularOsc", "Effect Type"), status="platform_verified", conversion="known_enum")
    r["sample.shaper.enabled"] = rule(pair("SampleGranularOsc", "Enable Effect"), status="platform_verified", conversion="binary")
    r["sample.shaper.bitcrush.decimate"] = rule(pair("SampleGranularOsc", "BitCrushDecimate"), status="platform_verified")
    r["sample.shaper.bitcrush.decimate_mode"] = rule(pair("SampleGranularOsc", "BitCrushMode"), status="platform_verified", automatic=False,
        conversion="binary_enum_direction_needs_audition")
    r["sample.shaper.bitcrush.bit_depth"] = rule(pair("SampleGranularOsc", "BitCrushBitDepth"), status="platform_verified", conversion="reverse_linear_approximation")
    r["sample.shaper.bitcrush.key_track"] = rule(pair("SampleGranularOsc", "BitCrushPitchFollow"), status="platform_verified", conversion="binary")
    r["sample.browser.sample_select"] = rule([], status="serialized_audio_object_mapping_required", confidence="high", automatic=False,
        conversion="asset_object_not_numeric")
    r["sample.viewer.mode"] = rule([], status="ui_state_not_confirmed_in_preset", confidence="low", automatic=False,
        conversion="not_a_safe_preset_parameter")
    r["sample.viewer.timeline_ruler"] = rule([], status="display_only", confidence="high", automatic=False,
        conversion="not_a_preset_parameter")
    r["sample.viewer.active_slot"] = rule(
        pair("SampleGranularOsc", "SamplePick", role="active_slot") + pair("SampleGranularOsc", "SinglePick", role="selection_mode"),
        status="candidate_requires_controlled_diff", confidence="medium", automatic=False,
        conversion="slot_enum_values_need_controlled_diff")
    r["sample.sample_grain.start"] = rule(pair("SampleGranularOsc", "Start"), status="platform_verified", conversion="normalized_sample_duration")
    r["sample.sample_grain.volume"] = rule(pair("SampleGranularOsc", "MainVolume"), conversion="display_db_curve_unverified")
    r["sample.granular.enabled"] = rule(pair("SampleGranularOsc", "GranularOn"), conversion="binary")
    r["sample.granular.limit"] = rule(pair("SampleGranularOsc", "MaxGrains"), automatic=False,
        status="high_confidence_stepped_candidate", conversion="grain_count_curve_needs_controlled_diff")
    r["sample.granular.scan"] = rule(pair("SampleGranularOsc", "Speed"), status="high_confidence_name_match")
    r["sample.granular.density"] = rule(
        pair("SampleGranularOsc", "GranularPhaseHelper_RateUnSynced", role="hz", condition="Density Mode is Hz")
        + pair("SampleGranularOsc", "GranularPhaseHelper_RateSynced", role="synced_rate", condition="Density Mode is tempo-synced")
        + pair("SampleGranularOsc", "GranularPhaseHelper_AllRateSynced", role="all_synced_rate", condition="Density Mode uses the combined synced selector"),
        status="conditional_multi_parameter_candidate", confidence="medium", automatic=False,
        conversion="rate_curve_and_mode_need_controlled_diff")
    r["sample.granular.density_mode"] = rule(pair("SampleGranularOsc", "DensityType"), automatic=False,
        status="high_confidence_enum_candidate", conversion="enum_values_need_controlled_diff")
    r["sample.granular.shape"] = rule(pair("SampleGranularOsc", "EnvelopeParam"))
    r["sample.granular.shape_type"] = rule(pair("SampleGranularOsc", "Envelope"), automatic=False,
        status="high_confidence_enum_candidate", conversion="enum_values_need_controlled_diff")
    r["sample.granular.size"] = rule(
        pair("SampleGranularOsc", "GrainSizeAbsolute", role="absolute", condition="Size Mode is time")
        + pair("SampleGranularOsc", "GrainSizeRatio", role="ratio", condition="Size Mode is ratio")
        + pair("SampleGranularOsc", "GrainSizeRatioContinuous", role="continuous_ratio", condition="Size Mode is continuous ratio")
        + pair("SampleGranularOsc", "GrainSizeSynced", role="synced", condition="Size Mode is tempo-synced"),
        status="conditional_multi_parameter_candidate", confidence="medium", automatic=False,
        conversion="size_mode_curve_needs_controlled_diff")
    r["sample.granular.size_mode"] = rule(pair("SampleGranularOsc", "SizeMode"), automatic=False,
        status="high_confidence_enum_candidate", conversion="enum_values_need_controlled_diff")
    simple_sample = {
        "sample.granular.random_start": "RandomStart",
        "sample.granular.random_start_mode": "RandomStartArrow",
        "sample.granular.random_pitch": "RandomPitch",
        "sample.granular.random_pitch_mode": "RandomPitchArrow",
        "sample.granular.random_density": "RandomDensity",
        "sample.granular.random_density_mode": "RandomDensityArrow",
        "sample.granular.direction": "RandomDirection",
        "sample.granular.random_size": "RandomSize",
        "sample.granular.random_size_mode": "RandomSizeArrow",
        "sample.granular.stereo": "RandomPan",
        "sample.granular.stereo_mode": "PanMode",
        "sample.granular.random_volume": "RandomVolume",
    }
    for control_id, suffix in simple_sample.items():
        is_mode = control_id.endswith("_mode") or control_id.endswith(".stereo_mode")
        r[control_id] = rule(pair("SampleGranularOsc", suffix), automatic=not is_mode,
            status="high_confidence_enum_candidate" if is_mode else "high_confidence_name_match",
            conversion="enum_values_need_controlled_diff" if is_mode else "normalized_curve_unverified")
    r["sample.modulator.source"] = rule(generic_pair("CrossModSource"), automatic=False,
        status="high_confidence_enum_candidate", conversion="enum_values_need_controlled_diff")
    r["sample.modulator.wave"] = rule(pair("SampleGranularOsc", "ModOscWf"), automatic=False,
        status="high_confidence_enum_candidate", conversion="wave_enum_values_need_controlled_diff")
    r["sample.modulator.volume"] = rule(pair("SampleGranularOsc", "ModOscVolume"), conversion="display_db_curve_unverified")
    r["sample.modulator.tune"] = rule(pair("SampleGranularOsc", "ModOscRatio"), automatic=False,
        status="high_confidence_ratio_candidate", conversion="ratio_curve_needs_controlled_diff")
    r["sample.modulator.fine"] = rule(pair("SampleGranularOsc", "ModOscFine"), conversion="ui_semitone_curve_unverified")
    r["sample.shaper.unison.detune"] = rule(pair("SampleGranularOsc", "UnisonDetune"))
    r["sample.shaper.unison.phase"] = rule(pair("SampleGranularOsc", "Unison_PhaseControl"))
    r["sample.shaper.unison.stereo"] = rule(pair("SampleGranularOsc", "UnisonStereo"))
    r["sample.shaper.unison.voices"] = rule(pair("SampleGranularOsc", "UnisonVoices"), automatic=False,
        status="high_confidence_stepped_candidate", conversion="voice_count_curve_needs_controlled_diff")
    r["sample.shaper.chord.chord"] = rule(pair("SampleGranularOsc", "UnisonChord"), automatic=False,
        status="high_confidence_enum_candidate", conversion="chord_enum_values_need_controlled_diff")
    r["sample.shaper.chord.stereo"] = rule(pair("SampleGranularOsc", "UnisonStereo"))
    r["sample.shaper.chord.voices"] = rule(pair("SampleGranularOsc", "UnisonVoices"), automatic=False,
        status="high_confidence_stepped_candidate", conversion="voice_count_curve_needs_controlled_diff")
    r["sample.shaper.super.detune"] = rule(pair("SampleGranularOsc", "UnisonDetune"))
    r["sample.shaper.super.mix"] = rule(pair("SampleGranularOsc", "UnisonMix"))
    r["sample.shaper.super.stereo"] = rule(pair("SampleGranularOsc", "UnisonStereo"))
    r["sample.shaper.resonator.coarse"] = rule(pair("SampleGranularOsc", "ResonatorFcCoarse"), conversion="ui_semitone_curve_unverified")
    r["sample.shaper.resonator.dry_wet"] = rule(pair("SampleGranularOsc", "ResonatorDryWet"))
    r["sample.shaper.resonator.inharmonicity"] = rule(pair("SampleGranularOsc", "ResonatorInharmonicity"))
    r["sample.shaper.resonator.resonance"] = rule(pair("SampleGranularOsc", "ResonatorQ"))
    r["sample.shaper.modulation.freq_mod"] = rule(pair("SampleGranularOsc", "FMAmount"))
    r["sample.shaper.modulation.ring_mod"] = rule(pair("SampleGranularOsc", "RingModAmount"))

    return r


def flatten_aliases(control: dict[str, Any]) -> list[str]:
    aliases: list[str] = []
    if control.get("normalized_name"):
        aliases.append(str(control["normalized_name"]))
    for row in control.get("aliases") or []:
        text = row.get("alias_text")
        if text:
            aliases.append(str(text))
    seen: set[str] = set()
    out: list[str] = []
    for text in aliases:
        key = text.casefold()
        if key not in seen:
            seen.add(key)
            out.append(text)
    return out


def display_unit(control: dict[str, Any]) -> str:
    unit = control.get("unit")
    if unit:
        return str(unit)
    ctype = control.get("control_type", "")
    if "enum" in ctype or ctype in {"source_selector", "browser_selector"}:
        return "enum"
    if ctype == "toggle":
        return "boolean"
    return "normalized_or_contextual"


def safe_spec_unit(control: dict[str, Any], mapping: dict[str, Any]) -> tuple[str | None, float | None, float | None, str]:
    """Return compiler unit/min/max/curve for conservative auto-edit specs."""
    if not mapping.get("automatic_edit"):
        return None, None, None, ""
    ctype = str(control.get("control_type") or "")
    minv = control.get("min_value")
    maxv = control.get("max_value")
    unit = control.get("unit")
    conversion = mapping.get("conversion_status", "")
    if ctype == "toggle":
        return "boolean", 0.0, 1.0, "linear"
    if ctype in {"enum_selector", "cyclic_enum_selector", "source_selector", "browser_selector"}:
        return None, None, None, ""
    if control["control_id"] == "sample.shaper.bitcrush.bit_depth":
        return "bits", 16.0, 1.5, "linear"
    if unit == "st" and isinstance(minv, (int, float)) and isinstance(maxv, (int, float)):
        return "semitones", float(minv), float(maxv), "linear"
    if unit == "%":
        return "percent", 0.0, 100.0, "linear"
    if unit == "dB":
        # Without a controlled curve, use normalized percent rather than claim exact dB.
        return "percent", 0.0, 100.0, "linear"
    if unit == "degrees":
        return "percent", 0.0, 100.0, "linear"
    if isinstance(minv, (int, float)) and isinstance(maxv, (int, float)):
        # A documented bipolar or stepped UI range does not prove that the
        # serialized value uses that same numeric domain. Pigments preset state
        # is usually normalized, so percent-of-knob-travel is the only safe
        # automatic unit until a controlled pair calibrates the conversion.
        return "percent", 0.0, 100.0, "linear"
    if conversion in {"binary", "inverted_boolean"}:
        return "boolean", 0.0, 1.0, "linear"
    return "percent", 0.0, 100.0, "linear"


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", type=Path, default=DEFAULT_SOURCE)
    args = parser.parse_args()
    source = args.source.resolve()
    data = json.loads(source.read_text(encoding="utf-8"))

    rules = build_rules()
    controls = data.get("controls", [])
    control_ids = {c["control_id"] for c in controls}
    missing_rules = sorted(control_ids - set(rules))
    extra_rules = sorted(set(rules) - control_ids)
    if missing_rules or extra_rules:
        raise SystemExit(f"mapping rule mismatch; missing={missing_rules}, extra={extra_rules}")

    internal_rows = data["pgtx_import"]["internal_parameters"]
    internal_ids = {row["internal_parameter_id"] for row in internal_rows}
    missing_targets: list[tuple[str, str]] = []
    for control_id, mapping in rules.items():
        for row in mapping["targets"]:
            if row["parameter_id"] not in internal_ids:
                missing_targets.append((control_id, row["parameter_id"]))
    if missing_targets:
        raise SystemExit("mapping targets missing from imported inventory: " + repr(missing_targets[:20]))

    modules = {row["module_id"]: row for row in data.get("modules", [])}
    engine_types = {row["engine_type_id"]: row for row in data.get("engine_types", [])}

    sections: dict[str, dict[str, Any]] = {}
    mapping_rows: list[dict[str, Any]] = []
    safe_specs: dict[str, dict[str, Any]] = {}
    for control in controls:
        control_id = control["control_id"]
        mapping = rules[control_id]
        module = modules.get(control["module_id"], {})
        aliases = flatten_aliases(control)
        parameter = {
            "control_id": control_id,
            "ui_name": control.get("display_name"),
            "normalized_name": control.get("normalized_name"),
            "control": control.get("control_type"),
            "unit": display_unit(control),
            "minimum": control.get("min_value"),
            "maximum": control.get("max_value"),
            "default": control.get("default_value"),
            "step": control.get("step_value"),
            "value_format": control.get("value_format"),
            "values": control.get("options") or [],
            "variants": control.get("variants") or [],
            "endpoints": control.get("endpoints") or {},
            "aliases": aliases,
            "data_status": control.get("data_status"),
            "description": control.get("notes") or "",
            "dependencies": control.get("dependencies") or [],
            "source_id": control.get("source_id"),
            "evidence_statuses": sorted({str(x.get("verification_status")) for x in control.get("evidence") or [] if x.get("verification_status")}),
            "canonical_ids": [x["parameter_id"] for x in mapping["targets"] if not x.get("legacy")],
            "mapping_targets": mapping["targets"],
            "mapping_status": mapping["mapping_status"],
            "mapping_confidence": mapping["confidence"],
            "automatic_edit": mapping["automatic_edit"],
            "conversion_status": mapping["conversion_status"],
            "mapping_notes": mapping["mapping_notes"],
        }
        section_id = control["module_id"]
        if section_id not in sections:
            engine_type_id = module.get("engine_type_id")
            engine = engine_types.get(engine_type_id, {}) if engine_type_id else {}
            sections[section_id] = {
                "id": section_id,
                "label": f"{engine.get('display_name') + ' / ' if engine.get('display_name') else ''}{module.get('display_name', section_id)}",
                "engine_type_id": engine_type_id,
                "module_category": module.get("module_category"),
                "data_status": module.get("data_status"),
                "parameters": [],
            }
        sections[section_id]["parameters"].append(parameter)
        mapping_rows.append({"control_id": control_id, **mapping})

        unit, minv, maxv, curve = safe_spec_unit(control, mapping)
        if unit:
            for target_row in mapping["targets"]:
                if target_row.get("legacy") or target_row.get("transform") or target_row.get("condition") or target_row.get("role") not in {None, "value"}:
                    continue
                pid = target_row["parameter_id"]
                if pid in safe_specs:
                    continue
                slot = target_row.get("engine_slot", "")
                slot_label = {"engine_1": "Engine 1", "engine_2": "Engine 2", "utility_engine": "Utility Engine"}.get(slot, slot)
                friendly = f"{slot_label} {control.get('normalized_name') or control.get('display_name')}".strip()
                spec_aliases = [control_id, control.get("display_name", ""), *(aliases or [])]
                spec_aliases = [x for x in dict.fromkeys(str(x).strip() for x in spec_aliases) if x]
                safe_specs[pid] = {
                    "id": pid,
                    "friendly": friendly,
                    "aliases": spec_aliases,
                    "unit": unit,
                    "min": minv,
                    "max": maxv,
                    "curve": curve,
                    "enum_values": {},
                    "description": (control.get("notes") or f"Mapped from {control_id} in the Pigments 7 master database.")
                        + f" Mapping status: {mapping['mapping_status']}; conversion: {mapping['conversion_status']}.",
                    "source_control_id": control_id,
                    "mapping_status": mapping["mapping_status"],
                    "conversion_status": mapping["conversion_status"],
                }

    # Known, calibrated enum values already supported by the compiler.
    sample_shaper_enum = {"unison": "0", "chord": "0.2", "super": "0.4", "resonator": "0.6", "bit crush": "0.8", "bitcrush": "0.8", "modulation": "1"}
    for engine in (1, 2):
        pid = f"Engine{engine}_SampleGranularOsc_Effect Type"
        safe_specs[pid] = {
            "id": pid,
            "friendly": f"Sample Engine {engine} shaper mode",
            "aliases": [f"sample engine {engine} shaper mode", "sample.shaper.mode"],
            "unit": "enum", "min": 0.0, "max": 1.0, "curve": "linear",
            "enum_values": sample_shaper_enum,
            "description": "Selects Unison, Chord, Super, Resonator, Bit Crush, or Modulation.",
            "source_control_id": "sample.shaper.mode", "mapping_status": "platform_verified", "conversion_status": "known_enum",
        }

    section_list = sorted(sections.values(), key=lambda x: x["id"])
    for section in section_list:
        section["parameters"].sort(key=lambda x: x["control_id"])

    status_counts = Counter(row["mapping_status"] for row in mapping_rows)
    confidence_counts = Counter(row["confidence"] for row in mapping_rows)
    automatic_control_count = sum(1 for row in mapping_rows if row["automatic_edit"])
    mapped_control_count = sum(1 for row in mapping_rows if row["targets"])
    unique_target_ids = {t["parameter_id"] for row in mapping_rows for t in row["targets"]}

    dbmeta = data["database"]
    summary = data["pgtx_import"]["summary"]
    notes = [
        "This runtime catalog is generated from the user-maintained Pigments 7 master database; the source database remains unchanged.",
        "UI documentation and raw .pgtx IDs are separate evidence layers. The mapping overlay records confidence and does not silently promote candidates to verified conversions.",
        "automatic_edit=true means the compiler may address the numeric ID conservatively; it does not mean every displayed-unit curve is exact.",
        "Controls involving assets, UI-only state, conditional modes, inverted power semantics, or uncalibrated enums remain excluded from automatic editing.",
    ]
    public_catalog = {
        "schema_version": "2.0.0",
        "product": dbmeta.get("plugin"),
        "target_version": dbmeta.get("plugin_latest_tracked_version") or dbmeta.get("plugin_baseline_version"),
        "source_database": {
            "name": dbmeta.get("name"), "schema_version": dbmeta.get("schema_version"),
            "data_revision": dbmeta.get("data_revision"), "last_updated": dbmeta.get("last_updated"),
            "plugin_build": dbmeta.get("pgtx_observed_plugin_build"),
            "version_knowledge_state": dbmeta.get("version_knowledge_state"),
        },
        "statistics": {
            "ui_control_count": len(controls),
            "module_count": len(data.get("modules", [])),
            "mapped_ui_control_count": mapped_control_count,
            "automatic_edit_control_count": automatic_control_count,
            "unique_mapped_internal_id_count": len(unique_target_ids),
            "internal_parameter_count": summary.get("unique_internal_parameter_count"),
            "preset_value_row_count": summary.get("total_preset_parameter_value_rows"),
            "preset_count": summary.get("preset_count"),
            "mapping_status_counts": dict(sorted(status_counts.items())),
            "mapping_confidence_counts": dict(sorted(confidence_counts.items())),
        },
        "notes": notes,
        "sections": section_list,
        "sample_browser": {
            "banks": data.get("sample_banks", []),
            "visible_entries": data.get("sample_library_entries", []),
            "slots": data.get("sample_slots", []),
            "exhaustive": False,
        },
    }

    internal_index = {
        "schema_version": "1.0.0",
        "source_database_revision": dbmeta.get("data_revision"),
        "internal_parameters": [
            {
                "id": row.get("internal_parameter_id"), "namespace": row.get("namespace"),
                "subsystem": row.get("subsystem"), "slot_index": row.get("slot_index"),
                "family_hint": row.get("family_hint"), "presence_count": row.get("presence_count"),
                "distinct_observed_value_count": row.get("distinct_observed_value_count"),
                "observed_min": row.get("observed_min"), "observed_max": row.get("observed_max"),
                "value_domain": row.get("value_domain"),
            }
            for row in internal_rows
        ],
    }

    outputs = {
        ROOT / "internal" / "knowledge" / "pigments_master_catalog.json": public_catalog,
        ROOT / "internal" / "knowledge" / "pigments_internal_index.json": internal_index,
        ROOT / "knowledge" / "control_parameter_mappings_v1_0.json": {
            "schema_version": "1.0.0", "source_database": source.name,
            "source_database_revision": dbmeta.get("data_revision"), "mappings": mapping_rows,
        },
        ROOT / "internal" / "arturia" / "master_parameter_specs.json": {
            "schema_version": "1.0.0", "specs": sorted(safe_specs.values(), key=lambda x: x["id"]),
        },
    }
    for path, payload in outputs.items():
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(payload, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")

    csv_path = ROOT / "knowledge" / "control_parameter_mappings_v1_0.csv"
    csv_fields = [
        "control_id", "mapping_status", "mapping_confidence", "automatic_edit",
        "conversion_status", "parameter_id", "role", "engine_slot", "condition",
        "transform", "legacy", "mapping_notes",
    ]
    with csv_path.open("w", newline="", encoding="utf-8") as handle:
        writer = csv.DictWriter(handle, fieldnames=csv_fields)
        writer.writeheader()
        for row in mapping_rows:
            targets = row.get("targets") or [{}]
            for mapping_target in targets:
                writer.writerow({
                    "control_id": row["control_id"],
                    "mapping_status": row["mapping_status"],
                    "mapping_confidence": row["confidence"],
                    "automatic_edit": str(bool(row["automatic_edit"])).lower(),
                    "conversion_status": row["conversion_status"],
                    "parameter_id": mapping_target.get("parameter_id", ""),
                    "role": mapping_target.get("role", ""),
                    "engine_slot": mapping_target.get("engine_slot", ""),
                    "condition": mapping_target.get("condition", ""),
                    "transform": mapping_target.get("transform", ""),
                    "legacy": str(bool(mapping_target.get("legacy", False))).lower(),
                    "mapping_notes": row["mapping_notes"],
                })

    report = ROOT / "docs" / "MASTER_DATABASE_IMPORT_REPORT.md"
    report.write_text(
        "# Pigments 7 master database import report\n\n"
        f"- Source: `{source.name}`\n"
        f"- Source schema: `{dbmeta.get('schema_version')}`\n"
        f"- Source revision: `{dbmeta.get('data_revision')}`\n"
        f"- Pigments build observed in presets: `{dbmeta.get('pgtx_observed_plugin_build')}`\n"
        f"- UI controls imported: **{len(controls)}**\n"
        f"- Internal parameter IDs indexed: **{len(internal_rows)}**\n"
        f"- UI controls with one or more mapping targets: **{mapped_control_count}**\n"
        f"- Controls permitted for conservative automatic numeric editing: **{automatic_control_count}**\n"
        f"- Unique mapped internal IDs: **{len(unique_target_ids)}**\n"
        f"- Generated compiler specs: **{len(safe_specs)}**\n\n"
        "## Mapping policy\n\n"
        "The import does not modify the master database. A separate overlay classifies each relationship as platform-verified, high-confidence by naming/structure, conditional, ambiguous, UI-only, or asset-object based. Candidate enums and conditional controls remain unavailable for automatic editing until a controlled before/after preset pair establishes the stored values.\n\n"
        "## Mapping-status counts\n\n" + "\n".join(f"- `{k}`: {v}" for k, v in sorted(status_counts.items())) + "\n",
        encoding="utf-8",
    )

    print(json.dumps(public_catalog["statistics"], indent=2))
    print(f"Generated {len(safe_specs)} conservative compiler specs")


if __name__ == "__main__":
    main()
