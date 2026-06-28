# Admin Authentication Spec

## Purpose

Gate the admin surface behind a single shared password. The admin panel is always on
(`SUBGEN_ADMIN_PASSWORD` is mandatory). A successful login mints a stateless, HMAC-signed
session cookie (`subgen_admin`); the cookie alone authorizes the JSON admin API and the SPA
pages — there is no server-side session store. Browser pages redirect when unauthenticated;
the JSON API answers `401`.

## Requirements

### Requirement: Session cookie format and lifetime

The session cookie `subgen_admin` SHALL be a stateless `<expiry-unix>.<hmac-sha256-hex>` value
signed with the configured secret, valid for 12 hours, and set with `HttpOnly`, `Secure`, and
`SameSite=Lax`.

#### Scenario: Cookie issued on login

- **WHEN** a login succeeds
- **THEN** a `subgen_admin` cookie is set whose value is `<expiry>.<hmac>` over the expiry
- **AND** the cookie is marked `HttpOnly`, `Secure`, `SameSite=Lax`, with a 12-hour max age

#### Scenario: Expired cookie is not valid

- **WHEN** a request presents a `subgen_admin` cookie whose expiry is in the past
- **THEN** the cookie is treated as invalid (the session is not authorized)

#### Scenario: Tampered cookie is rejected

- **WHEN** a request presents a `subgen_admin` cookie whose HMAC does not match its expiry
- **THEN** the cookie is treated as invalid (constant-time comparison, no partial trust)

### Requirement: Login

`POST /admin/api/login` SHALL accept a username and password, and MUST issue a session cookie
only when both match the configured admin credentials, comparing them in constant time.

#### Scenario: Valid credentials

- **WHEN** `POST /admin/api/login` is called with the correct username and password
- **THEN** the response is `200` with `{ "message": ... }`
- **AND** a valid `subgen_admin` session cookie is set

#### Scenario: Invalid credentials

- **WHEN** `POST /admin/api/login` is called with a wrong username or password
- **THEN** the response is `401` with `{ "errMessage": "Invalid username or password" }`
- **AND** no session cookie is set
- **AND** the comparison does not reveal whether the username or the password was wrong

### Requirement: Logout

`POST /admin/api/logout` SHALL clear the session cookie and always succeed.

#### Scenario: Logout clears the session

- **WHEN** `POST /admin/api/logout` is called
- **THEN** the response is `204` with no body
- **AND** the `subgen_admin` cookie is expired (cleared) on the client

### Requirement: Admin JSON API gate

Every `/admin/api/*` endpoint except login SHALL require a valid session cookie, returning
`401` when it is missing or invalid.

#### Scenario: Missing session on a protected endpoint

- **WHEN** an `/admin/api/*` endpoint (other than login) is called without a valid session cookie
- **THEN** the response is `401` with `{ "errMessage": "Authorization required" }`

#### Scenario: Valid session passes the gate

- **WHEN** a protected `/admin/api/*` endpoint is called with a valid session cookie
- **THEN** the request reaches its handler and is served normally

### Requirement: SPA page gating and redirects

The admin browser pages SHALL redirect based on session state instead of returning `401`:
unauthenticated visits to the app are sent to the login page, and an authenticated visit to the
login page is sent to the app.

#### Scenario: Unauthenticated app shell

- **WHEN** `GET /admin` or `GET /admin/{view}` is requested without a valid session
- **THEN** the response is `302` to `/admin/login`

#### Scenario: Authenticated app shell

- **WHEN** `GET /admin` or `GET /admin/{view}` is requested with a valid session
- **THEN** the response is `200` serving the SPA shell HTML

#### Scenario: Login page while already signed in

- **WHEN** `GET /admin/login` is requested with a valid session
- **THEN** the response is `302` to the app

#### Scenario: Login page while signed out

- **WHEN** `GET /admin/login` is requested without a valid session
- **THEN** the response is `200` serving the login page HTML

### Requirement: Central request-decoding errors

Requests that fail security or body/parameter decoding before reaching a handler SHALL be
answered by the central error handler: security failures as `401` and malformed
requests as `400`, without leaking internal detail.

#### Scenario: Malformed request body

- **WHEN** a request body or parameter cannot be decoded by the generated router
- **THEN** the response is `400` with `{ "errMessage": "Bad request" }`

#### Scenario: Security failure logged once

- **WHEN** a security check fails before the handler runs
- **THEN** the central error handler returns `401` and logs the event at warn level
- **AND** it does not re-log errors that handlers already logged with their own context
