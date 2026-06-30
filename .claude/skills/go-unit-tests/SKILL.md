---
name: go-unit-tests
description: >-
  Use this skill whenever unit tests are involved in this Go backend — writing, adding,
  generating, fixing, extending, or reviewing them — for a handler, service, client, or
  any exported method. Trigger it when the user asks for a test or a `*_test.go` file,
  wants more coverage for some function, needs a missing or failing unit test made to
  pass, adds a case to a table-driven test, or needs a `contract.go` / mock created or
  regenerated for a test — even if they never say "table-driven", "testify", "gomock",
  or name this skill. Also trigger on mentions of gomock, mockgen, contract.go, or Mock*
  types, and when refactoring existing Go tests toward the repo's convention. Reach for
  it by default so every test in the codebase stays consistent rather than ad hoc.
---

# Go Unit Tests

This skill captures one specific, opinionated way of writing Go unit tests. The goal
is that every test you write looks like it was written by the same person.
Consistency here matters more than cleverness — a reviewer
should be able to read any test in the codebase without re-learning the layout.

## A note on scope

These rules are for **unit** tests — pure logic with mocked dependencies — and apply in full
only to them. Other kinds of tests (integration / API tests / e2e / etc.) follow their own conventions; don't force this 
template onto them.

## The shape of every test

One test function per **exported method**, named `Test{Type}_{Method}`. The table holds
the cases for **that one method** — it is not a place to dump cases for several methods.
Each method gets its own `Test{Type}_{Method}` with its own table. And do **not** split a
single method's cases the other way either, into `TestX_Success` / `TestX_Error` — all
cases for a method live together in its one table. Private helpers are never tested
directly; exercise them through the exported method that uses them.

**When NOT to use a table.** The table earns its keep when a method has several distinct
cases worth enumerating. If a method is trivial or has effectively one case — say a
`CopyMap` that just copies a map, or a pure getter — a table is just ceremony. Write a
single straight-line test that sets up, calls the method, and asserts the behavior
directly. Reach for the table when there's real branching to cover; don't force one onto a
one-case method.

The canonical template for the common (multi-case) shape — this is the target, match it:

```go
func TestClient_Method(t *testing.T) {
	targetErr := errors.New("test")

	tt := []struct {
		name string
		// input fields for this method go here

		// one build function per dependency, named after the dependency.
		// Any of them may be left nil in a case where that dep isn't touched.
		buildClientMock func(m *MockClient)
		buildRepoMock   func(m *MockusersRepo)

		result  SomeOut
		err     error
		wantErr bool // only when err is NOT a sentinel (see "Checking errors")
	}{
		{name: "empty"},
		{
			name:            "success.one_user",
			buildClientMock: func(m *MockClient) { m.EXPECT().X(gomock.Any()).Return(out, nil) },
			buildRepoMock:   func(m *MockusersRepo) { m.EXPECT().GetByID(gomock.Any(), int64(7)).Return(u, nil) },
			result:          SomeOut{ /* expected mapping */ },
		},
		{
			name:            "error.downstream",
			buildClientMock: func(m *MockClient) { m.EXPECT().X(gomock.Any()).Return(nil, targetErr) },
			err:             targetErr,
		},
	}

	t.Parallel()
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			client := NewMockClient(ctrl)
			if tc.buildClientMock != nil {
				tc.buildClientMock(client)
			}

			repo := NewMockusersRepo(ctrl)
			if tc.buildRepoMock != nil {
				tc.buildRepoMock(repo)
			}

			s := New(client, repo)
			result, err := s.Method(context.Background() /*, tc.input */)

			require.ErrorIs(t, err, tc.err)
			assert.Equal(t, tc.result, result)
		})
	}
}
```

Why this shape: the table is the contract of the method made visible — a reader scans
`name` + `buildMock` + `result`/`err` and sees every branch at once. Per-method (not
per-outcome) functions keep that whole picture in one place.

For a trivial method, the straight-line form is the right call — no table:

```go
func TestCopyMap(t *testing.T) {
	src := map[string]int{"a": 1, "b": 2}

	got := CopyMap(src)

	assert.Equal(t, src, got)        // same contents
	got["a"] = 99
	assert.Equal(t, 1, src["a"])     // and it's a real copy, not an alias
}
```

## Rules

**Case names use dots, not spaces.** `success.one_user`, `error.invalid_user_id`,
`empty_fields`. They become subtest names (`go test -run`), so spaces are awkward and a
flat dotted namespace reads well in `-v` output.

**Always cover, at minimum:** an `empty` case (when a zero/empty input is meaningful), a
`success` case that asserts the actual mapping/output (not just "no error"), and an
`error.*` case driven by a failing dependency. Add domain edge cases as the method
warrants — don't pad with cases that don't test a distinct branch.

**Parallelism: `t.Parallel()` both on the table and inside `t.Run`.** The one exception
is when dependency codegen isn't goroutine-safe (a data race in the generated `New()` or
declarations) — then drop the parallel construction and say why in a short comment.

**Mocks come from the package's own `contract.go` via mockgen.** Each package declares
its dependencies as a **private** interface in `contract.go` describing exactly the methods
it needs (interface segregation). **Name the interface after the concrete dependency it
points at, not after an abstract role.** A repository is `<entity>Repo` (`usersRepo`,
`nodesRepo`, `configsRepo`, `routingRepo`); a service is `<entity>Service` (`fleetService`,
`provisioningService`, `nodesService`); a client is `<entity>Client` (`panelClient`).
Role names that hide whether the thing is a repo, a service or a client — `deleter`,
`creator`, `subLinker`, `configResolver`, `mihomoReader` — are the wrong call: someone
reading the test then has to open `contract.go` to learn that `deleter` is "the provisioning
service." The name should answer *what is this dependency* on sight, so the test reads on its
own. Keep these interfaces unexported by default — they exist only for this package to depend
on and to generate mocks from, so there's no reason to export them. The only reason to export
one is a DI container or codegen tool that needs to import the interface (dig, wire, etc.);
absent that, lowercase it. The generated `Mock*` types live next to it in `contract_mocks.go`
(mockgen names them `Mock<InterfaceName>`, so a private `nodesRepo` interface gives
`MocknodesRepo` / `NewMocknodesRepo`).
In a test, use only the **local** `Mock*` from that package — never pull another package's
`clients.MockClient` into a service test. Each layer mocks its own contract.

If `contract.go` doesn't exist yet for the dependencies you need to mock, create it with
the generation directive and run `go generate`:

```go
// contract.go
//go:generate go tool mockgen -source=contract.go -destination contract_mocks.go -package $GOPACKAGE
package mypkg

import "context"

type usersRepo interface {
	GetByID(ctx context.Context, id int64) (entity.User, error)
}
```

Then `go generate ./...` (mockgen is wired as a `go tool`).

## Checking errors and results

- Errors: `require.ErrorIs(t, err, tc.err)` against a **sentinel** error (the domain
  defines `var ErrFoo = errors.New(...)`). Use `require.ErrorAs` for typed errors and
  `require.NoError` when no error is expected. `require` (not `assert`) for errors so the
  test stops before dereferencing a bad result.
- Use `wantErr bool` / `require.ErrorContains` **only** when there's no concrete sentinel
  to match — prefer matching a sentinel whenever one exists.
- Results: `assert.Equal(t, tc.result, result)` — assert the whole expected value so the
  mapping is verified, not just that something non-nil came back.

## Readability and test helpers

- **Group the table struct fields with blank lines**: name on its own, then the
  input/expected-output fields (`in` / `expectedOut` read well), then the `buildXxxMock`
  funcs. The visual grouping makes a long table scannable.
- **Use a `Must` helper for setup that returns `(T, error)`.** Many repos keep a generic
  `func Must[T any](v T, err error) T` in an internal `utils/testing` package to unwrap
  fixture construction without an `if err != nil` in every test. If one exists, use it
  (`hash := testingUtils.Must(hashOf(data))`); building the *fixtures* shouldn't be where a
  test's error handling lives — that belongs in the assertions on the method under test.
- **Assert concrete expected arguments in `EXPECT` when the arguments are the point.** If
  the test is partly verifying that the method maps its input into the right call, spell out
  the full expected struct/slice in `m.EXPECT().Save(gomock.Any(), wantArg).Return(...)`
  rather than `gomock.Any()` — that's what catches a broken mapping. Use `gomock.Any()` only
  for arguments that genuinely aren't under test (typically `context.Context`).

## Setting up gomock expectations

Inside `buildMock`/`buildMocks`, set expectations with `m.EXPECT()`:

```go
m.EXPECT().DeleteUser(gomock.Any(), int64(7)).Return(nil)
```

Match concrete arguments when the value is part of what you're testing (e.g. that the
right id is passed through); use `gomock.Any()` for arguments that aren't the point of the
case (like a `context.Context`). **One build function per dependency**, named after the dependency it sets up
(`buildClientMock`, `buildRepoMock`), each taking only its own `Mock*`. Don't collapse
several deps into one combined `buildMocks(...)` — separate functions keep each case
readable and let a case leave a dependency's function `nil` when that case never touches
it (an untouched mock simply has no expectations set, so gomock won't fail on it).

## HTTP / external-API clients

Clients in `internal/clients/<dep>/` are thin adapters — one method per file
(`add_client.go`), one test file per method. Don't hit the real network: mock HTTP with
`httptest.Server` or an `http.RoundTripper` mock, feed it the dependency's real wire
response (with its quirks), and assert the client maps it to the right `entity.*` value.
The test verifies the DTO→domain mapping, which is the only logic a client should have.

## Workflow when asked to write tests

1. Read the code under test and its package's `contract.go` (if present) to learn the
   real dependency interfaces and the `entity.*` types involved.
2. Identify the exported methods that need tests and the sentinel errors they can return.
3. Write one `Test{Type}_{Method}` per method using the template above.
4. If mocks are missing, add/extend `contract.go` and run `go generate ./...`.
5. Run `go test ./...` (or the specific package) and make it green. Report the result —
   if a test reveals a real bug in the code, surface it rather than bending the test to pass.
