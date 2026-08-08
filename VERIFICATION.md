# Verification report — v0.3.0

Date: 2026-08-08

## Automated checks completed

- `go test ./...`
- `go vet ./...`
- `go test -race ./...`
- JavaScript syntax extraction and `node --check`
- byte-for-byte comparison and SHA-256 confirmation that the imported source database copy matches the uploaded JSON
- reproducible master-database import with exact expected totals: 125 controls, 34 modules, 3,525 IDs, 15,975 value rows, and five presets
- mapping-overlay validation: all 125 controls have an explicit classification and every mapped target exists in the imported internal-ID inventory
- regression checks for 119 mapped controls, 79 automatic-edit controls, 260 unique mapped IDs, 152 generated compiler specs, and 245 planner-visible write-safe specs after calibration locks
- safety regression proving serialized sample/wavetable asset objects are not admitted to the compiler catalog
- safety regression proving the uncalibrated Classic/Smooth Bit Crush selector is searchable but rejected by plan validation and omitted from the verified planner catalog
- safety regression proving uncalibrated bipolar/UI numeric controls use percent-of-knob-travel rather than unsafe raw UI ranges
- knowledge endpoint checks proving the full 3,525-ID list is not bulk-exposed to browsers
- generation endpoint creates a downloadable ZIP-based `.pgtx`
- generated output reopens through the project parser
- metadata override is written and can be re-inspected
- modification creates a separate output and does not overwrite the source
- modification preserves source metadata when rename was not requested
- modification report contains only the intended controlled change
- rights confirmation is required for uploaded-preset modification
- controlled preset diff identifies a single known parameter change
- wrong upload extension is rejected
- same-origin checks and security headers are applied
- OpenAI request tests confirm:
  - `/v1/responses` endpoint shape
  - selected model propagation
  - `store: false`
  - strict `text.format` JSON schema
  - missing API key failure
  - structured response parsing
- reverse bit-depth conversion is covered by unit tests
- archive parsing and preservation tests cover inner-path and additional-asset behavior
- clean-room extraction of the source ZIP followed by `go test`, `go vet`, and an independent build
- SHA-256 verification and ZIP integrity checks for every release package
- binary-format verification for macOS arm64, macOS x86_64, Windows x86_64, and Linux x86_64
- packaged Linux binary started independently and completed a full HTTP generate, download, reopen, and Sample Engine inspection flow

## Live local mock checks completed

A mock server was started on localhost and exercised through HTTP:

- `/api/status` returned ready mock-planner state;
- a Sample Engine Bit Crush prompt generated a `.pgtx`;
- inspection confirmed Sample engine selection, Shaper enable, Bit Crush selection, decimation, approximately 8-bit depth, and pitch-follow state;
- the download endpoint returned the generated file;
- a modification request raised Filter 1 cutoff from approximately 900 Hz to approximately 1100 Hz while preserving metadata;
- a controlled diff found exactly one changed serialized parameter;
- source and output SHA-256 hashes confirmed the source was not overwritten.

## Checks that remain outside this environment

- Real Responses API request using the owner's production API account.
- Import of generated presets into the exact installed Pigments 7.0.1 build.
- Audible verification across macOS and Windows.
- Controlled calibration of candidate enum values, conditional multi-parameter controls, and approximate display-unit curves.
- Confirmation of the 119 candidate mappings against purpose-built one-control before/after presets; only 79 are currently available for conservative automatic editing.
- Complete factory/user sample-browser inventory for the installed content version.
- Legal clearance of the development template and commercial compatibility workflow.
- External penetration testing and hosted-infrastructure testing.
- Docker image build was not executed because Docker is not installed in the build environment; the Dockerfile remains source-reviewed only.

Passing archive tests does not by itself prove that every generated preset will be accepted or sound as intended in Pigments. Round-trip import and audition tests remain mandatory before public release.
