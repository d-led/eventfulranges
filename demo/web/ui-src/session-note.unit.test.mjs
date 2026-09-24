import { test } from 'node:test';
import assert from 'node:assert/strict';
import { presenceLine, sessionNote } from './session-note.js';

test('the served build promises a shared model', () => {
  const note = sessionNote(false);
  assert.match(note, /Go hub/);
  assert.match(note, /everyone who opens this link/);
});

test('the browser-only build promises nothing shared', () => {
  const note = sessionNote(true);
  assert.match(note, /No server/);
  assert.match(note, /nothing syncs/);
  assert.doesNotMatch(note, /everyone|share link|converge/i);
});

test('the served build counts the viewers of the session', () => {
  assert.equal(
    presenceLine(false, { clients: 2, total: 5, clientID: 'ABCDE' }),
    '2 here · 5 connected · you are ABCDE',
  );
});

test('the browser-only build says the page is on its own', () => {
  assert.equal(
    presenceLine(true, { clients: 1, total: 1, clientID: 'ABCDE' }),
    'server-free build · this page only · you are ABCDE',
  );
  assert.equal(presenceLine(true), 'server-free build · this page only');
});
