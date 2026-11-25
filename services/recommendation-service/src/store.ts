import { Redis } from "ioredis";
import { LRUCache } from "lru-cache";

import type { Config } from "./config.js";
import { decodeBrands, decodeScored, type Scored } from "./codec.js";
import { ACTIVE_VERSION_KEY, keys } from "./keys.js";
import { activeVersionInfo, breakerOpen, cacheRequests, valkeyDuration } from "./metrics.js";

type Keyspace = "version" | "catalog" | "popular" | "similar" | "user";

/**
 * Two-tier read path over offline model outputs.
 *
 *   L1  in-process LRU (per pod): active version pointer, catalog side table, popular list,
 *       item-to-item neighbours. Low cardinality, high reuse, tolerant of seconds of staleness.
 *   L2  Valkey (shared): everything, including per-user lists. User lists are deliberately not
 *       cached in L1: with millions of users the per-pod hit ratio is near zero and they would
 *       only evict the hot keys.
 *
 * L1 entries are loaded through lru-cache's fetchMethod, which gives us three properties
 * without extra code:
 *   - request coalescing: concurrent misses for one key issue a single Valkey read;
 *   - invalidation by versioning: list and catalog keys embed the model version, so they are
 *     immutable and only need an LRU size bound; the version pointer alone has a short TTL;
 *   - stale-while-revalidate: an expired entry is served immediately while one background
 *     fetch refreshes it, so no request ever waits on a hot-key refresh;
 *   - stale-on-error: if Valkey is slow or down, the last good value keeps serving.
 * When the active pointer flips, readers simply ask for different keys and the old
 * entries age out of the LRU.
 */
export class RecoStore {
  readonly redis: Redis;
  private readonly versionCache: LRUCache<"active", string>;
  private readonly catalogCache: LRUCache<string, Uint16Array>;
  private readonly listCache: LRUCache<string, Scored[]>;
  private lastReportedVersion: string | null = null;
  // Circuit breaker: after N consecutive Valkey failures, skip Valkey for a cool-down so requests
  // drop straight to the fallback instead of each one waiting out the command timeout.
  private consecutiveFailures = 0;
  private openUntil = 0;

  constructor(private readonly cfg: Config) {
    this.redis = new Redis(cfg.valkeyUrl, {
      // Fail fast: the caller has a fallback, so a queued or retried command only adds latency.
      commandTimeout: cfg.valkeyCommandTimeoutMs,
      enableOfflineQueue: false,
      maxRetriesPerRequest: 0,
      // Coalesce all commands issued in the same event-loop tick into one round trip.
      enableAutoPipelining: true,
      connectTimeout: 2_000,
    });
    this.redis.on("error", () => {
      // Connection errors surface as failed commands and metrics; avoid log floods at high RPS.
    });

    this.versionCache = new LRUCache<"active", string>({
      max: 1,
      ttl: cfg.activeVersionTtlMs,
      allowStale: true, // stale-while-revalidate: serve the old value, refresh in the background
      allowStaleOnFetchRejection: true,
      noDeleteOnFetchRejection: true,
      fetchMethod: async () => {
        const v = await this.timed("version", () => this.redis.get(ACTIVE_VERSION_KEY));
        return v ?? undefined;
      },
    });

    this.catalogCache = new LRUCache<string, Uint16Array>({
      max: 2, // current + previous version during a cutover
      allowStale: true, // stale-while-revalidate: serve the old value, refresh in the background
      allowStaleOnFetchRejection: true,
      noDeleteOnFetchRejection: true,
      fetchMethod: async (version) => {
        const buf = await this.timed("catalog", () => this.redis.getBuffer(keys.catalogBrands(version)));
        return buf ? decodeBrands(buf) : undefined;
      },
    });

    this.listCache = new LRUCache<string, Scored[]>({
      max: cfg.l1MaxEntries,
      allowStale: true, // stale-while-revalidate: serve the old value, refresh in the background
      allowStaleOnFetchRejection: true,
      noDeleteOnFetchRejection: true,
      fetchMethod: async (key) => {
        const keyspace: Keyspace = key.includes(":pop:") ? "popular" : "similar";
        const buf = await this.timed(keyspace, () => this.redis.getBuffer(key));
        // Negative caching: an unknown product caches as [] so repeated misses stay in L1.
        return buf ? decodeScored(buf) : [];
      },
    });
  }

  async activeVersion(): Promise<string | null> {
    const v = await this.l1Fetch("version", this.versionCache, "active");
    if (v && v !== this.lastReportedVersion) {
      activeVersionInfo.reset();
      activeVersionInfo.labels(v).set(1);
      this.lastReportedVersion = v;
    }
    return v ?? null;
  }

  async brands(version: string): Promise<Uint16Array | null> {
    return (await this.l1Fetch("catalog", this.catalogCache, version)) ?? null;
  }

  async popular(version: string): Promise<Scored[]> {
    return (await this.l1Fetch("popular", this.listCache, keys.popular(version))) ?? [];
  }

  async similar(version: string, productId: number): Promise<Scored[]> {
    return (await this.l1Fetch("similar", this.listCache, keys.similar(version, productId))) ?? [];
  }

  /** Per-user list: L2 only. Returns null when the user has no model output (cold start). */
  async userList(version: string, userId: string): Promise<Scored[] | null> {
    try {
      const buf = await this.timed("user", () => this.redis.getBuffer(keys.user(version, userId)));
      cacheRequests.labels("l2", "user", buf ? "hit" : "miss").inc();
      return buf ? decodeScored(buf) : null;
    } catch {
      cacheRequests.labels("l2", "user", "error").inc();
      return null;
    }
  }

  async ping(): Promise<boolean> {
    try {
      return (await this.redis.ping()) === "PONG";
    } catch {
      return false;
    }
  }

  async close(): Promise<void> {
    await this.redis.quit().catch(() => this.redis.disconnect());
  }

  private async l1Fetch<K extends string, V extends {}>(
    keyspace: Keyspace,
    cache: LRUCache<K, V>,
    key: K,
  ): Promise<V | undefined> {
    const status: LRUCache.Status<V> = {};
    try {
      const value = await cache.fetch(key, { status });
      cacheRequests.labels("l1", keyspace, l1Result(status)).inc();
      return value;
    } catch {
      // No stale value to fall back to: the caller moves down its fallback chain.
      cacheRequests.labels("l1", keyspace, "error").inc();
      return undefined;
    }
  }

  private async timed<T>(keyspace: Keyspace, op: () => Promise<T>): Promise<T> {
    if (Date.now() < this.openUntil) {
      valkeyDuration.labels(keyspace, "short_circuited").observe(0);
      throw new Error("valkey circuit open");
    }
    const end = valkeyDuration.startTimer({ keyspace });
    try {
      const result = await op();
      end({ outcome: "ok" });
      if (this.consecutiveFailures > 0) {
        this.consecutiveFailures = 0;
        breakerOpen.set(0);
      }
      return result;
    } catch (err) {
      end({ outcome: "error" });
      if (++this.consecutiveFailures >= this.cfg.breakerFailureThreshold) {
        // Re-arms on every failure while tripped; the first request after the cool-down is the probe.
        this.openUntil = Date.now() + this.cfg.breakerCooldownMs;
        breakerOpen.set(1);
      }
      throw err;
    }
  }
}

function l1Result(status: LRUCache.Status<unknown>): "hit" | "miss" | "coalesced" | "stale" | "error" {
  if (status.returnedStale) return "stale";
  if (status.fetchRejected || status.fetchError) return "error";
  if (status.fetch === "hit") return "hit";
  // Another request already has a Valkey read in flight for this key; we await that one.
  if (status.fetch === "inflight") return "coalesced";
  return "miss";
}
