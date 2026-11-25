import Fastify from "fastify";

import type { Scored } from "./codec.js";
import { config } from "./config.js";
import { assembleFeed, assignVariant, distinctBrands, type CandidateList, type Variant } from "./feed.js";
import { feedBackfilled, feedDistinctBrands, feedServed, httpDuration, registry } from "./metrics.js";
import { RecoStore } from "./store.js";

type Surface = "home" | "product";

const store = new RecoStore(config);

// Last-resort stage of every fallback chain: needs no I/O, so the feed is never empty.
const STATIC_FALLBACK: Scored[] = Array.from({ length: config.staticFallbackSize }, (_, id) => ({ id, score: 0 }));

interface RecommendArgs {
  surface: Surface;
  userId?: string;
  productId?: number;
  limit: number;
  exclude: Set<number>;
}

async function recommend({ surface, userId, productId, limit, exclude }: RecommendArgs) {
  const variant: Variant = assignVariant(userId, config.experimentName, config.experimentTreatmentPercent);
  const version = await store.activeVersion();
  const chain: CandidateList[] = [];
  let brands: Uint16Array | null = null;

  if (version) {
    // Independent reads go out together; ioredis auto-pipelining folds the L1 misses and the
    // per-user GET into a single round trip.
    const primaryPromise =
      surface === "home"
        ? userId
          ? store.userList(version, userId)
          : Promise.resolve(null)
        : store.similar(version, productId!);
    const [catalog, popular, primary] = await Promise.all([
      store.brands(version),
      store.popular(version),
      primaryPromise,
    ]);
    brands = catalog;
    if (primary && primary.length > 0) {
      chain.push({ source: surface === "home" ? "personalized" : "similar", items: primary });
    }
    chain.push({ source: "popular", items: popular });
  }
  chain.push({ source: "static", items: STATIC_FALLBACK });

  if (surface === "product" && productId !== undefined) exclude.add(productId);

  const result = assembleFeed({
    chain,
    limit,
    exclude,
    brandOf: (id) => (brands && id < brands.length ? brands[id]! : -1),
    brandCap: variant === "diverse" ? config.brandCap : null,
  });

  feedServed.labels(surface, result.primarySource, variant).inc();
  if (result.backfilled) feedBackfilled.labels(surface).inc();
  feedDistinctBrands.labels(surface, variant).observe(distinctBrands(result.items));

  return {
    surface,
    userId: userId ?? null,
    variant,
    experiment: config.experimentName,
    modelVersion: version,
    source: result.primarySource,
    items: result.items,
  };
}

function parseExclude(raw: string | undefined): Set<number> {
  const out = new Set<number>();
  if (!raw) return out;
  for (const part of raw.split(",", 200)) {
    const n = Number(part);
    if (Number.isInteger(n) && n >= 0) out.add(n);
  }
  return out;
}

// Response schema lets Fastify compile a dedicated serializer instead of generic JSON.stringify.
const feedResponseSchema = {
  200: {
    type: "object",
    properties: {
      surface: { type: "string" },
      userId: { type: ["string", "null"] },
      variant: { type: "string" },
      experiment: { type: "string" },
      modelVersion: { type: ["string", "null"] },
      source: { type: "string" },
      items: {
        type: "array",
        items: {
          type: "object",
          properties: {
            productId: { type: "integer" },
            brandId: { type: "integer" },
            score: { type: "number" },
            source: { type: "string" },
          },
        },
      },
    },
  },
} as const;

const limitSchema = { type: "integer", minimum: 1, maximum: config.maxLimit, default: config.defaultLimit } as const;
const userIdSchema = { type: "string", minLength: 1, maxLength: 64, pattern: "^[A-Za-z0-9_-]+$" } as const;

const app = Fastify({
  logger: { level: process.env.LOG_LEVEL ?? "info" },
  disableRequestLogging: true, // per-request logs at thousands of RPS cost more than they tell us
});

app.addHook("onRequest", async (request) => {
  (request as unknown as { startedAt: bigint }).startedAt = process.hrtime.bigint();
});
app.addHook("onResponse", async (request, reply) => {
  const route = request.routeOptions.url ?? "unmatched";
  if (route === "/metrics" || route === "/healthz" || route === "/readyz") return;
  const started = (request as unknown as { startedAt: bigint }).startedAt;
  const seconds = Number(process.hrtime.bigint() - started) / 1e9;
  httpDuration.labels(route, String(reply.statusCode)).observe(seconds);
});

app.get<{ Querystring: { userId?: string; limit: number; exclude?: string } }>(
  "/v1/recommendations/home",
  {
    schema: {
      querystring: {
        type: "object",
        properties: { userId: userIdSchema, limit: limitSchema, exclude: { type: "string", maxLength: 2048 } },
      },
      response: feedResponseSchema,
    },
  },
  async (request) =>
    recommend({
      surface: "home",
      userId: request.query.userId,
      limit: request.query.limit,
      exclude: parseExclude(request.query.exclude),
    }),
);

app.get<{ Params: { productId: number }; Querystring: { userId?: string; limit: number; exclude?: string } }>(
  "/v1/recommendations/product/:productId",
  {
    schema: {
      params: {
        type: "object",
        properties: { productId: { type: "integer", minimum: 0 } },
        required: ["productId"],
      },
      querystring: {
        type: "object",
        properties: { userId: userIdSchema, limit: limitSchema, exclude: { type: "string", maxLength: 2048 } },
      },
      response: feedResponseSchema,
    },
  },
  async (request) =>
    recommend({
      surface: "product",
      productId: request.params.productId,
      userId: request.query.userId,
      limit: request.query.limit,
      exclude: parseExclude(request.query.exclude),
    }),
);

// Liveness: the process is serving. Never depends on Valkey, or a Valkey blip would restart
// every pod at once.
app.get("/healthz", async () => ({ status: "ok" }));

// Readiness also stays green without Valkey: the fallback chain still serves popular-from-L1 or
// the static list, which beats returning no recommendations. Valkey state is reported, not gated.
app.get("/readyz", async () => ({ status: "ready", valkey: (await store.ping()) ? "up" : "down" }));

app.get("/metrics", async (_request, reply) => {
  reply.header("Content-Type", registry.contentType);
  return registry.metrics();
});

async function shutdown(signal: string) {
  app.log.info({ signal }, "shutting down");
  await app.close();
  await store.close();
  process.exit(0);
}
process.on("SIGTERM", () => void shutdown("SIGTERM"));
process.on("SIGINT", () => void shutdown("SIGINT"));

await app.listen({ host: "0.0.0.0", port: config.port });
