## Summary

Describe the change and why it is needed.

## Type

- [ ] `docs`: documentation-only change
- [ ] `chore`: repository maintenance, metadata, tooling, or dependency update
- [ ] `feat`: new mock endpoint behavior or user-visible capability
- [ ] `fix`: bug fix or compatibility correction
- [ ] `test`: test-only change
- [ ] `refactor`: code restructuring without intended behavior change
- [ ] `perf`: performance or memory improvement
- [ ] `ci`: continuous integration change

## Checklist

- [ ] Target branch is `develop`, unless this is a release or hotfix.
- [ ] Commit messages follow `<type>(optional-scope): imperative summary`.
- [ ] I ran `gofmt -w .`.
- [ ] I ran `go test ./...`.
- [ ] I ran `go vet ./...`.
- [ ] I added or updated focused tests, or explained why tests are not needed.
- [ ] I documented new environment variables, endpoints, or scenario parameters.
- [ ] Examples, fixtures, and request/response snippets are synthetic.
- [ ] No real tokens, certificates, private keys, credentials, OMS IDs, GTINs,
      order IDs, production payloads, or proprietary examples are included.

## Validation

Paste relevant command output or explain why a check was not run.

```text
go test ./...
```

## Notes For Reviewers

Call out compatibility risks, intentionally partial behavior, or follow-up work.
