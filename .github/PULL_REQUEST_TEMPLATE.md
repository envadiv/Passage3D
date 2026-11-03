## Description

<!-- What the change does and why. Link the related issue, if any. -->

## Checklist

- [ ] Conventional commit type in the title (`feat`, `fix`, `chore`, `docs`, `test`, `ci`, ...)
- [ ] State-machine changes are behind an upgrade handler and covered by tests
- [ ] `make test` passes; `make test-upgrade` / `make test-contracts` run when the upgrade path or wasm is affected
- [ ] `CHANGELOG.md` updated
- [ ] Documentation updated
