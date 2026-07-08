# Security Policy

## Scope

This project is a local mock server for development and contract testing. It is
not a production service and must not be used to process real SUZ credentials,
certificates, tokens, order identifiers, GTINs, OMS IDs, or production payloads.

## Reporting a Vulnerability

If you find a security issue, open a private report through the hosting
platform when available. If private reporting is not available, open a public
issue with only a high-level description and avoid posting exploit details,
secrets, certificates, tokens, or production data.

Please include:

- affected version or commit;
- steps to reproduce with synthetic data;
- expected and actual behavior;
- any relevant logs with secrets removed.

## Secret Handling

Never commit real credentials, client tokens, access tokens, private keys,
certificates, OMS IDs, GTINs, order IDs, production request bodies, or copied
vendor examples. Use synthetic fixture values in examples, tests, and issue
reports.

If a real secret is committed, rotate or revoke it outside this repository
before opening a cleanup issue or pull request.

## Supported Versions

Until the project publishes tagged releases, security fixes target the current
default branch only.
