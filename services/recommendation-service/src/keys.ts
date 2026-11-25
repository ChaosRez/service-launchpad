/**
 * Valkey key layout shared by the reverse-ETL loader (seed.ts) and the serving path.
 *
 * Every model output is written under an immutable version prefix. The loader writes a full new
 * version, then flips the single `reco:active` pointer. Readers resolve the pointer first, so a
 * half-written load is never visible, and rollback is one SET back to the previous version.
 */
export const ACTIVE_VERSION_KEY = "reco:active";
export const PREVIOUS_VERSION_KEY = "reco:previous";

export const keys = {
  /** Matrix-factorization top-K for a user (binary scored list). */
  user: (version: string, userId: string) => `reco:${version}:u:${userId}`,
  /** Item-to-item neighbours from item embeddings (binary scored list). */
  similar: (version: string, productId: number) => `reco:${version}:sim:${productId}`,
  /** Global popularity list used for cold start and as filler (binary scored list). */
  popular: (version: string) => `reco:${version}:pop:global`,
  /** Brand id per product id (u16 array). */
  catalogBrands: (version: string) => `reco:${version}:catalog:brands`,
  /** Load metadata hash: counts and timestamps written by the loader. */
  meta: (version: string) => `reco:${version}:meta`,
  /** Pattern used to delete a superseded version. */
  versionPattern: (version: string) => `reco:${version}:*`,
};
