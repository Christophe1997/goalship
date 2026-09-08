// goalship review's front end: vanilla JS, no framework, no build step —
// everything here ships as the literal bytes embedded by
// internal/reviewserver's //go:embed assets.
//
// Security note: ticket-supplied text (title, description,
// acceptance_criteria, deps) is rendered exclusively via .textContent or
// <input>.value assignment, never innerHTML/insertAdjacentHTML/
// document.write or string-concatenated markup. That alone renders any
// HTML/script payload in ticket content as inert text — no sanitizer
// library needed.

// document.currentScript is only non-null during this script's own
// synchronous top-level execution — it becomes null inside any callback,
// including the very next microtask. The token lives solely in this
// script's own src query string (native EventSource can't set custom
// headers, so every request authenticates via ?token= instead), so it must
// be captured right here, not inside DOMContentLoaded or any async
// handler.
const TOKEN = new URL(document.currentScript.src).searchParams.get('token');

// Fixed polling fallback interval. fsnotify events can be coalesced or
// dropped, and a network-mounted .goalship/ may not support filesystem
// notifications at all, so this fallback runs independently of SSE rather
// than only kicking in when SSE visibly fails. 4s is short enough that a
// missed SSE push is barely noticeable to an operator watching the page,
// long enough not to hammer the server.
const POLL_INTERVAL_MS = 4000;

function apiURL(path) {
  const u = new URL(path, location.origin);
  u.searchParams.set('token', TOKEN);
  return u.toString();
}

async function fetchTickets() {
  const res = await fetch(apiURL('/api/tickets'));
  if (!res.ok) throw new Error('GET /api/tickets: ' + res.status);
  return res.json();
}

async function fetchStatus() {
  const res = await fetch(apiURL('/api/status'));
  if (!res.ok) throw new Error('GET /api/status: ' + res.status);
  return res.json();
}

// ---- Application state ----

let reviewState = 'pending';
let lastKnownReviewUpdatedAt = '';

// ticketCards holds one persistent DOM record per ticket id across
// re-renders, so a live-refresh landing while a form is open in-edit never
// has to choose between "clobber the DOM node" and "clobber the operator's
// typed content" — the node survives, and updateCardFields is simply never
// called on a dirty card's inputs.
const ticketCards = new Map();
// dirty holds ticket ids with an open, unsaved edit. A live-refresh event
// (SSE or poll) must never overwrite such a form; see applyTicketsUpdate.
const dirty = new Set();
// pendingUpdates holds the freshest server-side ticket data received while
// its id was dirty, applied once the operator saves or cancels.
const pendingUpdates = new Map();

let eventSource = null;
let pollTimer = null;
let statusTimer = null;

function isReadOnly() {
  return reviewState !== 'pending';
}

// ---- Shell construction (built once, via createElement only) ----

let bannerEl, statusLineEl, ticketsEl;
let rejectNotesEl, rejectBtn, withdrawBtn, approveBtn;

function buildShell() {
  const app = document.getElementById('app');
  app.textContent = ''; // clears the static "Loading review…" placeholder

  bannerEl = document.createElement('div');
  bannerEl.id = 'banner';
  bannerEl.hidden = true;

  statusLineEl = document.createElement('div');
  statusLineEl.id = 'status-line';
  statusLineEl.hidden = true;

  const controls = document.createElement('section');
  controls.id = 'controls';

  rejectNotesEl = document.createElement('textarea');
  rejectNotesEl.id = 'reject-notes';
  rejectNotesEl.placeholder = 'Rejection notes';

  rejectBtn = document.createElement('button');
  rejectBtn.type = 'button';
  rejectBtn.textContent = 'Reject';
  rejectBtn.addEventListener('click', onRejectClick);

  withdrawBtn = document.createElement('button');
  withdrawBtn.type = 'button';
  withdrawBtn.textContent = 'Withdraw rejection';
  withdrawBtn.addEventListener('click', onWithdrawClick);

  approveBtn = document.createElement('button');
  approveBtn.type = 'button';
  approveBtn.textContent = 'Approve';
  approveBtn.addEventListener('click', onApproveClick);

  controls.append(rejectNotesEl, rejectBtn, withdrawBtn, approveBtn);

  ticketsEl = document.createElement('div');
  ticketsEl.id = 'tickets';

  app.append(bannerEl, statusLineEl, controls, ticketsEl);
}

function showStatus(msg) {
  statusLineEl.textContent = msg;
  statusLineEl.hidden = false;
  clearTimeout(statusTimer);
  statusTimer = setTimeout(() => { statusLineEl.hidden = true; }, 5000);
}

// renderBanner shows an explicit, plain-language banner while the graph is
// read-only, and hides it the moment review_state moves back to pending —
// updated from every live-refresh signal (SSE or poll), never only on
// manual reload.
function renderBanner() {
  if (reviewState === 'rejected') {
    bannerEl.hidden = false;
    bannerEl.className = 'banner banner-rejected';
    bannerEl.textContent =
      'Regeneration pending: this ticket graph was rejected. Editing is locked until a newly regenerated graph lands.';
  } else if (reviewState === 'approved') {
    bannerEl.hidden = false;
    bannerEl.className = 'banner banner-approved';
    bannerEl.textContent = 'This run has been approved. The review session has ended — you may close this page.';
  } else {
    bannerEl.hidden = true;
    bannerEl.textContent = '';
  }
}

function renderControls() {
  rejectBtn.disabled = reviewState !== 'pending';
  rejectNotesEl.disabled = reviewState !== 'pending';
  withdrawBtn.disabled = reviewState !== 'rejected';
  approveBtn.disabled = reviewState !== 'pending';
}

// ---- Ticket cards ----

function labeled(labelText, field) {
  const label = document.createElement('label');
  const span = document.createElement('span');
  span.textContent = labelText;
  label.append(span, field);
  return label;
}

function buildTicketCard(id) {
  const root = document.createElement('article');
  root.className = 'ticket';

  const header = document.createElement('div');
  header.className = 'ticket-header';
  const idEl = document.createElement('span');
  idEl.className = 'ticket-id';
  idEl.textContent = id;
  const statusEl = document.createElement('span');
  statusEl.className = 'ticket-status';
  header.append(idEl, statusEl);

  const titleInput = document.createElement('input');
  titleInput.type = 'text';

  const descInput = document.createElement('textarea');
  descInput.rows = 3;

  const acInput = document.createElement('textarea');
  acInput.rows = 3;

  const priorityInput = document.createElement('input');
  priorityInput.type = 'number';

  const depsInput = document.createElement('input');
  depsInput.type = 'text';

  const queuedNote = document.createElement('p');
  queuedNote.className = 'queued-note';
  queuedNote.textContent = 'A newer version arrived while you were editing. Save or Cancel to load it.';
  queuedNote.hidden = true;

  const errorNote = document.createElement('p');
  errorNote.className = 'field-error';
  errorNote.hidden = true;

  const saveBtn = document.createElement('button');
  saveBtn.type = 'button';
  saveBtn.textContent = 'Save';

  const cancelBtn = document.createElement('button');
  cancelBtn.type = 'button';
  cancelBtn.textContent = 'Cancel';

  const actions = document.createElement('div');
  actions.className = 'ticket-actions';
  actions.append(saveBtn, cancelBtn);

  const form = document.createElement('div');
  form.className = 'ticket-form';
  form.append(
    labeled('Title', titleInput),
    labeled('Description', descInput),
    labeled('Acceptance Criteria', acInput),
    labeled('Priority', priorityInput),
    labeled('Deps (comma-separated ticket ids)', depsInput),
    queuedNote,
    errorNote,
    actions,
  );

  root.append(header, form);

  const card = {
    root, statusEl, titleInput, descInput, acInput, priorityInput, depsInput,
    queuedNote, errorNote, saveBtn, cancelBtn, saved: null,
  };

  const markDirty = () => dirty.add(id);
  titleInput.addEventListener('input', markDirty);
  descInput.addEventListener('input', markDirty);
  acInput.addEventListener('input', markDirty);
  priorityInput.addEventListener('input', markDirty);
  depsInput.addEventListener('input', markDirty);

  saveBtn.addEventListener('click', () => saveTicket(id));
  cancelBtn.addEventListener('click', () => cancelEdit(id));

  return card;
}

// updateCardFields overwrites a card's form values from server data. Must
// never be called on a card whose id is in `dirty` — callers are
// responsible for that check (see renderTickets).
function updateCardFields(card, t) {
  card.statusEl.textContent = t.status;
  card.titleInput.value = t.title;
  card.descInput.value = t.description;
  card.acInput.value = t.acceptance_criteria;
  card.priorityInput.value = String(t.priority);
  card.depsInput.value = t.deps.join(', ');
  card.saved = t;
}

function setCardReadOnly(card, readOnly) {
  card.titleInput.disabled = readOnly;
  card.descInput.disabled = readOnly;
  card.acInput.disabled = readOnly;
  card.priorityInput.disabled = readOnly;
  card.depsInput.disabled = readOnly;
  card.saveBtn.disabled = readOnly;
}

function renderTickets(tickets) {
  const seen = new Set();
  for (const t of tickets) {
    seen.add(t.id);
    let card = ticketCards.get(t.id);
    if (!card) {
      card = buildTicketCard(t.id);
      ticketCards.set(t.id, card);
      ticketsEl.appendChild(card.root);
    }
    if (dirty.has(t.id)) {
      // Never touch this card's input values — an open, unsaved edit is
      // never silently overwritten by a live-refresh event.
      card.queuedNote.hidden = !pendingUpdates.has(t.id);
    } else {
      updateCardFields(card, t);
      card.queuedNote.hidden = true;
    }
    setCardReadOnly(card, isReadOnly());
  }
  for (const [id, card] of ticketCards) {
    if (!seen.has(id)) {
      card.root.remove();
      ticketCards.delete(id);
      dirty.delete(id);
      pendingUpdates.delete(id);
    }
  }
}

function parseDeps(raw) {
  return raw.split(',').map((s) => s.trim()).filter(Boolean);
}

async function saveTicket(id) {
  const card = ticketCards.get(id);
  if (!card) return;
  card.errorNote.hidden = true;

  const patch = {
    title: card.titleInput.value,
    description: card.descInput.value,
    acceptance_criteria: card.acInput.value,
    priority: Number(card.priorityInput.value) || 0,
    deps: parseDeps(card.depsInput.value),
  };

  try {
    const res = await fetch(apiURL('/api/tickets/' + encodeURIComponent(id)), {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(patch),
    });
    if (!res.ok) {
      // The graph may have just flipped read-only (a rejection landing
      // mid-edit): the operator's typed content must stay visible exactly
      // as they left it, so the form inputs are never touched here.
      const text = await res.text();
      card.errorNote.textContent = 'Save failed (' + res.status + '): ' + text;
      card.errorNote.hidden = false;
      return;
    }
    const saved = await res.json();
    dirty.delete(id);
    const queued = pendingUpdates.get(id);
    pendingUpdates.delete(id);
    updateCardFields(card, queued || saved);
    card.queuedNote.hidden = true;
  } catch (e) {
    card.errorNote.textContent = 'Save failed: ' + e.message;
    card.errorNote.hidden = false;
  }
}

function cancelEdit(id) {
  const card = ticketCards.get(id);
  if (!card) return;
  dirty.delete(id);
  card.errorNote.hidden = true;
  const queued = pendingUpdates.get(id);
  pendingUpdates.delete(id);
  if (queued || card.saved) {
    updateCardFields(card, queued || card.saved);
  }
  card.queuedNote.hidden = true;
}

// ---- Live refresh (SSE + poll) ----

// applyTicketsUpdate is the single entry point every live-refresh signal
// (SSE message or poll-detected change) funnels through: any ticket
// currently dirty gets its incoming data queued instead of applied, so an
// open unsaved edit is never silently overwritten.
function applyTicketsUpdate(tickets) {
  for (const t of tickets) {
    if (dirty.has(t.id)) {
      pendingUpdates.set(t.id, t);
    } else {
      pendingUpdates.delete(t.id);
    }
  }
  renderTickets(tickets);
}

async function onLiveRefreshSignal(status) {
  try {
    if (!status) status = await fetchStatus();
    lastKnownReviewUpdatedAt = status.review_updated_at;
    reviewState = status.review_state;
    renderBanner();
    renderControls();

    const tickets = await fetchTickets();
    applyTicketsUpdate(tickets);
  } catch (e) {
    console.error('live refresh failed:', e);
  }
}

function connectSSE() {
  eventSource = new EventSource(apiURL('/api/events'));
  eventSource.onmessage = () => onLiveRefreshSignal();
  // onerror is intentionally a no-op: EventSource auto-reconnects on a
  // transient failure, and the poll below covers the gap either way — SSE
  // is a latency optimization, not the sole detection mechanism.
  eventSource.onerror = () => {};
}

async function pollStatus() {
  try {
    const status = await fetchStatus();
    if (status.review_updated_at !== lastKnownReviewUpdatedAt) {
      await onLiveRefreshSignal(status);
    }
  } catch (e) {
    console.error('poll status failed:', e);
  }
}

function stopLiveRefresh() {
  if (eventSource) {
    eventSource.close();
    eventSource = null;
  }
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
}

// ---- Review-decision actions ----

function applyDecision(decision) {
  reviewState = decision.review_state;
  lastKnownReviewUpdatedAt = decision.review_updated_at;
  renderBanner();
  renderControls();
}

async function onRejectClick() {
  try {
    const res = await fetch(apiURL('/api/reject'), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ notes: rejectNotesEl.value }),
    });
    if (!res.ok) {
      showStatus('Reject failed: ' + res.status);
      return;
    }
    applyDecision(await res.json());
    showStatus('Rejected. Waiting for a regenerated graph.');
  } catch (e) {
    showStatus('Reject failed: ' + e.message);
  }
}

async function onWithdrawClick() {
  try {
    const res = await fetch(apiURL('/api/withdraw'), { method: 'POST' });
    if (!res.ok) {
      showStatus('Withdraw failed: ' + res.status);
      return;
    }
    applyDecision(await res.json());
    showStatus('Rejection withdrawn.');
  } catch (e) {
    showStatus('Withdraw failed: ' + e.message);
  }
}

async function onApproveClick() {
  if (!confirm('Approve this run? This ends the review session.')) return;
  try {
    const res = await fetch(apiURL('/api/approve'), { method: 'POST' });
    if (!res.ok) {
      showStatus('Approve failed: ' + res.status);
      return;
    }
    applyDecision(await res.json());
    showStatus('Approved. The review session is ending.');
    stopLiveRefresh();
  } catch (e) {
    showStatus('Approve failed: ' + e.message);
  }
}

// ---- Init ----

async function init() {
  buildShell();
  try {
    const [status, tickets] = await Promise.all([fetchStatus(), fetchTickets()]);
    reviewState = status.review_state;
    lastKnownReviewUpdatedAt = status.review_updated_at;
    renderBanner();
    renderControls();
    renderTickets(tickets);
  } catch (e) {
    showStatus('Failed to load review data: ' + e.message);
  }
  connectSSE();
  pollTimer = setInterval(pollStatus, POLL_INTERVAL_MS);
}

init();
