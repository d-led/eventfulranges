import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readout } from './readout.js';

test('reports every count the session passed through, in order', () => {
  // 27 unit cubes plus the carve-out, partitioned into 26 cells, merged to 8.
  assert.equal(readout({ ops: 28, cells: 26, ranges: 8 }), '28 ops → 26 partition cells → 8 ranges');
});

test('leaves out the partition cells when no partition ran', () => {
  assert.equal(readout({ ops: 28, cells: 0, ranges: 8 }), '28 ops → 8 ranges');
});

test('leaves out the cells when the partition is the result', () => {
  // partition mode keeps the cells, so cells and ranges are the same number.
  assert.equal(readout({ ops: 2, cells: 5, ranges: 5 }), '2 ops → 5 ranges');
});

test('counts a single thing in the singular', () => {
  assert.equal(readout({ ops: 1, cells: 0, ranges: 1 }), '1 op → 1 range');
});

test('reports an empty session as no ranges', () => {
  assert.equal(readout({}), '0 ranges');
});
