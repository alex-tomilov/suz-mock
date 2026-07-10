---
name: Endpoint request
about: Request new or improved mock coverage for an API endpoint
title: "feat: mock "
labels: enhancement
assignees: ""
---

## Safety Reminder

Do not paste real tokens, certificates, private keys, credentials, OMS IDs,
GTINs, order IDs, production payloads, or proprietary examples. Use synthetic
field names, generated IDs, and minimal payload shapes.

## Endpoint

- Method:
- Path:
- Current status: missing/partial/incorrect

## Client Use Case

What client behavior, parser branch, retry path, or integration test would this
endpoint support?

## Desired Mock Behavior

Describe the response shape, status codes, headers, or scenario options needed.

## Suggested Scenarios

- happy path:
- forced error:
- latency/rate limit/streaming:
- large response:

## Synthetic Example

```bash
curl 'http://localhost:8080/api/v3/example?omsId=mock-oms-local'
```

```json
{
  "example": "synthetic"
}
```

## Compatibility Notes

Mention any client assumptions this mock behavior must preserve.
