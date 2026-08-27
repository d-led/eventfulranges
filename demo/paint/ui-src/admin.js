// Admin page script: fetch the instance's sessions, storage, and users, and
// delete inactive sessions. The server gates every /admin route by the proxy
// email, so this page only ever renders data the caller is allowed to see.
const $ = (id) => document.getElementById(id);

async function load() {
  const res = await fetch("/admin/api/info");
  if (!res.ok) {
    flash(`could not load admin info (${res.status})`);
    return;
  }
  $("adminError").hidden = true;
  render(await res.json());
}

function render(info) {
  $("storageBytes").textContent = formatBytes(info.storageBytes);

  const users = $("users");
  users.replaceChildren();
  if (info.users.length === 0) {
    users.appendChild(listItem("no users yet", "empty"));
  } else {
    for (const email of info.users) users.appendChild(listItem(email));
  }

  const tbody = $("sessions");
  tbody.replaceChildren();
  if (info.sessions.length === 0) {
    tbody.appendChild(emptyRow("no sessions"));
    return;
  }
  for (const session of info.sessions) tbody.appendChild(sessionRow(session));
}

function listItem(text, className) {
  const li = document.createElement("li");
  li.textContent = text;
  if (className) li.className = className;
  return li;
}

function emptyRow(text) {
  const tr = document.createElement("tr");
  const td = document.createElement("td");
  td.colSpan = 4;
  td.textContent = text;
  td.className = "empty";
  tr.appendChild(td);
  return tr;
}

function sessionRow(session) {
  const tr = document.createElement("tr");
  tr.appendChild(cell(session.id));
  tr.appendChild(cell(formatBytes(session.bytes)));

  const clients = cell(String(session.clients));
  if (session.clients > 0) clients.className = "active";
  tr.appendChild(clients);

  const actions = document.createElement("td");
  const del = document.createElement("button");
  del.textContent = "Delete";
  del.disabled = session.clients > 0;
  del.title =
    session.clients > 0
      ? "connected clients prevent deletion"
      : "delete this inactive session";
  del.addEventListener("click", () => remove(session.id));
  actions.appendChild(del);
  tr.appendChild(actions);
  return tr;
}

function cell(text) {
  const td = document.createElement("td");
  td.textContent = text;
  return td;
}

async function remove(id) {
  const res = await fetch(`/admin/api/sessions/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
  if (res.ok) {
    await load();
    return;
  }
  const body = await res.json().catch(() => ({}));
  flash(`delete failed: ${body.error || res.status}`);
}

function flash(text) {
  const el = $("adminError");
  el.hidden = false;
  el.textContent = text;
}

function formatBytes(n) {
  if (n < 1024) return `${n} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let value = n;
  let i = -1;
  do {
    value /= 1024;
    i++;
  } while (value >= 1024 && i < units.length - 1);
  return `${value.toFixed(1)} ${units[i]}`;
}

$("refreshBtn").addEventListener("click", load);
load();
