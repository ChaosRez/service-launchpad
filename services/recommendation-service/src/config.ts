function int(name: string, fallback: number): number {
  const raw = process.env[name];
  if (raw === undefined || raw === "") return fallback;
  const value = Number(raw);
  if (!Number.isFinite(value)) throw new Error(`${name} must be a number, got "${raw}"`);
  return value;
}

export const config = {
  port: int("PORT", 8080),
  valkeyUrl: process.env.VALKEY_URL ?? "redis://127.0.0.1:6379",
  // Hard per-command budget. A slow hot store must degrade to a fallback, not stall the feed.
  valkeyCommandTimeoutMs: int("VALKEY_COMMAND_TIMEOUT_MS", 25),
  breakerFailureThreshold: int("BREAKER_FAILURE_THRESHOLD", 20),
  breakerCooldownMs: int("BREAKER_COOLDOWN_MS", 1_000),

  // L1 (in-process) tier: only low-cardinality, high-reuse keys live here. List entries are keyed
  // by model version and therefore immutable, so they need a size bound but no TTL. The only TTL
  // is on the version pointer: it bounds how long a pod keeps serving a superseded model.
  l1MaxEntries: int("L1_MAX_ENTRIES", 20_000),
  activeVersionTtlMs: int("ACTIVE_VERSION_TTL_MS", 5_000),

  defaultLimit: int("DEFAULT_LIMIT", 24),
  maxLimit: int("MAX_LIMIT", 60),

  // A/B experiment: variant "diverse" applies a brand cap during re-ranking.
  experimentName: process.env.EXPERIMENT_NAME ?? "brand-diversity-v1",
  experimentTreatmentPercent: int("EXPERIMENT_TREATMENT_PERCENT", 50),
  brandCap: int("BRAND_CAP", 2),

  // Static last-resort fallback: product ids that always exist in the seeded catalog.
  staticFallbackSize: int("STATIC_FALLBACK_SIZE", 60),
} as const;

export type Config = typeof config;
