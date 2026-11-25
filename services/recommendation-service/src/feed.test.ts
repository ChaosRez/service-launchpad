import assert from "node:assert/strict";
import { test } from "node:test";

import { decodeBrands, decodeScored, encodeBrands, encodeScored } from "./codec.js";
import { assembleFeed, assignVariant, distinctBrands, type CandidateList } from "./feed.js";

const list = (source: CandidateList["source"], ids: number[]): CandidateList => ({
  source,
  items: ids.map((id, i) => ({ id, score: 1 - i / 100 })),
});
// brand = id / 10, so ids 0-9 are brand 0, 10-19 brand 1, ...
const brandOf = (id: number) => Math.floor(id / 10);

test("scored-list codec round-trips and rejects bad payloads", () => {
  const entries = [
    { id: 7, score: 0.5 },
    { id: 4_000_000_000, score: -1.25 },
  ];
  assert.deepEqual(decodeScored(encodeScored(entries)), entries);
  assert.equal(encodeScored(entries).length, 3 + 2 * 8);
  assert.throws(() => decodeScored(Buffer.from([2, 0, 0])), /version/);
  assert.throws(() => decodeScored(encodeScored(entries).subarray(0, 10)), /length/);
});

test("brand table codec survives an unaligned buffer", () => {
  const brands = Uint16Array.from([1, 2, 65535]);
  const unaligned = Buffer.concat([Buffer.from([0]), encodeBrands(brands)]).subarray(1);
  assert.deepEqual(Array.from(decodeBrands(unaligned)), [1, 2, 65535]);
});

test("primary list wins and popular only backfills, without duplicates", () => {
  const result = assembleFeed({
    chain: [list("personalized", [1, 2, 3]), list("popular", [3, 4, 5, 6])],
    limit: 5,
    brandOf,
    brandCap: null,
  });
  assert.deepEqual(result.items.map((i) => i.productId), [1, 2, 3, 4, 5]);
  assert.deepEqual(result.items.map((i) => i.source), ["personalized", "personalized", "personalized", "popular", "popular"]);
  assert.equal(result.primarySource, "personalized");
  assert.equal(result.backfilled, true);
});

test("cold start falls through to popular, then static", () => {
  const result = assembleFeed({
    chain: [list("popular", []), list("static", [0, 1, 2])],
    limit: 2,
    brandOf: () => -1,
    brandCap: 2,
  });
  assert.equal(result.primarySource, "static");
  assert.equal(result.backfilled, false);
  assert.deepEqual(result.items.map((i) => i.productId), [0, 1]);
});

test("excluded ids never appear", () => {
  const result = assembleFeed({
    chain: [list("similar", [1, 2, 3])],
    limit: 10,
    exclude: new Set([2]),
    brandOf,
    brandCap: null,
  });
  assert.deepEqual(result.items.map((i) => i.productId), [1, 3]);
});

test("brand cap spreads brands but never shortens the feed", () => {
  // All of brand 0 first, then brand 1, then brand 2.
  const candidates = list("personalized", [0, 1, 2, 3, 10, 11, 12, 20, 21]);
  const control = assembleFeed({ chain: [candidates], limit: 6, brandOf, brandCap: null });
  const diverse = assembleFeed({ chain: [candidates], limit: 6, brandOf, brandCap: 2 });

  assert.deepEqual(control.items.map((i) => i.productId), [0, 1, 2, 3, 10, 11]);
  assert.deepEqual(diverse.items.map((i) => i.productId), [0, 1, 10, 11, 20, 21]);
  assert.ok(distinctBrands(diverse.items) > distinctBrands(control.items));

  // Not enough other brands: overflow refills so the length is preserved.
  const short = assembleFeed({ chain: [list("personalized", [0, 1, 2, 3])], limit: 4, brandOf, brandCap: 2 });
  assert.equal(short.items.length, 4);
});

test("variant assignment is deterministic, salted, and roughly balanced", () => {
  assert.equal(assignVariant(undefined, "exp", 50), "none");
  assert.equal(assignVariant("u1", "exp", 50), assignVariant("u1", "exp", 50));
  assert.equal(assignVariant("u1", "exp", 0), "control");
  assert.equal(assignVariant("u1", "exp", 100), "diverse");

  let treated = 0;
  for (let i = 0; i < 10_000; i++) if (assignVariant(`u${i}`, "exp", 50) === "diverse") treated++;
  assert.ok(treated > 4_700 && treated < 5_300, `expected ~50% treated, got ${treated}`);
});
