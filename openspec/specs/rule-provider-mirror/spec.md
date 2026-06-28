# Rule-Provider Mirror Spec

## Purpose

subgen can mirror upstream rule-provider files so clients fetch them from subgen instead of the
origin. A background service fetches each mirrored provider referenced by any config (base or
custom), refreshes it on an interval, and keeps the last good copy when a refresh fails. The
public endpoint `GET /rules/{file}` serves the cached bytes with content-sniffing disabled.

## Requirements

### Requirement: Serve mirrored files

`GET /rules/{file}` SHALL return a mirrored file's bytes with `X-Content-Type-Options: nosniff`,
and `404` when the file is not in the mirror.

#### Scenario: File present

- **WHEN** `GET /rules/{file}` requests a file held in the mirror
- **THEN** the response is `200` with the file bytes and `X-Content-Type-Options: nosniff`

#### Scenario: File absent

- **WHEN** the requested file is not in the mirror
- **THEN** the response is `404`

### Requirement: Background refresh keeps the last good copy

The mirror SHALL refresh each mirrored provider on its interval and MUST keep the last good copy
when a refresh fails or returns empty, so a transient upstream outage never empties the cache.

#### Scenario: Successful refresh

- **WHEN** a refresh fetches new content successfully
- **THEN** the cached copy is replaced with the new bytes

#### Scenario: Failed or empty refresh

- **WHEN** a refresh errors or returns an empty body
- **THEN** the previously cached copy is kept unchanged

### Requirement: Mirror set covers all configs

The mirror SHALL include the mirrored providers referenced by any config, base or custom.

#### Scenario: Provider from a custom config

- **WHEN** a custom config references a mirrored provider
- **THEN** that provider is included in the mirror set
