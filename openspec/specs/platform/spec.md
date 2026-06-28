# Platform Spec

## Purpose

The non-feature HTTP surface that supports operations and the admin UI: a liveness probe and the
static asset handler. The ogen server is the only typed `http.Handler`; the static route is the
single exception, mounted alongside it on the stdlib mux. Static assets are served from the
embedded copy by default, or live from disk for local development.

## Requirements

### Requirement: Liveness probe

`GET /healthz` SHALL return `200` with a plain-text body and require no authentication.

#### Scenario: Health check

- **WHEN** `GET /healthz` is called
- **THEN** the response is `200` with a plain-text `ok` body

### Requirement: Static asset serving

`GET /admin/static/*` SHALL serve the admin UI assets, from the embedded copy by default and from
disk when a static directory override is configured.

#### Scenario: Default embedded assets

- **WHEN** no static-directory override is set
- **THEN** assets are served from the embedded copy (a self-contained image)

#### Scenario: Live-from-disk override

- **WHEN** a static-directory override is configured
- **THEN** assets are served live from that directory, so CSS/JS edits show on reload without rebuilding
