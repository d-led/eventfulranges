// How a session's compaction is named on screen. The mode is chosen once, when
// the session starts, and the boxes alone do not reveal it — the same shell is
// a fine grid under canonical and a few slabs under merge adjacent — so the
// view has to say which one produced what is on screen.
//
// Two registers, because the two places that name the mode have very different
// room: `label` for the readout over the canvas, where everything has to fit on
// one line, and `detail` for the panel, where there is space to say what the
// strategy does to the boxes.
//
// The names must be the ones the hub reports in its view (demo/web/hub.go);
// compaction.unit.test.mjs keeps them in step with the New-session dialog,
// which is where a new mode would first appear.

export const COMPACTION_MODES = {
  canonical: {
    label: 'Compaction: canonical — every box kept',
    detail: 'Compaction: canonical — every box is kept exactly as materialized.',
  },
  merge: {
    label: 'Compaction: merge adjacent — touching boxes joined',
    detail: 'Compaction: merge adjacent — touching boxes are joined into larger ones.',
  },
  partition: {
    label: 'Compaction: partition — overlaps split apart',
    detail: 'Compaction: partition — overlaps are split so each point appears in exactly one rectangle.',
  },
  'partition-merge': {
    label: 'Compaction: partition + merge — split, then joined',
    detail: 'Compaction: partition + merge — overlaps are split, then touching boxes are joined (disjoint and compact).',
  },
};

// The hub answers an unrecognized ?compact= with a canonical session, so a
// readout that invented a mode for the same parameter would describe a session
// nobody is running.
const FALLBACK = 'canonical';

// compactionFor resolves a mode name to the mode that is actually running and
// its wording.
export function compactionFor(mode) {
  const running = COMPACTION_MODES[mode] ? mode : FALLBACK;
  return { mode: running, ...COMPACTION_MODES[running] };
}
