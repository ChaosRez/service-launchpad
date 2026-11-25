/**
 * Compact binary encoding for scored candidate lists stored in Valkey.
 *
 * Layout (little-endian):
 *   u8  format version (1)
 *   u16 entry count
 *   count x { u32 productId, f32 score }
 *
 * 200 candidates = 1,603 bytes, versus ~5 KB as JSON. Decoding is a tight loop over a
 * DataView with no JSON parse and no per-item string allocation. A production system would
 * likely use Protobuf/Avro for schema evolution; the fixed layout keeps this demo dependency-free
 * while still showing why binary payloads matter on the hot path.
 */
export const FORMAT_VERSION = 1;
const HEADER_BYTES = 3;
const ENTRY_BYTES = 8;

export interface Scored {
  id: number;
  score: number;
}

export function encodeScored(entries: readonly Scored[]): Buffer {
  if (entries.length > 0xffff) throw new RangeError("too many entries for u16 count");
  const buf = Buffer.allocUnsafe(HEADER_BYTES + entries.length * ENTRY_BYTES);
  buf.writeUInt8(FORMAT_VERSION, 0);
  buf.writeUInt16LE(entries.length, 1);
  let offset = HEADER_BYTES;
  for (const { id, score } of entries) {
    buf.writeUInt32LE(id, offset);
    buf.writeFloatLE(score, offset + 4);
    offset += ENTRY_BYTES;
  }
  return buf;
}

export function decodeScored(buf: Buffer): Scored[] {
  if (buf.length < HEADER_BYTES) throw new RangeError("payload too short");
  const version = buf.readUInt8(0);
  if (version !== FORMAT_VERSION) throw new RangeError(`unsupported format version ${version}`);
  const count = buf.readUInt16LE(1);
  if (buf.length !== HEADER_BYTES + count * ENTRY_BYTES) throw new RangeError("payload length mismatch");
  const out = new Array<Scored>(count);
  let offset = HEADER_BYTES;
  for (let i = 0; i < count; i++) {
    out[i] = { id: buf.readUInt32LE(offset), score: buf.readFloatLE(offset + 4) };
    offset += ENTRY_BYTES;
  }
  return out;
}

/** Catalog side table: brand id per product id, as a u16 array indexed by product id. */
export function encodeBrands(brands: Uint16Array): Buffer {
  return Buffer.from(brands.buffer, brands.byteOffset, brands.byteLength);
}

export function decodeBrands(buf: Buffer): Uint16Array {
  // Copy into an aligned buffer; Valkey replies are not guaranteed to be 2-byte aligned.
  const copy = new Uint8Array(buf.length);
  copy.set(buf);
  return new Uint16Array(copy.buffer, 0, Math.floor(buf.length / 2));
}
