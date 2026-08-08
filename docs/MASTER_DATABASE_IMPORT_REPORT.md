# Pigments 7 master database import report

- Source: `pigments7_master_database_v1_6.json`
- Source schema: `1.5.0`
- Source revision: `pgtx_import_001`
- Pigments build observed in presets: `7.0.1.6772`
- UI controls imported: **125**
- Internal parameter IDs indexed: **3525**
- UI controls with one or more mapping targets: **119**
- Controls permitted for conservative automatic numeric editing: **79**
- Unique mapped internal IDs: **260**
- Generated compiler specs: **152**

## Mapping policy

The import does not modify the master database. A separate overlay classifies each relationship as platform-verified, high-confidence by naming/structure, conditional, ambiguous, UI-only, or asset-object based. Candidate enums and conditional controls remain unavailable for automatic editing until a controlled before/after preset pair establishes the stored values.

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
