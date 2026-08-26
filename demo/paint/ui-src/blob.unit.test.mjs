import { describe, it, expect } from 'vitest';
import { encodeBase36, decodeBase36, serializeBoxes, parseBoxes } from './blob.js';

describe('base36', () => {
  it('round-trips bytes with a non-zero first byte', () => {
    const bytes = new Uint8Array([0x1f, 0x8b, 0x08, 0x00, 0xab, 0xcd, 0xef]);
    const encoded = encodeBase36(bytes);
    expect(encoded).toMatch(/^[0-9a-z]+$/);
    expect([...decodeBase36(encoded)]).toEqual([...bytes]);
  });

  it('encodes zero as "0"', () => {
    expect(encodeBase36(new Uint8Array([0]))).toBe('0');
    expect([...decodeBase36('0')]).toEqual([0]);
  });

  it('is case-insensitive on decode', () => {
    const bytes = new Uint8Array([0x2a, 0x3b]);
    expect(decodeBase36(encodeBase36(bytes).toUpperCase())).toEqual(bytes);
  });

  it('rejects invalid digits', () => {
    expect(() => decodeBase36('zz!')).toThrow(/invalid/);
  });

  it('round-trips the snapshot shape', () => {
    const boxes = [
      { x: 0, y: 0, size: 4 },
      { x: 4, y: 0, size: 2 },
    ];
    expect(parseBoxes(serializeBoxes(boxes))).toEqual(boxes);
  });
});
