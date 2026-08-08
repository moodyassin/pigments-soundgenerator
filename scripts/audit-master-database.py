#!/usr/bin/env python3
"""Audit the Pigments master database and its generated runtime overlays.

The audit is intentionally read-only. It validates identifiers, references,
source counts, mapping targets, compiler specs, and default-template coverage.
"""
from __future__ import annotations

import argparse
import hashlib
import json
from collections import Counter
from pathlib import Path
from typing import Any, Iterable

ROOT = Path(__file__).resolve().parents[1]
DEFAULT_SOURCE = ROOT / "knowledge" / "pigments7_master_database_v1_6.json"
DEFAULT_MAPPING = ROOT / "knowledge" / "control_parameter_mappings_v1_0.json"
DEFAULT_SPECS = ROOT / "internal" / "arturia" / "master_parameter_specs.json"
DEFAULT_TEMPLATE = ROOT / "internal" / "arturia" / "Default"
DEFAULT_REPORT = ROOT / "docs" / "MASTER_DATABASE_AUDIT.md"


def load_json(path: Path) -> Any:
    return json.loads(path.read_text(encoding="utf-8"))


def sha256(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as fh:
        for chunk in iter(lambda: fh.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def ids(rows: Iterable[dict[str, Any]], key: str, label: str, errors: list[str]) -> set[str]:
    values: list[str] = []
    for index, row in enumerate(rows):
        value = row.get(key)
        if not isinstance(value, str) or not value:
            errors.append(f"{label}[{index}] has no non-empty {key}")
            continue
        values.append(value)
    duplicates = sorted(value for value, count in Counter(values).items() if count > 1)
    if duplicates:
        errors.append(f"{label} has duplicate {key} values: {duplicates[:20]}")
    return set(values)


def require_ref(value: Any, valid: set[str], location: str, errors: list[str], *, optional: bool = False) -> None:
    if value in (None, "") and optional:
        return
    if value not in valid:
        errors.append(f"{location} references missing ID {value!r}")


def extract_template_parameter_ids(path: Path) -> set[str]:
    data = path.read_bytes()
    marker = b"36 AfterTouchCurve_LastActivePointIndex"
    first = data.find(marker)
    if first < 0:
        raise ValueError("default template parameter marker not found")
    suffix = data.rfind(b" 0 0 0 ", 0, first)
    if suffix < 0:
        raise ValueError("default template parameter-count suffix not found")
    space = data.rfind(b" ", 0, suffix)
    count = int(data[space + 1 : suffix])
    pos = first
    result: set[str] = set()
    for item in range(count):
        length_end = data.find(b" ", pos)
        if length_end < 0:
            raise ValueError(f"truncated parameter length at item {item}")
        id_length = int(data[pos:length_end])
        id_start = length_end + 1
        id_end = id_start + id_length
        if id_end >= len(data) or data[id_end] != 0x20:
            raise ValueError(f"malformed parameter ID at item {item}")
        parameter_id = data[id_start:id_end].decode("utf-8")
        value_start = id_end + 1
        value_end = data.find(b" ", value_start)
        if value_end < 0:
            raise ValueError(f"truncated parameter value at item {item}")
        if parameter_id in result:
            raise ValueError(f"duplicate parameter ID in template: {parameter_id}")
        result.add(parameter_id)
        pos = value_end + 1
    return result


def audit(source: Path, mapping_path: Path, specs_path: Path, template_path: Path) -> dict[str, Any]:
    data = load_json(source)
    mappings = load_json(mapping_path)
    specs_payload = load_json(specs_path)
    errors: list[str] = []
    warnings: list[str] = []

    required = {
        "database", "revisions", "sources", "engine_slots", "engine_types",
        "modules", "controls", "screenshots", "sample_banks",
        "sample_library_entries", "sample_slots", "observed_control_states",
        "plugin_releases", "deprecated_control_ids", "deprecated_module_ids",
        "pgtx_import",
    }
    missing_top = sorted(required - set(data))
    if missing_top:
        errors.append(f"missing top-level keys: {missing_top}")

    revision_ids = ids(data.get("revisions", []), "revision_id", "revisions", errors)
    source_ids = ids(data.get("sources", []), "source_id", "sources", errors)
    engine_slot_ids = ids(data.get("engine_slots", []), "slot_id", "engine_slots", errors)
    engine_type_ids = ids(data.get("engine_types", []), "engine_type_id", "engine_types", errors)
    module_ids = ids(data.get("modules", []), "module_id", "modules", errors)
    control_ids = ids(data.get("controls", []), "control_id", "controls", errors)
    screenshot_ids = ids(data.get("screenshots", []), "screenshot_id", "screenshots", errors)
    bank_ids = ids(data.get("sample_banks", []), "bank_id", "sample_banks", errors)
    sample_entry_ids = ids(data.get("sample_library_entries", []), "sample_entry_id", "sample_library_entries", errors)
    sample_slot_ids = ids(data.get("sample_slots", []), "slot_id", "sample_slots", errors)

    pgtx = data.get("pgtx_import", {})
    archive_ids = ids(pgtx.get("archives", []), "archive_id", "pgtx_import.archives", errors)
    preset_ids = ids(pgtx.get("presets", []), "preset_id", "pgtx_import.presets", errors)
    internal_rows = pgtx.get("internal_parameters", [])
    internal_ids = ids(internal_rows, "internal_parameter_id", "pgtx_import.internal_parameters", errors)

    deferred_revision_source_refs: list[str] = []
    for row in data.get("revisions", []):
        source_ref = row.get("source_id")
        if source_ref not in source_ids and source_ref == data.get("database", {}).get("data_revision"):
            deferred_revision_source_refs.append(str(source_ref))
            continue
        require_ref(source_ref, source_ids, f"revision {row.get('revision_id')}.source_id", errors)
    for row in data.get("modules", []):
        require_ref(row.get("engine_type_id"), engine_type_ids, f"module {row.get('module_id')}.engine_type_id", errors, optional=True)
        require_ref(row.get("parent_module_id"), module_ids, f"module {row.get('module_id')}.parent_module_id", errors, optional=True)
    for row in data.get("controls", []):
        cid = row.get("control_id")
        require_ref(row.get("module_id"), module_ids, f"control {cid}.module_id", errors)
        require_ref(row.get("source_id"), source_ids, f"control {cid}.source_id", errors)
        for index, evidence in enumerate(row.get("evidence") or []):
            require_ref(evidence.get("source_id"), source_ids, f"control {cid}.evidence[{index}].source_id", errors)
    for row in data.get("screenshots", []):
        sid = row.get("screenshot_id")
        require_ref(row.get("source_id"), source_ids, f"screenshot {sid}.source_id", errors)
        require_ref(row.get("engine_type_id"), engine_type_ids, f"screenshot {sid}.engine_type_id", errors)
        require_ref(row.get("selected_engine_slot"), engine_slot_ids, f"screenshot {sid}.selected_engine_slot", errors)
    for row in data.get("sample_banks", []):
        require_ref(row.get("source_id"), source_ids, f"sample bank {row.get('bank_id')}.source_id", errors)
    for row in data.get("sample_library_entries", []):
        eid = row.get("sample_entry_id")
        require_ref(row.get("bank_id"), bank_ids, f"sample entry {eid}.bank_id", errors)
        require_ref(row.get("source_id"), source_ids, f"sample entry {eid}.source_id", errors)
    for row in data.get("sample_slots", []):
        require_ref(row.get("source_id"), source_ids, f"sample slot {row.get('slot_id')}.source_id", errors)
    for index, row in enumerate(data.get("observed_control_states", [])):
        require_ref(row.get("control_id"), control_ids, f"observed_control_states[{index}].control_id", errors)
        require_ref(row.get("source_id"), source_ids, f"observed_control_states[{index}].source_id", errors)
    for row in data.get("plugin_releases", []):
        require_ref(row.get("source_id"), source_ids, f"plugin release {row.get('version')}.source_id", errors)
    for row in data.get("deprecated_control_ids", []):
        require_ref(row.get("replacement_control_id"), control_ids, f"deprecated control {row.get('old_control_id')}.replacement", errors)
    for row in data.get("deprecated_module_ids", []):
        require_ref(row.get("replacement_module_id"), module_ids, f"deprecated module {row.get('old_module_id')}.replacement", errors)
    for row in pgtx.get("archive_entries", []):
        require_ref(row.get("archive_id"), archive_ids, f"archive entry {row.get('entry_path')}.archive_id", errors)
    for row in pgtx.get("presets", []):
        require_ref(row.get("archive_id"), archive_ids, f"preset {row.get('preset_id')}.archive_id", errors)
    for row in internal_rows:
        require_ref(row.get("mapped_control_id"), control_ids, f"internal parameter {row.get('internal_parameter_id')}.mapped_control_id", errors, optional=True)

    mapping_rows = mappings.get("mappings", [])
    mapping_control_ids = ids(mapping_rows, "control_id", "mapping overlay", errors)
    missing_mapping_controls = sorted(control_ids - mapping_control_ids)
    extra_mapping_controls = sorted(mapping_control_ids - control_ids)
    if missing_mapping_controls:
        errors.append(f"mapping overlay misses controls: {missing_mapping_controls[:20]}")
    if extra_mapping_controls:
        errors.append(f"mapping overlay contains unknown controls: {extra_mapping_controls[:20]}")

    mapped_targets: set[str] = set()
    automatic_control_count = 0
    for row in mapping_rows:
        if row.get("automatic_edit"):
            automatic_control_count += 1
        for target in row.get("targets") or []:
            parameter_id = target.get("parameter_id")
            if parameter_id not in internal_ids:
                errors.append(f"mapping {row.get('control_id')} targets missing internal ID {parameter_id!r}")
            elif isinstance(parameter_id, str):
                mapped_targets.add(parameter_id)

    specs = specs_payload.get("specs", [])
    spec_ids = ids(specs, "id", "compiler specs", errors)
    unknown_specs = sorted(spec_ids - internal_ids)
    if unknown_specs:
        errors.append(f"compiler specs reference unobserved internal IDs: {unknown_specs[:20]}")

    try:
        template_ids = extract_template_parameter_ids(template_path)
    except Exception as exc:  # pragma: no cover - fatal audit path
        errors.append(f"could not parse default template: {exc}")
        template_ids = set()
    missing_template_specs = sorted(spec_ids - template_ids)
    if missing_template_specs:
        errors.append(f"compiler specs missing from default template: {missing_template_specs[:20]}")
    missing_template_targets = sorted(mapped_targets - template_ids)
    if missing_template_targets:
        errors.append(f"mapping targets missing from default template: {missing_template_targets[:20]}")

    controls = data.get("controls", [])
    continuous_types = {
        "continuous_knob", "continuous_or_stepped_knob", "numeric_knob",
        "numeric_field", "continuous_labeled_knob", "continuous_crossfade_knob",
        "dual_unit_knob",
    }
    selector_types = {"enum_selector", "cyclic_enum_selector", "source_selector"}
    controls_with_range = 0
    continuous_missing_range: list[str] = []
    selectors_missing_options: list[str] = []
    defaults_missing: list[str] = []
    for row in controls:
        has_range = row.get("min_value") is not None and row.get("max_value") is not None
        if not has_range:
            has_range = any(v.get("min_value") is not None and v.get("max_value") is not None for v in row.get("variants") or [])
        if has_range:
            controls_with_range += 1
        if row.get("control_type") in continuous_types and not has_range:
            continuous_missing_range.append(row["control_id"])
        if row.get("control_type") in selector_types and not (row.get("options") or []):
            selectors_missing_options.append(row["control_id"])
        if row.get("default_value") is None:
            defaults_missing.append(row["control_id"])

    summary = pgtx.get("summary", {})
    expected_checks = {
        "unique_internal_parameter_count": len(internal_ids),
        "preset_count": len(preset_ids),
        "archive_count": len(archive_ids),
    }
    for key, actual in expected_checks.items():
        if summary.get(key) != actual:
            errors.append(f"pgtx_import.summary.{key}={summary.get(key)!r}, calculated={actual}")

    db = data.get("database", {})
    if db.get("schema_version") == "1.5.0" and "v1_6" in source.name:
        warnings.append("The filename carries the database release label v1_6 while the internal schema_version is 1.5.0; these are treated as different version axes, not silently reconciled.")
    if all(row.get("default_value") is None for row in controls):
        warnings.append("All documented UI controls currently have default_value=null; defaults must not be inferred from arbitrary preset observations.")
    if not any(row.get("mapped_control_id") for row in internal_rows):
        warnings.append("The source database keeps raw internal IDs unmapped; the versioned mapping overlay is therefore required at runtime.")
    if deferred_revision_source_refs:
        warnings.append("Revision pgtx_import_001 uses its import revision ID as source_id, but that ID is not represented in the sources array. The runtime importer treats this as provenance metadata and does not modify the source database.")

    result = {
        "source": str(source),
        "source_sha256": sha256(source),
        "database": {
            "name": db.get("name"),
            "schema_version": db.get("schema_version"),
            "data_revision": db.get("data_revision"),
            "last_updated": db.get("last_updated"),
            "plugin_build": db.get("pgtx_observed_plugin_build"),
        },
        "counts": {
            "revisions": len(revision_ids),
            "sources": len(source_ids),
            "engine_slots": len(engine_slot_ids),
            "engine_types": len(engine_type_ids),
            "modules": len(module_ids),
            "documented_controls": len(control_ids),
            "screenshots": len(screenshot_ids),
            "sample_banks": len(bank_ids),
            "sample_entries": len(sample_entry_ids),
            "sample_slots": len(sample_slot_ids),
            "observed_control_states": len(data.get("observed_control_states", [])),
            "pgtx_archives": len(archive_ids),
            "pgtx_presets": len(preset_ids),
            "internal_parameters": len(internal_ids),
            "preset_value_rows": summary.get("total_preset_parameter_value_rows"),
            "mapping_rows": len(mapping_rows),
            "mapped_target_ids": len(mapped_targets),
            "automatic_edit_controls": automatic_control_count,
            "compiler_specs": len(spec_ids),
            "template_parameters": len(template_ids),
            "controls_with_numeric_range": controls_with_range,
            "continuous_controls_missing_ranges": len(continuous_missing_range),
            "selectors_missing_options": len(selectors_missing_options),
            "controls_missing_defaults": len(defaults_missing),
        },
        "gaps": {
            "continuous_controls_missing_ranges": continuous_missing_range,
            "selectors_missing_options": selectors_missing_options,
            "controls_missing_defaults": defaults_missing,
        },
        "mapping_status_counts": dict(sorted(Counter(row.get("mapping_status") for row in mapping_rows).items())),
        "mapping_confidence_counts": dict(sorted(Counter(row.get("confidence") for row in mapping_rows).items())),
        "errors": errors,
        "warnings": warnings,
        "passed": not errors,
    }
    return result


def render_markdown(result: dict[str, Any]) -> str:
    c = result["counts"]
    d = result["database"]
    lines = [
        "# Pigments 7 master database audit",
        "",
        f"- Result: **{'PASS' if result['passed'] else 'FAIL'}**",
        f"- Source SHA-256: `{result['source_sha256']}`",
        f"- Database name: `{d.get('name')}`",
        f"- Internal schema version: `{d.get('schema_version')}`",
        f"- Data revision: `{d.get('data_revision')}`",
        f"- Last updated: `{d.get('last_updated')}`",
        f"- Observed Pigments build: `{d.get('plugin_build')}`",
        "",
        "## Verified inventory",
        "",
        f"- {c['documented_controls']} documented UI controls in {c['modules']} modules",
        f"- {c['internal_parameters']} exact serialized parameter IDs",
        f"- {c['preset_value_rows']} preset-value rows from {c['pgtx_presets']} presets in {c['pgtx_archives']} archives",
        f"- {c['mapping_rows']} confidence-rated UI mapping records",
        f"- {c['mapped_target_ids']} unique mapped internal targets",
        f"- {c['automatic_edit_controls']} controls allowed for conservative automatic editing",
        f"- {c['compiler_specs']} generated compiler specs",
        f"- {c['template_parameters']} parameters in the embedded default template",
        "",
        "## Calibration coverage",
        "",
        f"- Controls with a documented numeric range: **{c['controls_with_numeric_range']}**",
        f"- Continuous controls still missing ranges: **{c['continuous_controls_missing_ranges']}**",
        f"- Selectors still missing option lists: **{c['selectors_missing_options']}**",
        f"- Controls with no documented default: **{c['controls_missing_defaults']}**",
        "",
        "## Integrity checks",
        "",
        "- Unique identifiers and cross-references",
        "- Mapping overlay coverage for every documented control",
        "- Every mapping target exists in the imported internal inventory",
        "- Every generated compiler spec exists in the imported inventory",
        "- Every mapped target and compiler spec exists in the embedded default template",
        "- PGTX summary counts agree with calculated archive, preset, and parameter counts",
        "",
        "## Mapping-status counts",
        "",
    ]
    lines.extend(f"- `{key}`: {value}" for key, value in result["mapping_status_counts"].items())
    lines.extend(["", "## Remaining calibration targets", ""])
    for label, items in result["gaps"].items():
        lines.append(f"### {label.replace('_', ' ').title()}")
        lines.append("")
        if not items:
            lines.append("None.")
        elif label == "controls_missing_defaults":
            lines.append(f"All **{len(items)}** documented controls currently omit defaults. Defaults must be measured or explicitly defined; they are not inferred from arbitrary presets.")
        else:
            lines.extend(f"- `{item}`" for item in items)
        lines.append("")
    if result["warnings"]:
        lines.extend(["## Warnings", ""])
        lines.extend(f"- {warning}" for warning in result["warnings"])
        lines.append("")
    if result["errors"]:
        lines.extend(["## Errors", ""])
        lines.extend(f"- {error}" for error in result["errors"])
        lines.append("")
    lines.extend([
        "## Interpretation",
        "",
        "A passing audit verifies structural consistency and compiler coverage. It does not prove that every UI-to-serialized conversion curve is exact or that every generated sound behaves as intended inside Pigments. Controlled before/after preset pairs and real Pigments import/audition tests remain the authority for those questions.",
        "",
    ])
    return "\n".join(lines)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", type=Path, default=DEFAULT_SOURCE)
    parser.add_argument("--mapping", type=Path, default=DEFAULT_MAPPING)
    parser.add_argument("--specs", type=Path, default=DEFAULT_SPECS)
    parser.add_argument("--template", type=Path, default=DEFAULT_TEMPLATE)
    parser.add_argument("--report", type=Path, default=DEFAULT_REPORT)
    parser.add_argument("--json", action="store_true", help="print full JSON audit result")
    args = parser.parse_args()

    result = audit(*(path.resolve() for path in (args.source, args.mapping, args.specs, args.template)))
    args.report.parent.mkdir(parents=True, exist_ok=True)
    args.report.write_text(render_markdown(result), encoding="utf-8")
    print(json.dumps(result if args.json else {"passed": result["passed"], "counts": result["counts"], "errors": result["errors"], "warnings": result["warnings"]}, indent=2))
    if not result["passed"]:
        raise SystemExit(1)


if __name__ == "__main__":
    main()
