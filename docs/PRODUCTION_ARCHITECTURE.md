# Production architecture

## Recommended hosted flow

```text
Browser / mobile browser
        |
        v
CDN + WAF + TLS
        |
        v
Web application / API
  - authentication
  - plan and credit checks
  - prompt validation
  - job ownership
        |
        +---------------------> PostgreSQL
        |                       users, jobs, usage, audit, consent
        |
        +---------------------> Private object storage
        |                       uploads, generated presets, reports
        |
        v
Job queue
        |
        v
Isolated preset worker
  1. validate upload/archive
  2. inspect current preset
  3. build compact relevant parameter context
  4. call OpenAI Responses API
  5. validate strict structured plan
  6. compile preset deterministically
  7. reopen and validate output
  8. scan output and store privately
        |
        v
Short-lived signed download URL
```

## Why the language model does not write `.pgtx` directly

The model is responsible for musical interpretation: translating “dark,” “fuzzy,” “wide,” “slow attack,” or “preserve the identity” into a small structured plan. The application owns file construction. This creates enforceable boundaries:

- known parameter IDs only;
- known units and value ranges;
- no arbitrary archive entries;
- predictable non-destructive modification;
- reproducible reports;
- easier compatibility testing and rollback.

## Services needed after the MVP

### Identity and billing

- email or social sign-in;
- verified email;
- terms/privacy acceptance with version history;
- plans or generation credits;
- per-user quotas;
- payment provider webhook verification;
- refund and dispute controls.

### Data layer

- PostgreSQL for users, jobs, prompt metadata, selected model, compiler version, template version, knowledge-base version, mapping-overlay version, and deletion status;
- private S3-compatible object storage;
- short-lived signed upload/download URLs;
- lifecycle policies matching the published retention period;
- encryption at rest and in transit.

### Pigments knowledge service

- Store every accepted master-database revision immutably with a checksum.
- Run the import and audit pipeline in CI before promoting a revision.
- Keep documented UI controls, raw observed internal IDs, mapping confidence, and compiler-safe specifications as separate layers.
- Publish only bounded, non-sensitive search results to browsers.
- Give workers a signed compiler allowlist rather than direct write access to every observed ID.
- Record the source-database checksum, mapping-overlay checksum, generated-spec checksum, and calibration status with every job.
- Require controlled before/after preset evidence before promoting enum orders, conditional mappings, or nonlinear conversion curves.

### Queue and workers

- asynchronous queue for generation jobs;
- isolated workers with CPU, memory, time, and archive-size limits;
- idempotency key per generation;
- retry only safe/transient failures;
- dead-letter queue for malformed or repeatedly failing jobs.

### AI service

- OpenAI Responses API called from workers only;
- Structured Outputs with the included JSON schema;
- `store: false` unless a reviewed product requirement says otherwise;
- model choice controlled server-side;
- request ID and token usage recorded without logging customer preset bytes;
- prompt-injection resistance by treating preset metadata and user text as untrusted data;
- evaluation set for prompt-to-plan quality.

### Security and abuse

- WAF, request-size limits, managed rate limiting, and bot protection;
- archive and malware scanning;
- content-rights confirmation for modifications;
- no public sharing of modified uploads by default;
- human support path for accidental proprietary-content uploads;
- admin access protected with MFA and audited roles.

### Observability

- structured logs with secrets and preset content redacted;
- latency, API error, validation failure, import success, and support-ticket metrics;
- OpenAI request IDs for support;
- alerting on cost spikes, abuse spikes, and elevated invalid-preset rates.

## Suggested deployment stages

### Stage 0 — internal research

- run mock mode and a private API key;
- collect parameter pairs and screenshots;
- import every generated file into the exact target Pigments version;
- maintain an allowlist of proven controls.

### Stage 1 — closed alpha

- invite-only accounts;
- generate-new-preset flow first;
- no public marketplace;
- manual review of failures;
- short retention;
- explicit compatibility label.

### Stage 2 — private preset modification

- rights confirmation;
- files returned only to uploader;
- embedded-sample detection and policy;
- complete audit trail and deletion controls.

### Stage 3 — paid beta

- billing, credits, usage dashboard, support process, stronger monitoring;
- multiple Pigments-version templates only after separate compatibility suites pass;
- signed release artifacts and rollback support.

### Stage 4 — optional local companion

A separately signed local application may later assist with installing presets or MIDI control. It should be developed only after technical and legal review with Arturia. The public website itself should not attempt to write directly into a user's desktop application.

## Versioning fields to store with every job

- application version;
- compiler version;
- planner prompt version;
- JSON schema version;
- Pigments target version;
- base-template checksum;
- master-database checksum;
- mapping-overlay checksum;
- compiler-spec checksum;
- parameter-catalog checksum;
- model and model snapshot/alias;
- input preset checksum for modifications;
- output preset checksum;
- validation result;
- creation and deletion timestamps.
