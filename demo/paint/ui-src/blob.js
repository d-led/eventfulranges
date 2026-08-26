// Pure, environment-free helpers for the share-link blob. The browser gzips
// the drawing; this module encodes the gzipped bytes in base 36 — the most
// compact case-insensitive alphanumeric numeral system — so a whole board
// fits in a share URL without padding or reserved characters.

const ALPHABET = '0123456789abcdefghijklmnopqrstuvwxyz';

// encodeBase36 maps bytes to a base-36 string. The first byte must be non-zero
// (gzip's 0x1f magic guarantees this), so a big-integer round trip cannot
// lose a leading zero byte.
export function encodeBase36(bytes) {
  let n = 0n;
  for (const b of bytes) n = (n << 8n) | BigInt(b);
  if (n === 0n) return '0';
  let out = '';
  while (n > 0n) {
    out = ALPHABET[Number(n % 36n)] + out;
    n /= 36n;
  }
  return out;
}

export function decodeBase36(text) {
  let n = 0n;
  for (const ch of text.toLowerCase()) {
    const d = ALPHABET.indexOf(ch);
    if (d < 0) throw new Error(`invalid base-36 digit "${ch}"`);
    n = n * 36n + BigInt(d);
  }
  const bytes = [];
  while (n > 0n) {
    bytes.unshift(Number(n & 0xffn));
    n >>= 8n;
  }
  return new Uint8Array(bytes.length ? bytes : [0]);
}

// serializeBoxes and parseBoxes define the snapshot carried in a share link:
// the materialized view, as aligned boxes. The log (JSONL) is exported
// separately and remains the source of truth.
export function serializeBoxes(boxes) {
  return JSON.stringify({ version: 1, boxes });
}

export function parseBoxes(text) {
  const parsed = JSON.parse(text);
  if (!parsed || !Array.isArray(parsed.boxes)) throw new Error('invalid snapshot');
  return parsed.boxes;
}
