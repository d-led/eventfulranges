// What the panel says about where a session lives.
//
// The two builds look identical but promise different things. The served one
// shares every edit through the Go hub, so a share link really does put several
// people on one model. The browser-only build runs the engine inside the page
// (WebAssembly), which means nothing syncs to another browser, tab or person —
// the copy has to say so, or a share link reads like an invitation to a shared
// session that does not exist.

const NOTE = {
  server:
    'Served by the Go hub: everyone who opens this link folds the same operation log, '
    + 'so all screens converge. A session expires after a day of silence.',
  local:
    'No server: the engine runs in this page, so nothing syncs to another browser, tab '
    + 'or person. This session lives in this browser’s local copy, which survives a reload.',
};

export function sessionNote(isLocal) {
  return isLocal ? NOTE.local : NOTE.server;
}

// presenceLine names who is in the session: the hub counts viewers over the
// socket, while the browser-only build has exactly this page and nobody else.
export function presenceLine(isLocal, { clients = 0, total = 0, clientID = '' } = {}) {
  const me = clientID ? ` · you are ${clientID}` : '';
  return isLocal
    ? `server-free build · this page only${me}`
    : `${clients} here · ${total} connected${me}`;
}
