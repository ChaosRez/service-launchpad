import client from "prom-client";

export const registry = new client.Registry();

// process_cpu_seconds_total, nodejs_eventloop_lag_*, heap, GC. Event-loop lag is the first
// signal that a single-threaded Node pod is saturated, usually before CPU reaches its limit.
client.collectDefaultMetrics({ register: registry, eventLoopMonitoringPrecision: 10 });

export const httpDuration = new client.Histogram({
  name: "reco_http_request_duration_seconds",
  help: "Server-side HTTP latency by route and status.",
  labelNames: ["route", "status"] as const,
  // Tight buckets around the latency SLO (p99 < 50 ms) instead of generic web buckets.
  buckets: [0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1],
  registers: [registry],
});

export const feedServed = new client.Counter({
  name: "reco_feed_served_total",
  help: "Feeds served, by surface, primary candidate source, and experiment variant.",
  labelNames: ["surface", "source", "variant"] as const,
  registers: [registry],
});

export const feedBackfilled = new client.Counter({
  name: "reco_feed_backfilled_total",
  help: "Feeds whose primary source was too short and needed popularity backfill.",
  labelNames: ["surface"] as const,
  registers: [registry],
});

export const feedDistinctBrands = new client.Histogram({
  name: "reco_feed_distinct_brands",
  help: "Distinct brands in a returned feed. The A/B guardrail metric for diversity re-ranking.",
  labelNames: ["surface", "variant"] as const,
  buckets: [2, 4, 6, 8, 10, 12, 14, 16, 18, 20, 24],
  registers: [registry],
});

export const cacheRequests = new client.Counter({
  name: "reco_cache_requests_total",
  help: "Lookups per cache tier and keyspace. result is hit, miss, coalesced, stale, or error.",
  labelNames: ["tier", "keyspace", "result"] as const,
  registers: [registry],
});

export const valkeyDuration = new client.Histogram({
  name: "reco_valkey_command_duration_seconds",
  help: "Latency of Valkey reads issued by the service.",
  labelNames: ["keyspace", "outcome"] as const,
  buckets: [0.0002, 0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05],
  registers: [registry],
});

export const activeVersionInfo = new client.Gauge({
  name: "reco_model_version_info",
  help: "Model-output version currently served (value is always 1).",
  labelNames: ["version"] as const,
  registers: [registry],
});

export const breakerOpen = new client.Gauge({
  name: "reco_valkey_circuit_open",
  help: "1 while the Valkey circuit breaker is open and reads short-circuit to the fallback chain.",
  registers: [registry],
});
