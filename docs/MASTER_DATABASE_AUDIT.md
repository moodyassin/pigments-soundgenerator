# Pigments 7 master database audit

- Result: **PASS**
- Source SHA-256: `76923e666c33afb2405232f2d3100ee539548a6c81fafd9c9f2ca1d7e2887045`
- Database name: `Pigments 7 Sound Design Master Database`
- Internal schema version: `1.5.0`
- Data revision: `pgtx_import_001`
- Last updated: `2026-08-08`
- Observed Pigments build: `7.0.1.6772`

## Verified inventory

- 125 documented UI controls in 34 modules
- 3525 exact serialized parameter IDs
- 15975 preset-value rows from 5 presets in 3 archives
- 125 confidence-rated UI mapping records
- 260 unique mapped internal targets
- 79 controls allowed for conservative automatic editing
- 152 generated compiler specs
- 3335 parameters in the embedded default template

## Calibration coverage

- Controls with a documented numeric range: **67**
- Continuous controls still missing ranges: **14**
- Selectors still missing option lists: **8**
- Controls with no documented default: **125**

## Integrity checks

- Unique identifiers and cross-references
- Mapping overlay coverage for every documented control
- Every mapping target exists in the imported internal inventory
- Every generated compiler spec exists in the imported inventory
- Every mapped target and compiler spec exists in the embedded default template
- PGTX summary counts agree with calculated archive, preset, and parameter counts

## Mapping-status counts

- `ambiguous_shared_parameter_candidate`: 2
- `candidate_requires_controlled_diff`: 1
- `conditional_multi_parameter_candidate`: 7
- `confirmed_inverse_semantics`: 1
- `display_only`: 1
- `high_confidence_enum_candidate`: 21
- `high_confidence_name_match`: 58
- `high_confidence_ratio_candidate`: 2
- `high_confidence_stepped_candidate`: 4
- `high_confidence_with_legacy_id`: 1
- `platform_verified`: 22
- `serialized_audio_object_mapping_required`: 1
- `serialized_object_mapping_required`: 1
- `ui_state_not_confirmed_in_preset`: 2
- `unmapped_no_wavetable_specific_id`: 1

## Remaining calibration targets

### Continuous Controls Missing Ranges

- `sample.tune.filter`
- `sample.sample_grain.volume`
- `sample.granular.limit`
- `sample.granular.scan`
- `sample.granular.density`
- `sample.granular.shape`
- `sample.granular.size`
- `sample.granular.random_start`
- `sample.granular.random_pitch`
- `sample.granular.random_density`
- `sample.granular.direction`
- `sample.granular.random_size`
- `sample.granular.stereo`
- `sample.granular.random_volume`

### Selectors Missing Options

- `sample.granular.density_mode`
- `sample.granular.shape_type`
- `sample.granular.size_mode`
- `sample.granular.random_start_mode`
- `sample.granular.random_pitch_mode`
- `sample.granular.random_density_mode`
- `sample.granular.random_size_mode`
- `sample.granular.stereo_mode`

### Controls Missing Defaults

All **125** documented controls currently omit defaults. Defaults must be measured or explicitly defined; they are not inferred from arbitrary presets.

## Warnings

- The filename carries the database release label v1_6 while the internal schema_version is 1.5.0; these are treated as different version axes, not silently reconciled.
- All documented UI controls currently have default_value=null; defaults must not be inferred from arbitrary preset observations.
- The source database keeps raw internal IDs unmapped; the versioned mapping overlay is therefore required at runtime.
- Revision pgtx_import_001 uses its import revision ID as source_id, but that ID is not represented in the sources array. The runtime importer treats this as provenance metadata and does not modify the source database.

## Interpretation

A passing audit verifies structural consistency and compiler coverage. It does not prove that every UI-to-serialized conversion curve is exact or that every generated sound behaves as intended inside Pigments. Controlled before/after preset pairs and real Pigments import/audition tests remain the authority for those questions.
