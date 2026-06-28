# Tasks — adopt OpenSpec, retire ADRs

- [x] Scaffold OpenSpec (`openspec init --tools claude`): `config.yaml`, `specs/`, `changes/`.
- [x] Fill `openspec/config.yaml` with the project context and per-artifact rules.
- [x] Narrow the `.claude` gitignore to `.claude/settings.local.json` so `/opsx:*` is tracked.
- [x] Write the full spec baseline under `openspec/specs/` (12 capabilities); validate with
      `openspec validate --specs --strict`.
- [x] Rewrite the process in `AGENTS.md`, `CONTRIBUTING.md`, `README.md`, and the `CHANGELOG.md`
      header to point at OpenSpec.
- [x] Remove the `docs/decisions/` ADR catalog; drop or repoint inline ADR citations across the
      docs and code comments.
- [x] Record this migration as an archived OpenSpec change.
- [x] Add the `CHANGELOG.md` entry.
