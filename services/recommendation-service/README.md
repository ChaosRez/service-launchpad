# recommendation-service

A minimal Node.js/TypeScript recommendation feed service. It serves offline model outputs from
Valkey through a two-tier cache and assembles feeds with a fallback chain, deduplication,
brand-diversity re-ranking, and deterministic A/B bucketing. It is instrumented so that
autoscaling under constant k6 load can be observed in Grafana.

Deployment and the demo runbook live in
[`k8s/recommendation/README.md`](../../k8s/recommendation/README.md).

## API

| Endpoint | Purpose |
| --- | --- |
| `GET /v1/recommendations/home?userId=&limit=&exclude=` | Home feed: personalized, then popular, then static |
| `GET /v1/recommendations/product/:productId?userId=&limit=&exclude=` | Product page: similar items, then popular, then static. The product itself is excluded. |
| `GET /healthz` | Liveness. Never checks Valkey. |
| `GET /readyz` | Readiness. Reports Valkey state but does not fail on it. |
| `GET /metrics` | Prometheus metrics |

Example response (truncated):

```json
{"surface":"home","userId":"u1","variant":"diverse","experiment":"brand-diversity-v1",
 "modelVersion":"v20261125T195125","source":"personalized",
 "items":[{"productId":1740,"brandId":17,"score":1.53,"source":"personalized"}]}
```

## Design

**Data model (written by `src/seed.ts`, read by `src/store.ts`; key layout in `src/keys.ts`)**

| Key | Value |
| --- | --- |
| `reco:active` / `reco:previous` | Version pointers |
| `reco:<v>:u:<userId>` | Matrix-factorization top-200 for a user |
| `reco:<v>:sim:<productId>` | Top-50 neighbours by item-embedding cosine |
| `reco:<v>:pop:global` | Top-500 by popularity (cold start and backfill) |
| `reco:<v>:catalog:brands` | `u16` brand id per product id |

Lists use a fixed binary layout (`src/codec.ts`): `u8 version, u16 count, count x (u32 id, f32 score)`.
That is 1.6 KB for 200 candidates versus about 5 KB of JSON, and decoding never calls
`JSON.parse`. A real system would put Protobuf/Avro here for schema evolution.

**Reverse-ETL load protocol.** Write a full new version under a fresh prefix, then flip
`reco:active` in one `MULTI`, then delete the version two generations back. Readers never see a
half-written load, and rollback is a pointer swap (`node dist/seed.js --rollback`).

**Cache tiers.**

- **L1** is an in-process LRU (`lru-cache` with `fetchMethod`). It holds the version pointer
  (5 s TTL), the catalog, the popular list, and similar-item lists. Because these keys embed the
  version, they are immutable and need only a size bound. Invalidation happens by versioning, not
  by TTL. L1 provides request coalescing, stale-while-revalidate, and stale-on-error.
- **L2** is Valkey. Per-user lists are read from L2 only: with millions of users the per-pod hit
  ratio would be close to zero, and those entries would only evict the hot keys.

**Latency budget.** Each Valkey command has a 25 ms timeout, no offline queue, and no retries.
ioredis auto-pipelining folds all reads from one event-loop tick into a single round trip. After
20 consecutive failures, a circuit breaker skips Valkey for 1 s. In a local Valkey-outage test this
took p50 from 30 ms (every request waiting out the timeout) to 1 ms, with zero failed requests.

**Feed assembly (`src/feed.ts`, pure and unit-tested).** Assembly walks the fallback chain in
order, dedupes across lists, drops excluded ids, and applies a soft per-brand cap. Over-cap items
are parked and used only if the feed would otherwise be short. The work is O(candidates), with no
sort because upstream lists are already ranked.

**Experiment.** `fnv1a("<experiment>:<userId>") % 100 < treatment%` assigns each user to `diverse`
(brand cap 2) or `control` (pure model order). Assignment is stable across pods with no shared
state, and anonymous requests get `none`. The guardrail metric is distinct brands per feed.

**Metrics.**

| Metric | Labels | What it shows |
| --- | --- | --- |
| `reco_http_request_duration_seconds` | route, status | Latency; buckets centred on the 50 ms SLO |
| `reco_feed_served_total` | surface, source, variant | Coverage and fallback rate per surface; experiment exposure |
| `reco_feed_distinct_brands` | surface, variant | A/B guardrail |
| `reco_feed_backfilled_total` | surface | Primary list was too short |
| `reco_cache_requests_total` | tier, keyspace, result | `hit`, `miss`, `coalesced`, `stale`, `error` |
| `reco_valkey_command_duration_seconds` | keyspace, outcome | Valkey latency, including `short_circuited` |
| `reco_valkey_circuit_open` | none | Breaker state |
| `reco_model_version_info` | version | Version currently served |
| Node defaults | none | `process_cpu_seconds_total`, `nodejs_eventloop_lag_*`, heap, GC |

Every label has a fixed, small value set. User ids and product ids never become labels, and the
k6 script sets a `name` tag so k6 does not create one series per URL.

## Local development

```bash
npm ci
npm test                                   # tsc + node:test unit tests
docker run -d --rm --name valkey -p 6379:6379 valkey/valkey:8.1.4-alpine
npm run build && npm run seed              # ~5 s: 20k users, 5k products
npm start                                  # :8080
curl -s 'localhost:8080/v1/recommendations/home?userId=u1&limit=5'
BASE_URL=http://127.0.0.1:8080 RATE=1000 DURATION=30s k6 run ../../loadtests/k6/recommendation-service.js
```

Local reference numbers (Apple Silicon, single process): 3,000 rps at p99 2.4 ms server-side,
about 0.16 ms CPU per request.
