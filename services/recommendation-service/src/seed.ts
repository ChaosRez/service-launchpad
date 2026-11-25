/**
 * Reverse-ETL loader: simulates "offline model outputs -> hot store".
 *
 * In production this job would read matrix-factorization and embedding-neighbour tables exported
 * from BigQuery. Here it generates them with a small, real MF-shaped model so the data has the
 * properties that matter for serving:
 *   - items carry a brand and an embedding clustered around that brand's centroid;
 *   - user taste vectors mix a few brand centroids, so a user's top-K is dominated by a handful of
 *     brands (which is exactly what the diversity A/B variant has to correct);
 *   - item popularity is long-tailed (Zipf-like), feeding the cold-start popularity list.
 *
 * Load protocol (zero-downtime, one-step rollback):
 *   1. write every key under a fresh immutable prefix  reco:<version>:*
 *   2. SET reco:previous <old active>, SET reco:active <version> in one MULTI
 *   3. delete the version that is now two generations old
 *
 * Usage:  node dist/seed.js             load a new version
 *         node dist/seed.js --rollback  swap active and previous
 */
import { Redis } from "ioredis";

import { encodeBrands, encodeScored, type Scored } from "./codec.js";
import { ACTIVE_VERSION_KEY, PREVIOUS_VERSION_KEY, keys } from "./keys.js";

const env = (name: string, fallback: number) => Number(process.env[name] ?? fallback);

const PRODUCTS = env("SEED_PRODUCTS", 5_000);
const BRANDS = env("SEED_BRANDS", 120);
const USERS = env("SEED_USERS", 20_000);
const DIM = env("SEED_DIM", 16);
const USER_TOP_K = env("SEED_USER_TOP_K", 200);
const SIMILAR_TOP_K = env("SEED_SIMILAR_TOP_K", 50);
const POPULAR_TOP_K = env("SEED_POPULAR_TOP_K", 500);
const BATCH = env("SEED_BATCH", 500);
const SEED = env("SEED_RANDOM_SEED", 42);

// Deterministic PRNG (mulberry32) so a given SEED reproduces the same "model".
let rngState = SEED >>> 0;
function rand(): number {
  rngState = (rngState + 0x6d2b79f5) >>> 0;
  let t = rngState;
  t = Math.imul(t ^ (t >>> 15), t | 1);
  t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
}
function gaussian(): number {
  const u = Math.max(rand(), 1e-12);
  return Math.sqrt(-2 * Math.log(u)) * Math.cos(2 * Math.PI * rand());
}
function zipfIndex(n: number, s = 1.1): number {
  // Inverse-CDF approximation; good enough to skew brand sizes and popularity.
  return Math.min(n - 1, Math.floor(Math.pow(rand(), s * 2.5) * n));
}

/** Keeps the K highest scores seen. O(N log K) instead of sorting all N candidates. */
function topK(scores: Float32Array, k: number, skip = -1): Scored[] {
  const heapIds = new Int32Array(k);
  const heapScores = new Float32Array(k);
  let size = 0;
  const siftDown = (i: number) => {
    for (;;) {
      const l = 2 * i + 1;
      const r = l + 1;
      let m = i;
      if (l < size && heapScores[l]! < heapScores[m]!) m = l;
      if (r < size && heapScores[r]! < heapScores[m]!) m = r;
      if (m === i) return;
      [heapScores[i], heapScores[m]] = [heapScores[m]!, heapScores[i]!];
      [heapIds[i], heapIds[m]] = [heapIds[m]!, heapIds[i]!];
      i = m;
    }
  };
  for (let id = 0; id < scores.length; id++) {
    if (id === skip) continue;
    const s = scores[id]!;
    if (size < k) {
      let i = size++;
      heapScores[i] = s;
      heapIds[i] = id;
      while (i > 0) {
        const p = (i - 1) >> 1;
        if (heapScores[p]! <= heapScores[i]!) break;
        [heapScores[i], heapScores[p]] = [heapScores[p]!, heapScores[i]!];
        [heapIds[i], heapIds[p]] = [heapIds[p]!, heapIds[i]!];
        i = p;
      }
    } else if (s > heapScores[0]!) {
      heapScores[0] = s;
      heapIds[0] = id;
      siftDown(0);
    }
  }
  const out: Scored[] = [];
  for (let i = 0; i < size; i++) out.push({ id: heapIds[i]!, score: heapScores[i]! });
  return out.sort((a, b) => b.score - a.score);
}

function generateModel() {
  const brandCentroids = new Float32Array(BRANDS * DIM).map(() => gaussian());
  const brandOf = new Uint16Array(PRODUCTS);
  const itemVecs = new Float32Array(PRODUCTS * DIM);
  const itemBias = new Float32Array(PRODUCTS);

  for (let p = 0; p < PRODUCTS; p++) {
    const b = zipfIndex(BRANDS);
    brandOf[p] = b;
    for (let d = 0; d < DIM; d++) itemVecs[p * DIM + d] = brandCentroids[b * DIM + d]! + 0.6 * gaussian();
    itemBias[p] = -Math.log(1 + zipfIndex(PRODUCTS)) * 0.15; // long-tail popularity
  }
  // Normalise item vectors so neighbour scores are cosine similarities.
  for (let p = 0; p < PRODUCTS; p++) {
    let norm = 0;
    for (let d = 0; d < DIM; d++) norm += itemVecs[p * DIM + d]! ** 2;
    norm = Math.sqrt(norm) || 1;
    for (let d = 0; d < DIM; d++) itemVecs[p * DIM + d]! /= norm;
  }
  return { brandCentroids, brandOf, itemVecs, itemBias };
}

async function flush(redis: Redis, batch: [string, Buffer][]) {
  if (batch.length === 0) return;
  const pipe = redis.pipeline();
  for (const [k, v] of batch) pipe.set(k, v);
  const results = await pipe.exec();
  for (const [err] of results ?? []) if (err) throw err;
  batch.length = 0;
}

async function deleteVersion(redis: Redis, version: string): Promise<number> {
  let cursor = "0";
  let deleted = 0;
  do {
    const [next, found] = await redis.scan(cursor, "MATCH", keys.versionPattern(version), "COUNT", 1000);
    cursor = next;
    if (found.length > 0) deleted += await redis.unlink(...found);
  } while (cursor !== "0");
  return deleted;
}

async function load(redis: Redis) {
  const version = `v${new Date().toISOString().replace(/[-:]/g, "").replace(/\..*/, "")}`;
  const started = Date.now();
  console.log(`generating model: products=${PRODUCTS} brands=${BRANDS} users=${USERS} dim=${DIM}`);
  const { brandCentroids, brandOf, itemVecs, itemBias } = generateModel();

  const batch: [string, Buffer][] = [];
  const push = async (k: string, v: Buffer) => {
    batch.push([k, v]);
    if (batch.length >= BATCH) await flush(redis, batch);
  };

  await push(keys.catalogBrands(version), encodeBrands(brandOf));

  // Popularity list (cold start + backfill).
  await push(keys.popular(version), encodeScored(topK(itemBias, POPULAR_TOP_K)));

  // Item-to-item neighbours from embeddings.
  const scores = new Float32Array(PRODUCTS);
  for (let p = 0; p < PRODUCTS; p++) {
    for (let q = 0; q < PRODUCTS; q++) {
      let dot = 0;
      for (let d = 0; d < DIM; d++) dot += itemVecs[p * DIM + d]! * itemVecs[q * DIM + d]!;
      scores[q] = dot;
    }
    await push(keys.similar(version, p), encodeScored(topK(scores, SIMILAR_TOP_K, p)));
  }
  console.log(`similar lists written (${((Date.now() - started) / 1000).toFixed(1)}s)`);

  // Matrix-factorization style user top-K: score = user . item + item bias.
  const userVec = new Float32Array(DIM);
  for (let u = 0; u < USERS; u++) {
    userVec.fill(0);
    const tastes = 1 + Math.floor(rand() * 3);
    for (let t = 0; t < tastes; t++) {
      const b = zipfIndex(BRANDS);
      for (let d = 0; d < DIM; d++) userVec[d]! += brandCentroids[b * DIM + d]! / tastes;
    }
    for (let p = 0; p < PRODUCTS; p++) {
      let dot = itemBias[p]!;
      for (let d = 0; d < DIM; d++) dot += userVec[d]! * itemVecs[p * DIM + d]!;
      scores[p] = dot;
    }
    await push(keys.user(version, `u${u}`), encodeScored(topK(scores, USER_TOP_K)));
    if ((u + 1) % 5000 === 0) console.log(`  users ${u + 1}/${USERS}`);
  }
  await flush(redis, batch);

  await redis.hset(keys.meta(version), {
    products: PRODUCTS,
    brands: BRANDS,
    users: USERS,
    loadedAt: new Date().toISOString(),
  });

  // Atomic cutover. Readers resolve reco:active, so they switch versions in one step.
  const previousActive = await redis.get(ACTIVE_VERSION_KEY);
  const staleVersion = await redis.get(PREVIOUS_VERSION_KEY);
  const tx = redis.multi().set(ACTIVE_VERSION_KEY, version);
  if (previousActive) tx.set(PREVIOUS_VERSION_KEY, previousActive);
  await tx.exec();
  console.log(`active version -> ${version} (previous: ${previousActive ?? "none"})`);

  if (staleVersion && staleVersion !== previousActive && staleVersion !== version) {
    const n = await deleteVersion(redis, staleVersion);
    console.log(`deleted superseded version ${staleVersion} (${n} keys)`);
  }
  console.log(`done in ${((Date.now() - started) / 1000).toFixed(1)}s`);
}

async function rollback(redis: Redis) {
  const [active, previous] = await redis.mget(ACTIVE_VERSION_KEY, PREVIOUS_VERSION_KEY);
  if (!previous) throw new Error("no previous version to roll back to");
  await redis.multi().set(ACTIVE_VERSION_KEY, previous).set(PREVIOUS_VERSION_KEY, active ?? "").exec();
  console.log(`rolled back: active ${active} -> ${previous}`);
}

const redis = new Redis(process.env.VALKEY_URL ?? "redis://127.0.0.1:6379", { maxRetriesPerRequest: 3 });
try {
  if (process.argv.includes("--rollback")) await rollback(redis);
  else await load(redis);
} finally {
  await redis.quit();
}
