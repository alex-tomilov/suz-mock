# SUZ API v3 Go mock

## Public safety notice

This is an unofficial local contract-oriented mock server. It is not affiliated
with, endorsed by, certified by, or supported by SUZ, Chestny Znak, or CRPT.

Use it only for local development, parser tests, client integration tests, and
contract checks where a predictable HTTP server is useful. It intentionally does
not perform real authentication, request signing, cryptography, marking-code
generation, or SUZ business validation.

Do not commit or paste real credentials, OMS IDs, GTINs, order IDs,
certificates, private keys, access tokens, client tokens, production request
payloads, or proprietary examples into this repository. Keep examples synthetic
and do not copy chunks from official PDFs, private integrations, or vendor
documentation.

Small local mock for contract/development tests when real SUZ credentials are
unavailable. It focuses on stable response shapes for API v3 client code while
remaining deliberately detached from real SUZ services.

## Project metadata

- License: MIT, see [LICENSE](LICENSE).
- Security policy: see [SECURITY.md](SECURITY.md).
- Contribution guide: see [CONTRIBUTING.md](CONTRIBUTING.md).
- Code of conduct: see [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).
- Changes: see [CHANGELOG.md](CHANGELOG.md).

## Run

```bash
go run ./cmd/suz-mock
```

Default address: `:8080`.

```bash
SUZ_MOCK_ADDR=:9090 go run ./cmd/suz-mock
```

## Docker

Build and run the local image:

```bash
docker build -t suz-mock:local .
docker run --rm -p 8080:8080 suz-mock:local
```

When a published image is available, run it directly:

```bash
docker run --rm -p 8080:8080 ghcr.io/alex-tomilov/suz-mock:latest
```

Docker Compose is available for local use:

```bash
docker compose up --build
```

## Useful options

```bash
# Require either a mock clientToken or Authorization header.
SUZ_MOCK_REQUIRE_TOKEN=true go run ./cmd/suz-mock

# Generate dynamic order/report ids instead of deterministic fixture ids.
SUZ_MOCK_DYNAMIC_IDS=true go run ./cmd/suz-mock

# Override deterministic order id.
SUZ_MOCK_ORDER_ID=11111111-1111-4111-8111-111111111111 go run ./cmd/suz-mock
```

## Implemented endpoints

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/healthz` | mock healthcheck |
| GET | `/api/v3/ping?omsId=...` | SUZ availability |
| POST | `/api/v3/order?omsId=...` | create emission order |
| GET | `/api/v3/order/status?omsId=...&orderId=...&gtin=...` | get code buffer status |
| GET | `/api/v3/order/list?omsId=...` | list orders |
| GET | `/api/v3/codes?omsId=...&orderId=...&gtin=...&quantity=...` | get codes |
| GET | `/api/v3/order/codes/blocks?omsId=...&orderId=...&gtin=...` | list issued code blocks |
| GET | `/api/v3/order/codes/retry?omsId=...&blockId=...` | retry getting codes by block id |
| GET | `/api/v3/order/product?omsId=...&orderId=...` | product attributes |
| POST | `/api/v3/order/close?omsId=...` | close order/suborder |
| POST | `/api/v3/dropout?omsId=...` | dropout/rejection report |
| POST | `/api/v3/aggregation?omsId=...` | aggregation report |
| POST | `/api/v3/utilisation?omsId=...` | utilisation/application report |
| POST | `/api/v3/surplus?omsId=...` | surplus acceptance report |
| GET | `/api/v3/report/info?omsId=...&reportId=...` | report processing status |
| GET | `/api/v3/providers?omsId=...` | service providers |
| GET | `/api/v3/documents/content?omsId=...&docId=...` | document content |

## API-scale / heavy-load helpers

The API materials describe these useful scale boundaries:

- up to 10 product positions in one emission order;
- up to 2,000,000 codes for one GTIN in one order;
- up to 150,000 codes per GTIN when an order contains several GTINs;
- up to 30,000 codes/items in utilisation, dropout, and aggregation reports;
- up to 100 active orders;
- create-order call rate mentioned as 100 requests/sec per IP + `omsId`.

The mock does not validate every business rule, but it now lets you reproduce these shapes safely.

### Huge `/api/v3/codes` responses

By default, large code responses are streamed instead of being fully allocated by the mock process.

```bash
# API-report-sized stress response: 30,000 codes.
curl -o /tmp/suz-codes-30k.json \
  'http://localhost:8080/api/v3/codes?omsId=test&gtin=01334567894339&__scenario=huge'

# Same, but explicitly choose size.
curl -o /tmp/suz-codes-50k.json \
  'http://localhost:8080/api/v3/codes?omsId=test&gtin=01334567894339&quantity=50000'
```

The default maximum for one `/api/v3/codes` response is 30,000 codes. Raise it when you intentionally want larger tests:

```bash
# Multi-GTIN order-sized response: up to 150,000 codes.
SUZ_MOCK_MAX_CODES_PER_RESPONSE=150000 go run ./cmd/suz-mock
curl -o /tmp/suz-codes-150k.json \
  'http://localhost:8080/api/v3/codes?omsId=test&gtin=01334567894339&__scenario=massive'

# Single-GTIN API maximum: up to 2,000,000 codes. This can produce a very large file.
SUZ_MOCK_MAX_CODES_PER_RESPONSE=2000000 go run ./cmd/suz-mock
curl -o /tmp/suz-codes-2m.json \
  'http://localhost:8080/api/v3/codes?omsId=test&gtin=01334567894339&__scenario=api_max'
```

Streaming controls:

```bash
# Flush every 500 generated codes and pause 10ms after each flush to mimic a slow/chunky network.
curl -o /tmp/suz-codes-slow.json \
  'http://localhost:8080/api/v3/codes?omsId=test&quantity=30000&__stream=1&__flushEvery=500&__chunkDelay=10ms'
```

Environment variables:

| Variable | Default | Purpose |
| --- | ---: | --- |
| `SUZ_MOCK_MAX_CODES_PER_RESPONSE` | `30000` | safety cap for `/api/v3/codes` and retry-code responses |
| `SUZ_MOCK_STREAM_THRESHOLD` | `1000` | responses above this count are streamed |
| `SUZ_MOCK_STREAM_FLUSH_EVERY` | `1000` | flush interval for streamed JSON arrays |
| `SUZ_MOCK_MAX_ORDER_QUANTITY` | `2000000` | cap for stored mock order quantity |
| `SUZ_MOCK_MAX_ORDER_LIST` | `100` | cap for synthetic order-list size |
| `SUZ_MOCK_MAX_BLOCKS` | `1000` | cap for synthetic code-block list size |
| `SUZ_MOCK_MAX_PRODUCTS` | `10` | cap for product-attributes map size |
| `SUZ_MOCK_MAX_DOCUMENT_ITEMS` | `30000` | cap for synthetic document `cislist` size |

### Synthetic large lists

```bash
# Simulate 100 active orders with 10 buffers each.
curl -o /tmp/suz-orders.json \
  'http://localhost:8080/api/v3/order/list?omsId=test&__scenario=huge&__buffers=10&quantity=150000'

# Simulate many block ids for already issued code blocks.
curl -o /tmp/suz-blocks.json \
  'http://localhost:8080/api/v3/order/codes/blocks?omsId=test&__scenario=huge&__blocks=1000&__blockQuantity=1000'

# Simulate 10 product attributes, matching the order product-position API boundary.
curl -o /tmp/suz-products.json \
  'http://localhost:8080/api/v3/order/product?omsId=test&__scenario=huge'

# Simulate a large document content cislist, useful for report/document parsers.
curl -o /tmp/suz-document-content.json \
  'http://localhost:8080/api/v3/documents/content?omsId=test&__scenario=huge'
```

### Latency, chunking, and rate-limit simulation

```bash
# Add fixed latency before any endpoint response.
curl 'http://localhost:8080/api/v3/ping?omsId=test&__delay=750ms'

# Enable mock 429 responses after N requests per second per remote address + omsId + path.
SUZ_MOCK_RATE_LIMIT=true SUZ_MOCK_RATE_LIMIT_PER_SECOND=100 go run ./cmd/suz-mock

# Or enable it per request for focused tests.
curl 'http://localhost:8080/api/v3/ping?omsId=test&__rateLimit=1'
```

## Scenarios

Force API-like error response:

```bash
curl 'http://localhost:8080/api/v3/ping?omsId=test&__error=400'
```

Buffer status scenarios:

```bash
curl 'http://localhost:8080/api/v3/order/status?__scenario=active'
curl 'http://localhost:8080/api/v3/order/status?__scenario=pending'
curl 'http://localhost:8080/api/v3/order/status?__scenario=rejected'
```

Report status scenario:

```bash
curl 'http://localhost:8080/api/v3/report/info?__scenario=failed'
curl 'http://localhost:8080/api/v3/report/info?__scenario=in_progress'
```

Empty product attributes:

```bash
curl 'http://localhost:8080/api/v3/order/product?__scenario=empty'
```

## Smoke test

The UUID-shaped values below are deterministic mock fixture IDs used by the
default local server configuration.

```bash
OMS_ID='mock-oms-local'
GTIN='00000000000000'
ORDER_ID='b024ae09-ef7c-449e-b461-05d8eb116c79'
REPORT_ID='fab1c0e4-9590-4ed7-8d58-18862d6a9aab'

curl -s "http://localhost:8080/api/v3/ping?omsId=$OMS_ID" | jq .

curl -s -X POST "http://localhost:8080/api/v3/order?omsId=$OMS_ID" \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'clientToken: mock-token' \
  -d "{\"productGroup\":\"mockpharma\",\"products\":[{\"gtin\":\"$GTIN\",\"quantity\":20,\"templateId\":50}]}" | jq .

curl -s "http://localhost:8080/api/v3/order/status?omsId=$OMS_ID&orderId=$ORDER_ID&gtin=$GTIN" | jq .

curl -s "http://localhost:8080/api/v3/codes?omsId=$OMS_ID&orderId=$ORDER_ID&gtin=$GTIN&quantity=2" | jq .

curl -s -X POST "http://localhost:8080/api/v3/utilisation?omsId=$OMS_ID" \
  -H 'Content-Type: application/json' \
  -d '{"productGroup":"mockpharma","sntins":["MOCK-CODE-0001"]}' | jq .

curl -s "http://localhost:8080/api/v3/report/info?omsId=$OMS_ID&reportId=$REPORT_ID" | jq .
```

## Ruby client test idea

Point your gem at `http://localhost:8080` as the base SUZ URL. Keep the `/api/v3/...` paths unchanged.
For request-signature tests, assert that your Ruby client sends `X-Signature`, but let this mock ignore the value.

Recommended parser stress test cases:

1. `quantity=30000` for a realistic large JSON array.
2. `quantity=150000` with `SUZ_MOCK_MAX_CODES_PER_RESPONSE=150000` for multi-GTIN order scale.
3. `__stream=1&__flushEvery=500&__chunkDelay=10ms` to catch buffering/timeouts.
4. `__rateLimit=1` or `SUZ_MOCK_RATE_LIMIT=true` to verify retry/backoff behavior.
5. `__delay=2s` to verify request timeout configuration.
