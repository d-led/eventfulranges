// backoff.js — exponential reconnect scheduling for the demo UI.
export const RECONNECT_BASE_MS = 1000;
export const RECONNECT_MAX_MS = 30000;

// delayFor returns the wait before the attempt-th reconnect (0-based):
// 1s, 2s, 4s, … capped at 30s, so a dead server is retried at most twice a
// minute once the cap is reached.
export function delayFor(attempt) {
  return Math.min(
    RECONNECT_BASE_MS * 2 ** Math.max(0, attempt),
    RECONNECT_MAX_MS,
  );
}
