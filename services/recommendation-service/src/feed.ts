import type { Scored } from "./codec.js";

export type Source = "personalized" | "similar" | "popular" | "static";
export type Variant = "control" | "diverse" | "none";

export interface FeedItem {
  productId: number;
  brandId: number;
  score: number;
  source: Source;
}

export interface CandidateList {
  source: Source;
  items: readonly Scored[];
}

export interface AssembleInput {
  /** Ordered fallback chain. Earlier lists win; later lists only backfill. */
  chain: readonly CandidateList[];
  limit: number;
  exclude?: ReadonlySet<number>;
  /** Brand lookup; returns -1 for unknown products (no catalog loaded). */
  brandOf: (productId: number) => number;
  /** Max items per brand. null disables the diversity constraint (control variant). */
  brandCap: number | null;
}

export interface AssembleResult {
  items: FeedItem[];
  /** Source of the first item, i.e. the deepest fallback stage that actually served the feed. */
  primarySource: Source;
  /** True when anything after the first non-empty list contributed items. */
  backfilled: boolean;
}

/**
 * Feed assembly: walk the fallback chain in order, dedupe across lists, drop excluded ids, and
 * greedily enforce a soft per-brand cap. Items that would break the cap are parked in an overflow
 * list and only used if the feed would otherwise be short, so diversity never costs feed length.
 *
 * O(total candidates); no sorting, because each upstream list is already ranked by the model.
 */
export function assembleFeed(input: AssembleInput): AssembleResult {
  const { chain, limit, exclude, brandOf, brandCap } = input;
  const seen = new Set<number>(exclude);
  const perBrand = new Map<number, number>();
  const items: FeedItem[] = [];
  const overflow: FeedItem[] = [];
  let firstSource: Source | null = null;
  let backfilled = false;

  for (const list of chain) {
    if (items.length >= limit) break;
    let contributed = false;
    for (const { id, score } of list.items) {
      if (items.length >= limit) break;
      if (seen.has(id)) continue;
      seen.add(id);

      const brandId = brandOf(id);
      const item: FeedItem = { productId: id, brandId, score, source: list.source };
      const count = perBrand.get(brandId) ?? 0;
      if (brandCap !== null && brandId >= 0 && count >= brandCap) {
        overflow.push(item);
        continue;
      }
      perBrand.set(brandId, count + 1);
      items.push(item);
      contributed = true;
    }
    if (contributed) {
      if (firstSource === null) firstSource = list.source;
      else backfilled = true;
    }
  }

  for (const item of overflow) {
    if (items.length >= limit) break;
    items.push(item);
  }

  return { items, primarySource: firstSource ?? "static", backfilled };
}

/** 32-bit FNV-1a. Stable across processes and languages, which is what bucketing needs. */
export function fnv1a(input: string): number {
  let hash = 0x811c9dc5;
  for (let i = 0; i < input.length; i++) {
    hash ^= input.charCodeAt(i);
    hash = Math.imul(hash, 0x01000193);
  }
  return hash >>> 0;
}

/**
 * Deterministic A/B assignment: the same user always lands in the same variant for a given
 * experiment, on every pod, without shared state. Salting with the experiment name keeps
 * buckets independent across experiments.
 */
export function assignVariant(userId: string | undefined, experiment: string, treatmentPercent: number): Variant {
  if (!userId) return "none";
  return fnv1a(`${experiment}:${userId}`) % 100 < treatmentPercent ? "diverse" : "control";
}

export function distinctBrands(items: readonly FeedItem[]): number {
  const set = new Set<number>();
  for (const item of items) if (item.brandId >= 0) set.add(item.brandId);
  return set.size;
}
