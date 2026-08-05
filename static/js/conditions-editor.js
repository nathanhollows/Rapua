// Conditions editor — wires into the #edit-panel sidebar in layouts.templ.

// ── Vars ──────────────────────────────────────────────────────────────────

let _varsCache = null;

async function getVars() {
  if (_varsCache) return _varsCache;
  try {
    const r = await fetch('/admin/vars');
    _varsCache = await r.json();
  } catch {
    _varsCache = [];
  }
  return _varsCache;
}

// ── Constants ─────────────────────────────────────────────────────────────

const OPS = {
  bool:   [['','is set'], ['not','is not set']],
  int:    [['gte','is at least'], ['gt','is greater than'], ['lte','is at most'], ['lt','is less than'], ['eq','equals'], ['neq',"doesn't equal"]],
  string: [['eq','equals'], ['neq',"doesn't equal"], ['in','is one of'], ['not_in','is not one of']],
};

// ── State ─────────────────────────────────────────────────────────────────

let _uid = 0;
const uid = () => ++_uid;

// activeTarget: { type: 'block'|'location'|'group', id, slug, btnEl, allOf, anyOf, sets, isInteractive }
let activeTarget = null;

// Sets is served as a map {name: value}.
function parseSets(setsJSON) {
  if (!setsJSON || setsJSON === 'null') return [];
  let parsed;
  try { parsed = JSON.parse(setsJSON); } catch { return []; }
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return [];
  return Object.entries(parsed).map(([name, value]) => ({
    id: uid(), varName: name || '', value: value === undefined || value === null ? '' : String(value),
  }));
}

function newTarget(type, id, btnEl, whenJSON, setsJSON, isInteractive) {
  const when = whenJSON && whenJSON !== 'null' ? JSON.parse(whenJSON) : null;

  const allOf = (when?.all_of || []).map(c => ({
    id: uid(), varName: c.var || '', op: c.not ? 'not' : (c.op || ''), value: c.value || '', inValues: c.in_values || [],
  }));
  const anyOf = (when?.any_of || []).map(c => ({
    id: uid(), varName: c.var || '', op: c.not ? 'not' : (c.op || ''), value: c.value || '', inValues: c.in_values || [],
  }));

  return { type, id, btnEl, allOf, anyOf, sets: parseSets(setsJSON), isInteractive };
}

// setsMap collapses the editor rows into the map the server stores.
// Values are sent as typed — new rows are seeded with "true" rather than
// coercing a blank at save time, so what the author sees is what is written.
function setsMap(rows) {
  const out = {};
  rows.filter(r => r.varName).forEach(r => { out[r.varName] = r.value; });
  return out;
}

// ── Entry points ─────────────────────────────────────────────────────────

function openBlockConditions(event, btn) {
  event.stopPropagation();
  const blockId    = btn.dataset.blockId;
  const blockType  = btn.dataset.blockType;
  const interactive = btn.dataset.interactive === 'true';
  activeTarget = newTarget('block', blockId, btn, btn.dataset.when, btn.dataset.sets, interactive);
  activeTarget.blockType = blockType;
  _openPanel(btn.dataset.blockType + ' block');
}

function openLocationConditions(event, btn) {
  event.stopPropagation();
  activeTarget = newTarget('location', btn.dataset.locationSlug, btn, btn.dataset.when, null, false);
  _openPanel('Location');
}

function openGroupConditions(event, btn) {
  event.stopPropagation();
  activeTarget = newTarget('group', btn.dataset.groupId, btn, btn.dataset.when, null, false);
  _openPanel('Group');
}

function _openPanel(title) {
  document.getElementById('edit-panel-title').textContent = title;
  document.getElementById('edit-drawer').checked = true;
  _renderPanel();
  document.getElementById('edit-panel-save').addEventListener('click', saveConditions, { once: true });
}

// ── Render ────────────────────────────────────────────────────────────────

async function _renderPanel() {
  const t = activeTarget;
  if (!t) return;
  const vars = await getVars();
  const hasAny = t.allOf.length > 0 || t.anyOf.length > 0;
  let html = '';
  if (t.isInteractive) html += _renderSets(t, vars);
  html += _renderWhen(t, vars, hasAny);
  document.getElementById('edit-panel-body').innerHTML = html;
}

// ── Sets section ─────────────────────────────────────────────────────────

function _renderSets(t, _vars) {
  const rows = t.sets.map(row => `
    <div class="bg-base-200 rounded-lg px-3 pt-1.5 pb-2.5">
      <div class="flex items-center justify-between mb-1">
        <span class="text-xs text-base-content/40">Variable name</span>
        <button class="btn btn-ghost btn-xs btn-circle text-base-content/30 hover:text-error"
          onclick="condRemoveSets(${row.id})">✕</button>
      </div>
      <input type="text" placeholder="e.g. found_clue" class="input input-sm bg-base-100 border-0 shadow-none w-full font-mono"
        value="${esc(row.varName)}"
        oninput="condSetVarName(${row.id}, this.value)" />
      <div class="flex items-center gap-2 mt-1.5 pl-3">
        <span class="text-xs text-base-content/30">to</span>
        <input type="text" placeholder="true" class="input input-sm bg-base-100 border-0 shadow-none flex-1 min-w-0 font-mono"
          value="${esc(row.value)}"
          oninput="condSetVarValue(${row.id}, this.value)" />
      </div>
    </div>`).join('');

  return `<div>
    <div class="flex items-center mb-2">
      <span class="text-sm font-medium flex items-center gap-1.5">
        <svg xmlns="http://www.w3.org/2000/svg" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/></svg>
        On completion, set variable
      </span>
    </div>
    ${t.sets.length === 0
      ? `<div class="rounded-xl border-2 border-dashed border-base-300 p-6 text-center space-y-2">
           <p class="text-sm text-base-content/40">No variable set on completion.</p>
           <button onclick="condAddSets()" class="btn btn-sm btn-outline btn-primary">+ Set variable</button>
         </div>`
      : `<div class="space-y-2">${rows}
           <button onclick="condAddSets()" class="mt-1 text-xs text-base-content/40 hover:text-base-content/70 cursor-pointer block">+ Set another variable</button>
         </div>`}
  </div>`;
}

// ── When section ─────────────────────────────────────────────────────────

function _renderWhen(t, vars, hasAny) {
  const hasAllOf = t.allOf.length > 0;
  const hasAnyOf = t.anyOf.length > 0;

  return `<div>
    <div class="flex items-center justify-between mb-2">
      <span class="text-sm font-medium flex items-center gap-1.5">
        <svg xmlns="http://www.w3.org/2000/svg" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M2 12s3-7 10-7 10 7 10 7-3 7-10 7-10-7-10-7Z"/><circle cx="12" cy="12" r="3"/></svg>
        Show this when
      </span>
      ${hasAny ? `<button onclick="condClearAll()" class="btn btn-xs btn-ghost text-error/60">Clear all</button>` : ''}
    </div>
    ${!hasAny ? `
      <div class="rounded-xl border-2 border-dashed border-base-300 p-6 text-center space-y-2">
        <p class="text-sm text-base-content/40">Always visible to players.</p>
        <button onclick="condAddAllOf()" class="btn btn-sm btn-outline btn-primary">+ Add condition</button>
      </div>` : `
      <div class="space-y-4">
        <div>
          ${hasAllOf ? `<div class="space-y-2">${t.allOf.map(row => _condRow(row, 'condUpdateAllOf', 'condRemoveAllOf', vars)).join('')}</div>` : ''}
          <button onclick="condAddAllOf()" class="mt-2 text-xs text-base-content/40 hover:text-base-content/70 cursor-pointer block">
            ${hasAllOf ? '+ and also…' : '+ Add condition'}
          </button>
        </div>
        ${hasAnyOf ? `
          <div class="flex items-center gap-2 my-1">
            <div class="flex-1 border-t border-base-200"></div>
            <span class="text-xs text-base-content/40">and at least one of:</span>
            <div class="flex-1 border-t border-base-200"></div>
          </div>
          <div>
            <div class="space-y-2">${t.anyOf.map(row => _condRow(row, 'condUpdateAnyOf', 'condRemoveAnyOf', vars)).join('')}</div>
            <button onclick="condAddAnyOf()" class="mt-2 text-xs text-base-content/40 hover:text-base-content/70 cursor-pointer block">+ or also…</button>
          </div>` : ''}
        ${!hasAnyOf ? `<button onclick="condAddAnyOf()" class="text-xs text-base-content/30 hover:text-base-content/60 cursor-pointer block">+ Add OR group</button>` : ''}
      </div>`}
  </div>`;
}

function _condRow(row, updateFn, removeFn, vars) {
  const varDef = vars.find(v => v.name === row.varName);
  const type = varDef ? varDef.type : 'bool';
  const ops = OPS[type] || OPS.bool;
  const needsValue = row.op && row.op !== '' && row.op !== 'not';
  const isMulti = row.op === 'in' || row.op === 'not_in';

  const varGroups = vars.reduce((g, v) => { (g[v.group] = g[v.group] || []).push(v); return g; }, {});
  const varOpts = Object.entries(varGroups).map(([grp, gvars]) =>
    `<optgroup label="${grp}">${gvars.map(v =>
      `<option value="${v.name}" ${row.varName === v.name ? 'selected' : ''}>${v.name}</option>`
    ).join('')}</optgroup>`
  ).join('');

  let valueEl = '';
  if (needsValue && isMulti) {
    valueEl = `<input type="text" placeholder="a, b, c…" class="input input-sm input-bordered flex-1 min-w-0"
      value="${esc((row.inValues || []).join(', '))}"
      oninput="${updateFn}(${row.id},'inValues',this.value.split(',').map(s=>s.trim()).filter(Boolean))" />`;
  } else if (needsValue) {
    valueEl = `<input type="${type === 'int' ? 'number' : 'text'}" placeholder="value"
      class="input input-sm input-bordered w-24"
      value="${esc(row.value)}"
      oninput="${updateFn}(${row.id},'value',this.value)" />`;
  }

  return `
    <div class="bg-base-200 rounded-lg px-3 pt-1.5 pb-2">
      <div class="flex items-center gap-1 mb-0.5">
        <span class="text-xs text-base-content/40 shrink-0">If</span>
        <select class="select select-sm bg-base-100 border-0 shadow-none flex-1 min-w-0"
          onchange="${updateFn}(${row.id},'varName',this.value)">
          <option value="">— pick a variable —</option>${varOpts}
        </select>
        <button onclick="${removeFn}(${row.id})"
          class="btn btn-ghost btn-xs btn-circle text-base-content/30 hover:text-error shrink-0">✕</button>
      </div>
      <div class="flex items-center gap-2 pl-3">
        <span class="text-xs text-base-content/30">→</span>
        <select class="select select-sm bg-base-100 border-0 shadow-none"
          onchange="${updateFn}(${row.id},'op',this.value)">
          ${ops.map(([v, l]) => `<option value="${v}" ${row.op === v ? 'selected' : ''}>${l}</option>`).join('')}
        </select>
        ${valueEl}
      </div>
    </div>`;
}

// ── Mutations ─────────────────────────────────────────────────────────────

function condAddSets() { activeTarget.sets.push({ id: uid(), varName: '', value: 'true' }); _renderPanel(); }
function condRemoveSets(id) { activeTarget.sets = activeTarget.sets.filter(r => r.id !== id); _renderPanel(); }
function condSetVarName(id, v) { const r = activeTarget.sets.find(r => r.id === id); if (r) r.varName = v; }
function condSetVarValue(id, v) { const r = activeTarget.sets.find(r => r.id === id); if (r) r.value = v; }

function condAddAllOf() { activeTarget.allOf.push({ id: uid(), varName: '', op: '', value: '', inValues: [] }); _renderPanel(); }
function condRemoveAllOf(id) { activeTarget.allOf = activeTarget.allOf.filter(r => r.id !== id); _renderPanel(); }
function condUpdateAllOf(id, f, v) {
  const r = activeTarget.allOf.find(r => r.id === id);
  if (!r) return;
  const needsRerender = _applyCondUpdate(r, f, v);
  if (needsRerender) _renderPanel();
}

function condAddAnyOf() { activeTarget.anyOf.push({ id: uid(), varName: '', op: '', value: '', inValues: [] }); _renderPanel(); }
function condRemoveAnyOf(id) { activeTarget.anyOf = activeTarget.anyOf.filter(r => r.id !== id); _renderPanel(); }
function condUpdateAnyOf(id, f, v) {
  const r = activeTarget.anyOf.find(r => r.id === id);
  if (!r) return;
  const needsRerender = _applyCondUpdate(r, f, v);
  if (needsRerender) _renderPanel();
}

function condClearAll() { activeTarget.allOf = []; activeTarget.anyOf = []; _renderPanel(); }

// Returns true when a structural change requires re-rendering (var or op change shows/hides value input).
function _applyCondUpdate(r, f, v) {
  if (f === 'varName') {
    r.varName = v;
    const varDef = (_varsCache || []).find(x => x.name === v);
    r.op = varDef?.type === 'int' ? 'gte' : '';
    return true;
  }
  if (f === 'op') {
    r.op = v;
    return true; // may show/hide value input
  }
  r[f] = v;
  return false;
}

// ── Save ──────────────────────────────────────────────────────────────────

async function saveConditions() {
  const t = activeTarget;
  if (!t) return;

  // URLSearchParams sends as application/x-www-form-urlencoded, which r.ParseForm() handles.
  const body = new URLSearchParams();

  // Build when_clause JSON.
  const allOf = t.allOf
    .filter(r => r.varName)
    .map(r => {
      const c = { var: r.varName };
      if (r.op === 'not') { c.not = true; }
      else if (r.op === 'in' || r.op === 'not_in') { c.op = r.op; c.in_values = r.inValues || []; }
      else if (r.op) { c.op = r.op; c.value = r.value || ''; }
      return c;
    });
  const anyOf = t.anyOf
    .filter(r => r.varName)
    .map(r => {
      const c = { var: r.varName };
      if (r.op === 'not') { c.not = true; }
      else if (r.op === 'in' || r.op === 'not_in') { c.op = r.op; c.in_values = r.inValues || []; }
      else if (r.op) { c.op = r.op; c.value = r.value || ''; }
      return c;
    });

  if (allOf.length > 0 || anyOf.length > 0) {
    const wc = {};
    if (allOf.length > 0) wc.all_of = allOf;
    if (anyOf.length > 0) wc.any_of = anyOf;
    body.append('when_clause', JSON.stringify(wc));
  } else {
    body.append('when_clause', '');
  }

  // Build sets (block only). Sent as one JSON object, matching SetsField.
  if (t.type === 'block') {
    const sets = setsMap(t.sets);
    body.append('sets', Object.keys(sets).length > 0 ? JSON.stringify(sets) : '');
  }

  let url;
  if (t.type === 'block')    url = `/admin/blocks/${t.id}/conditions`;
  if (t.type === 'location') url = `/admin/locations/${t.id}/conditions`;
  if (t.type === 'group')    url = `/admin/groups/${t.id}/conditions`;

  const csrfToken = (() => {
    try { return JSON.parse(document.body.getAttribute('hx-headers') || '{}')['X-CSRF-TOKEN'] || ''; }
    catch { return ''; }
  })();

  if (!csrfToken) {
    _showToast('Session expired — please refresh the page', 'error');
    return;
  }

  let ok = false;
  try {
    const resp = await fetch(url, {
      method: 'PATCH',
      headers: { 'X-CSRF-TOKEN': csrfToken },
      body,
    });
    ok = resp.ok;
  } catch { ok = false; }

  if (ok) {
    _updateBtnIcon(t);
    // Update data attributes so reopening reflects saved state.
    if (t.btnEl) {
      t.btnEl.dataset.when = (allOf.length > 0 || anyOf.length > 0)
        ? JSON.stringify({ all_of: allOf.length > 0 ? allOf : undefined, any_of: anyOf.length > 0 ? anyOf : undefined })
        : 'null';
      if (t.type === 'block') t.btnEl.dataset.sets = JSON.stringify(setsMap(t.sets));
    }
    document.getElementById('edit-drawer').checked = false;
    activeTarget = null;
    _showToast('Conditions saved', 'success');
  }
}

function _showToast(message, style) {
  const alerts = document.getElementById('alerts');
  if (!alerts) return;
  const div = document.createElement('div');
  div.role = 'alert';
  div.className = `alert alert-${style} mb-5`;
  div.innerHTML = `
    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="stroke-current shrink-0 w-6 h-6">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
    </svg>
    <span>${esc(message)}</span>
    <button type="button" class="btn btn-sm btn-ghost btn-circle" aria-label="Close" onclick="this.parentElement.remove();">
      <div class="radial-progress alert-progress" style="--value:0; --size:1rem;" role="progressbar">
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-x w-3 h-3"><path d="M18 6 6 18"></path><path d="m6 6 12 12"></path></svg>
      </div>
    </button>`;
  alerts.appendChild(div);
  // Auto-dismiss after 5s.
  const progress = div.querySelector('.alert-progress');
  let remaining = 5, timer;
  const tick = () => {
    remaining -= 0.1;
    if (progress) progress.style.setProperty('--value', Math.max(0, ((5 - remaining) / 5) * 100));
    if (remaining <= 0) { clearInterval(timer); div.style.transition = 'opacity 0.2s'; div.style.opacity = 0; setTimeout(() => div.remove(), 200); }
  };
  timer = setInterval(tick, 100);
  alerts.addEventListener('mouseenter', () => clearInterval(timer), { once: false });
  alerts.addEventListener('mouseleave', () => { timer = setInterval(tick, 100); }, { once: false });
}

function _updateBtnIcon(t) {
  const btn = t.btnEl;
  if (!btn) return;
  const hasConditions = t.allOf.filter(r => r.varName).length > 0 || t.anyOf.filter(r => r.varName).length > 0;
  const hasSets = t.type === 'block' && t.sets.filter(r => r.varName).length > 0;
  const active = hasConditions || hasSets;

  const eyeSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="w-3 h-3"><path d="M2.062 12.348a1 1 0 0 1 0-.696 10.75 10.75 0 0 1 19.876 0 1 1 0 0 1 0 .696 10.75 10.75 0 0 1-19.876 0"></path><circle cx="12" cy="12" r="3"></circle></svg>`;
  const scanEyeSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="w-3 h-3"><path d="M3 7V5a2 2 0 0 1 2-2h2"></path><path d="M17 3h2a2 2 0 0 1 2 2v2"></path><path d="M21 17v2a2 2 0 0 1-2 2h-2"></path><path d="M7 21H5a2 2 0 0 1-2-2v-2"></path><circle cx="12" cy="12" r="1"></circle><path d="M18.944 12.33a1 1 0 0 0 0-.66 7.5 7.5 0 0 0-13.888 0 1 1 0 0 0 0 .66 7.5 7.5 0 0 0 13.888 0"></path></svg>`;
  btn.innerHTML = active ? scanEyeSVG : eyeSVG;
}

// ── Helpers ───────────────────────────────────────────────────────────────

function esc(s) { return String(s || '').replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;').replace(/>/g, '&gt;'); }
