---
name: Bug report
about: Report incorrect or surprising mock behavior
title: "fix: "
labels: bug
assignees: ""
---

## Safety Reminder

Do not paste real tokens, certificates, private keys, credentials, OMS IDs,
GTINs, order IDs, production payloads, or proprietary examples. Use synthetic
values and reduce payloads to the smallest safe reproduction.

## Summary

Describe the behavior that looks wrong.

## Reproduction

1. Start the mock with:

   ```bash
   go run ./cmd/suz-mock
   ```

2. Send this synthetic request:

   ```bash
   curl 'http://localhost:8080/api/v3/ping?omsId=mock-oms-local'
   ```

## Expected Behavior

What should the mock return?

## Actual Behavior

What did it return instead?

## Environment

- suz-mock version or commit:
- OS:
- Go version:
- Docker usage: yes/no
- Relevant `SUZ_MOCK_*` variables:

## Additional Context

Add any synthetic response snippets, test failure output, or client parser notes.
