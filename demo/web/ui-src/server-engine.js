// The server-backed session engine: the page talks over a WebSocket to the Go
// visualizer, where every browser that opens the same share link converges on
// one shared model. This is the default when the UI is served by the Go app.
import { delayFor } from './backoff.js';

export function createServerEngine({ onMessage, onStatus, onOnline, onFirstSync }) {
  let socket = null;
  let online = false;
  let reconnectAttempts = 0;
  let reconnectTimer = null;

  function clearReconnectTimer() {
    if (reconnectTimer !== null) {
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
  }

  // scheduleReconnect queues one retry with exponential back-off: 1s, 2s, 4s,
  // … capped at 30s, so a dead server is retried at most twice a minute once
  // the cap is reached.
  function scheduleReconnect() {
    if (reconnectTimer !== null) return;
    const delay = delayFor(reconnectAttempts);
    reconnectAttempts += 1;
    const secs = Math.round(delay / 1000);
    onStatus(`disconnected — retrying in ${secs}s`);
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null;
      connect();
    }, delay);
  }

  function connect() {
    if (socket && socket.readyState === WebSocket.CONNECTING) return;
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    const params = new URLSearchParams(location.search);
    const session = params.get('s');
    if (!session) {
      // A page without a session id (e.g. an old bookmark) mints one instead
      // of opening a socket the server would have to reject.
      location.replace('/ui/');
      return;
    }
    const compact = params.get('compact') || '';
    const compactQuery = compact ? `&compact=${encodeURIComponent(compact)}` : '';
    const ws = new WebSocket(
      `${proto}://${location.host}/ws?s=${encodeURIComponent(session)}${compactQuery}`,
    );
    socket = ws;

    ws.onopen = () => {
      online = true;
      reconnectAttempts = 0;
      clearReconnectTimer();
      onOnline(true);
      onStatus('connected — edits are shared live');
      onFirstSync();
    };
    ws.onclose = () => {
      online = false;
      onOnline(false);
      scheduleReconnect();
    };
    ws.onerror = () => onStatus('connection error');

    ws.onmessage = (ev) => {
      let msg;
      try {
        msg = JSON.parse(ev.data);
      } catch {
        return;
      }
      onMessage(msg);
    };
  }

  return {
    start: connect,
    send(op) {
      if (!online || !socket || socket.readyState !== WebSocket.OPEN) {
        onStatus('disconnected — reconnect to edit');
        return;
      }
      socket.send(JSON.stringify(op));
    },
    // reconnectNow skips the back-off: the banner button forces an immediate
    // fresh connection attempt.
    reconnect() {
      clearReconnectTimer();
      reconnectAttempts = 0;
      if (socket && socket.readyState !== WebSocket.CLOSED) {
        socket.onclose = null; // this close is ours; connect() opens a fresh socket
        socket.close();
      }
      socket = null;
      connect();
    },
    // Test seam: Playwright closes the live socket through this to exercise
    // the reconnection flow. It is inert in normal use.
    close() {
      if (socket && socket.readyState !== WebSocket.CLOSED) socket.close();
    },
    // A fresh session is a fresh share link under the Go server, which mints
    // the session id on redirect.
    newSessionURL(dims, compact) {
      return compact
        ? `/ui/?dims=${dims}&compact=${encodeURIComponent(compact)}`
        : `/ui/?dims=${dims}`;
    },
  };
}
