import { test } from 'node:test';
import assert from 'node:assert/strict';
import { orthoFrustum, orthoHalf, perspDistance } from './camera.js';

test('orthoFrustum is symmetric around the camera origin', () => {
  const f = orthoFrustum(2, 1.5);
  assert.equal(f.left, -3);
  assert.equal(f.right, 3);
  assert.equal(f.top, 2);
  assert.equal(f.bottom, -2);
});

test('orthoFrustum frames the horizontal extent through the aspect', () => {
  const square = orthoFrustum(2, 1);
  const wide = orthoFrustum(2, 2);
  assert.equal(square.left, -2);
  assert.equal(square.right, 2);
  assert.equal(wide.left, -4);
  assert.equal(wide.right, 4);
});

test('orthoHalf frames both extents so nothing clips', () => {
  const bounds = { min: [100, 100, 0], max: [104, 104, 0.02] };
  assert.equal(orthoHalf(bounds, 1), 2); // square canvas: height dominates
  assert.equal(orthoHalf(bounds, 4), 2); // wider canvas: height still dominates
  assert.equal(orthoHalf(null, 1), 4); // empty scene fallback
});

test('orthoHalf never shrinks below the minimum framing height', () => {
  const tiny = { min: [0, 0, 0], max: [0.1, 0.1, 0.02] };
  assert.equal(orthoHalf(tiny, 1), 1);
});

test('perspDistance fits a sphere within the narrower field of view', () => {
  // A taller canvas narrows the horizontal field, so the camera steps back.
  const wideCanvas = perspDistance(1, 50, 2);
  const tallCanvas = perspDistance(1, 50, 0.5);
  assert.ok(wideCanvas > 0);
  assert.ok(tallCanvas > wideCanvas);
});
