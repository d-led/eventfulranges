import { test } from 'node:test';
import assert from 'node:assert/strict';
import { fadeOpacity } from './slice.js';

test('fadeOpacity is 0 outside and 1 deep inside the w-interval', () => {
  assert.equal(fadeOpacity(0, 3, -0.5), 0);
  assert.equal(fadeOpacity(0, 3, 0), 0);
  assert.equal(fadeOpacity(0, 3, 1.5), 1);
  assert.equal(fadeOpacity(0, 3, 3), 0);
  assert.equal(fadeOpacity(0, 3, 4), 0);
});

test('fadeOpacity ramps in and out instead of cutting hard', () => {
  const leading = fadeOpacity(0, 3, 0.3);
  const trailing = fadeOpacity(0, 3, 2.7);
  assert.ok(leading > 0 && leading < 1, 'leading edge is partially faded');
  assert.ok(trailing > 0 && trailing < 1, 'trailing edge is partially faded');
  assert.ok(fadeOpacity(0, 3, 0.3) < fadeOpacity(0, 3, 0.6), 'fade-in ramps up');
  assert.ok(fadeOpacity(0, 3, 2.7) < fadeOpacity(0, 3, 2.4), 'fade-out ramps down');
});
