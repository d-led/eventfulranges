import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { COMPACTION_MODES, compactionFor } from './compaction.js';

test('every consolidation mode is named, distinctly', () => {
  const modes = Object.keys(COMPACTION_MODES);
  const labels = modes.map((mode) => compactionFor(mode).label);

  assert.equal(new Set(labels).size, modes.length, 'no two modes share a name');
  for (const label of labels) {
    assert.match(label, /^Compaction: \S/, `${label} reads as a readout of the running mode`);
  }
});

test('a mode the hub does not know reads as canonical', () => {
  // ?compact=typo opens a canonical session, so the readout must not claim
  // that something else is running.
  assert.deepEqual(compactionFor('typo'), compactionFor('canonical'));
});

// The dialog is where a new mode shows up first: an option without wording
// would leave the readout announcing canonical for a session that is not.
test('the New-session dialog offers exactly the modes that can be named', () => {
  const dialog = readFileSync(new URL('./index.html', import.meta.url), 'utf8');
  const start = dialog.indexOf('<select id="compact">');
  const select = dialog.slice(start, dialog.indexOf('</select>', start));
  const offered = [...select.matchAll(/<option value="([^"]*)"/g)]
    .map(([, value]) => value || 'canonical'); // the dialog's empty value means "no compaction"

  assert.deepEqual(offered.sort(), Object.keys(COMPACTION_MODES).sort());
});
