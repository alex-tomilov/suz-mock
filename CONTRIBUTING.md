# Contributing

Thanks for helping improve this local SUZ API v3 mock server.

## Ground Rules

- Keep the project unofficial and clearly detached from real SUZ services.
- Do not add real credentials, tokens, certificates, OMS IDs, GTINs, order IDs,
  production payloads, or copied proprietary examples.
- Prefer synthetic fixtures and locally generated IDs in docs and tests.
- Keep endpoint behavior deterministic unless a scenario option explicitly asks
  for dynamic behavior.

## Development

Use the project's git-flow branch model:

```text
main      = stable public releases only
develop   = next release integration branch
feature/* = focused implementation branches
release/* = final stabilization before tag
hotfix/*  = urgent fixes from main
```

Start regular work from `develop`:

```bash
git checkout develop
git pull origin develop
git flow feature start short-topic-name
```

Run the project locally:

```bash
go run ./cmd/suz-mock
```

Run checks before opening a pull request:

```bash
gofmt -w .
go test ./...
go vet ./...
```

## Pull Requests

Keep pull requests focused. Include:

- a short description of the behavior or documentation change;
- tests or a clear reason tests are not needed;
- notes about any new environment variables or mock scenarios;
- confirmation that examples and fixtures are synthetic.

Open feature pull requests against `develop`. Use `main` only for release merges
and urgent hotfixes.

## Commit Messages

Use concise conventional-style commit messages:

```text
<type>(optional-scope): imperative summary
```

Recommended types:

- `docs`: documentation-only changes;
- `chore`: repository maintenance, metadata, tooling, or dependency updates;
- `feat`: new mock endpoint behavior or user-visible capability;
- `fix`: bug fixes or compatibility corrections;
- `test`: test-only changes;
- `refactor`: code restructuring without intended behavior changes;
- `perf`: performance or memory improvements;
- `ci`: continuous integration changes.

Keep the summary under 72 characters when practical, use lowercase type names,
and write the summary in the imperative mood.

Examples:

```text
docs: clarify unofficial mock server status
docs: add public repository metadata
chore: set public Go module path
refactor: split mock server into internal package
test: cover rate limit behavior
fix(api): preserve requested omsId in report status
```

For git-flow branches, prefer short lowercase branch names:

```text
feature/public-positioning
feature/license-and-metadata
feature/basic-tests
release/v0.1.0
hotfix/readme-security-warning
```
