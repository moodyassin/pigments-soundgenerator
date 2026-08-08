# Security policy

## Reporting

Do not publish suspected vulnerabilities with working exploitation details before maintainers have had a reasonable opportunity to investigate. Report them privately to the project owner with:

- affected version;
- reproduction steps;
- expected and actual behavior;
- impact assessment;
- relevant request IDs, logs, or malformed test files without third-party confidential content.

## Threat model covered by the MVP

The server implements defenses for the most immediate local/prototype risks:

- maximum HTTP request and preset upload sizes;
- `.pgtx` extension checks and ZIP signature validation;
- archive entry count, per-file, and total expanded-size limits;
- path traversal and unsafe archive-name rejection;
- no execution of uploaded content;
- random private temporary filenames;
- same-origin checks for state-changing browser requests;
- security response headers and a restrictive Content Security Policy;
- basic per-client request limiting;
- generated-file retention and cleanup;
- model output validation and parameter allowlisting;
- clamping or rejection of invalid parameter values;
- originals are never overwritten by the modify workflow.

## Required additions before an internet-facing launch

- authenticated accounts and authorization checks on every job;
- CSRF protection integrated with the authentication framework;
- managed rate limiting and abuse detection;
- malware scanning for uploaded archives and embedded audio;
- isolated worker processes or containers for archive parsing;
- cloud object storage with short-lived signed download URLs;
- database-backed ownership, audit, deletion, and retention records;
- secret management rather than environment files on disk;
- structured redacted logs and OpenAI request-ID tracking;
- dependency and container scanning in CI;
- backups, incident response, and deletion verification;
- penetration testing before handling paid customer files.

## API key handling

`OPENAI_API_KEY` must remain server-side. Never return it to the browser, commit it to Git, place it in a downloadable desktop bundle, or log it.

## Content ownership

Uploaded presets can contain proprietary samples or other embedded material. Do not use customer files for public training, examples, galleries, or cross-user retrieval. Require rights confirmation and delete uploads according to the published retention policy.
