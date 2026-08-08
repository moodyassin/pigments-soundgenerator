# Changelog

## 0.3.0 — 2026-08-08

- Imported the user-maintained Pigments 7 master database as the canonical research source.
- Added a reproducible read-only database import pipeline.
- Indexed 125 documented UI controls and 3,525 exact serialized parameter IDs.
- Added a versioned confidence-rated UI-to-serialization mapping overlay.
- Mapped 119 UI controls to one or more candidate or verified targets, covering 260 unique serialized IDs.
- Enabled 79 controls for conservative automatic numeric editing.
- Generated 152 Engine 1/Engine 2 compiler parameter specifications and exposed 245 planner-visible write-safe specs after applying calibration locks.
- Added bounded server-side search across documented UI controls, compiler-safe parameters, exact internal IDs, and sample-browser records.
- Added master-database context retrieval to OpenAI planning prompts without sending the entire database.
- Added database summary metrics to the status and knowledge endpoints.
- Added a permanent audit script covering references, mappings, generated specs, default-template coverage, and PGTX counts.
- Added master-database import and audit reports.
- Added regression tests for database counts, knowledge-layer separation, exact-ID search, public payload size, generated-spec uniqueness, and default-template coverage.
- Preserved the source database unchanged; all platform-specific joins remain in a separate overlay.

## 0.2.0 — 2026-08-08

- Reframed the project as the Audio Prompters Phase-1 web MVP.
- Replaced local Codex planning with server-side OpenAI Responses API planning.
- Added strict JSON-schema Structured Outputs.
- Added browser workflows for generate, modify, inspect, Parameter Lab, and knowledge search.
- Added non-destructive arbitrary-preset modification and change reporting.
- Added controlled before/after parameter discovery.
- Added Sample Engine Tune, Bit Crush, sample-start, and A–F slot knowledge.
- Added deterministic mock mode.
- Added upload/archive safeguards, security headers, retention, and rate limits.
- Replaced the inherited user-bank image with an original Audio Prompters image.
- Added production, security, legal, parameter-research, and screenshot documentation.

## 0.1.1

- Corrected Codex CLI argument ordering in the earlier local proof of concept.

## 0.1.0

- Initial local Pigments/ChatGPT bridge proof of concept.
