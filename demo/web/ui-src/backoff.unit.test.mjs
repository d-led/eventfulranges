import { test } from 'node:test';
import assert from 'node:assert/strict';
import { delayFor, RECONNECT_BASE_MS, RECONNECT_MAX_MS } from './backoff.js';

test('first retry waits the base delay', () => {
  assert.equal(delayFor(0), RECONNECT_BASE_MS);
});

test('delay doubles each attempt until the cap', () => {
  assert.equal(delayFor(0), 1000);
  assert.equal(delayFor(1), 2000);
  assert.equal(delayFor(2), 4000);
  assert.equal(delayFor(3), 8000);
  assert.equal(delayFor(4), 16000);
  assert.equal(delayFor(5), RECONNECT_MAX_MS);
  assert.equal(delayFor(6), RECONNECT_MAX_MS);
});

test('negative attempts clamp to the base delay', () => {
  assert.equal(delayFor(-1), RECONNECT_BASE_MS);
});
