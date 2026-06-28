(function () {
  'use strict';

  const state = {
    fleet: [],
    syncedAt: null,
    selected: new Set(),
    eligibleOnly: false,
    search: '',
    activeBatchId: null,
    recentBatches: [],
    allBatches: [],
    jobs: [],
    batchRunning: false,
    batchTiming: null,
    batchProfile: 'wss-only',
    reanalyzingServerId: null,
    report: null,
    pollTimer: null,
    view: 'fleet',
    batchSubview: 'detail',
    fleetSort: { key: 'username', dir: 'asc' },
    batchesSort: { key: 'started_at', dir: 'desc' },
    jobsSort: { key: 'server_id', dir: 'asc' },
  };

  const JOB_STAGGER_MS = 30 * 1000;

  const REFRESH_ICON_SVG =
    '<svg class="icon-btn-svg" viewBox="0 0 24 24" width="18" height="18" aria-hidden="true" focusable="false">' +
    '<path fill="currentColor" d="M17.65 6.35A7.958 7.958 0 0 0 12 4c-4.42 0-7.99 3.58-7.99 8s3.57 8 7.99 8c3.73 0 6.84-2.55 7.73-6h-2.08A5.99 5.99 0 0 1 12 18c-3.31 0-6-2.69-6-6s2.69-6 6-6c1.66 0 3.14.69 4.22 1.78L13 11h7V4l-2.35 2.35z"/>' +
    '</svg>';

  function reanalyzeBtnHtml(serverId, busy, extraClass) {
    const cls = ['icon-btn', 'icon-btn-sm', 'reanalyze-job'].concat(extraClass || []);
    if (busy) cls.push('is-busy');
    const disabled = busy ? ' disabled aria-busy="true"' : '';
    return (
      `<button type="button" class="${cls.join(' ')}" data-sid="${serverId}" ` +
      `title="Re-analyze report" aria-label="Re-analyze report for server ${serverId}"${disabled}>` +
      `${REFRESH_ICON_SVG}</button>`
    );
  }

  function jobActionsCell(job) {
    const canReport = job.status === 'complete' && job.overall;
    const canReanalyze = !!job.run_dir && !['running', 'queued', 'skipped'].includes(job.status);
    const isBusy = state.reanalyzingServerId === job.server_id;
    const parts = [];
    if (canReanalyze) parts.push(reanalyzeBtnHtml(job.server_id, isBusy));
    if (canReport) {
      parts.push(
        `<button type="button" class="btn-link view-report" data-sid="${job.server_id}">View report</button>`
      );
    }
    if (!parts.length) {
      if (job.status === 'running' || job.status === 'queued') return '<span class="muted">—</span>';
      return '<span class="muted">pending</span>';
    }
    return `<div class="job-actions">${parts.join('')}</div>`;
  }

  const batchesSortGetters = {
    id: (b) => b.id || '',
    status: (b) => b.status || '',
    started_at: (b) => (b.started_at ? new Date(b.started_at).getTime() : 0),
    completed_at: (b) => (b.completed_at ? new Date(b.completed_at).getTime() : 0),
    estimated_end_at: (b) => batchEndMs(b) || 0,
    duration: (b) => parseDuration(b.duration) || 0,
    job_count: (b) => b.job_count ?? 0,
    pass_count: (b) => b.pass_count ?? 0,
    fail_count: (b) => b.fail_count ?? 0,
  };

  const fleetSortGetters = {
    username: (r) => r.username || '',
    server_id: (r) => r.server_id ?? 0,
    pair_id: (r) => r.pair_id || '',
    qa_eligible: (r) => !!r.qa_eligible,
    deployed_image_hash: (r) => r.deployed_image_hash || '',
    categories: (r) => (r.categories || []).join(', '),
  };

  const jobsSortGetters = {
    username: (j) => fleetUsername(j.server_id),
    server_id: (j) => j.server_id ?? 0,
    pair_id: (j) => j.pair_id || '',
    status: (j) => j.status || '',
    samples: (j) => j.samples ?? -1,
    last_bus_drops: (j) => j.last_bus_drops ?? -1,
    overall: (j) => j.overall || '',
  };

  const $ = (id) => document.getElementById(id);

  const els = {
    headerMeta: $('header-meta'),
    btnSync: $('btn-sync'),
    btnSyncEmpty: $('btn-sync-empty'),
    fleetEmpty: $('fleet-empty'),
    fleetContent: $('fleet-content'),
    fleetTbody: $('fleet-tbody'),
    fleetSearch: $('fleet-search'),
    fleetFilterEmpty: $('fleet-filter-empty'),
    chipEligible: $('chip-eligible'),
    btnSelectAll: $('btn-select-all'),
    selectionLabel: $('selection-label'),
    btnStart: $('btn-start'),
    soakDuration: $('soak-duration'),
    soakProfile: $('soak-profile'),
    recentBatches: $('recent-batches'),
    batchesLoading: $('batches-loading'),
    batchesEmpty: $('batches-empty'),
    batchesContent: $('batches-content'),
    batchesCount: $('batches-count'),
    batchesTbody: $('batches-tbody'),
    batchEmpty: $('batch-empty'),
    batchContent: $('batch-content'),
    batchTitle: $('batch-title'),
    batchStatusBadge: $('batch-status-badge'),
    batchUpdated: $('batch-updated'),
    batchTimeline: $('batch-timeline'),
    batchOutcomes: $('batch-outcomes'),
    batchJobsLabel: $('batch-jobs-label'),
    batchTimeLabel: $('batch-time-label'),
    batchProgress: $('batch-progress'),
    btnCancel: $('btn-cancel'),
    batchDetailPanel: $('batch-detail-panel'),
    batchReportPanel: $('batch-report-panel'),
    jobsTbody: $('jobs-tbody'),
    reportTitle: $('report-title'),
    reportBanner: $('report-banner'),
    reportBody: $('report-body'),
    artifactLinks: $('artifact-links'),
    btnBackBatch: $('btn-back-batch'),
    btnReportRefresh: $('btn-report-refresh'),
    lifecycleDialog: $('lifecycle-dialog'),
    toast: $('toast'),
  };

  function showToast(msg, isError) {
    els.toast.textContent = msg;
    els.toast.classList.toggle('error', !!isError);
    els.toast.classList.remove('hidden');
    clearTimeout(showToast._t);
    showToast._t = setTimeout(() => els.toast.classList.add('hidden'), 5000);
  }

  async function api(path, opts) {
    const res = await fetch(path, opts);
    const ct = res.headers.get('content-type') || '';
    let body;
    if (ct.includes('application/json')) {
      body = await res.json();
    } else {
      body = await res.text();
    }
    if (!res.ok) {
      const err = typeof body === 'object' && body.error ? body.error : res.statusText;
      throw new Error(err);
    }
    return body;
  }

  function shortHash(h) {
    if (!h || h.length <= 8) return h || '—';
    return h.slice(0, 4) + '…';
  }

  function parseOverall(md, jobOverall) {
    if (jobOverall) return jobOverall;
    const m = md.match(/OVERALL:\s*(PASS|FAIL|UNKNOWN)/i);
    return m ? m[1].toUpperCase() : 'UNKNOWN';
  }

  function compareValues(a, b, dir) {
    const mul = dir === 'asc' ? 1 : -1;
    if (a == null && b == null) return 0;
    if (a == null) return 1 * mul;
    if (b == null) return -1 * mul;
    if (typeof a === 'boolean' && typeof b === 'boolean') {
      if (a === b) return 0;
      return (a ? -1 : 1) * mul;
    }
    if (typeof a === 'number' && typeof b === 'number') return (a - b) * mul;
    return String(a).localeCompare(String(b), undefined, { numeric: true }) * mul;
  }

  function sortRows(rows, key, dir, getters) {
    const get = getters[key] || ((r) => r[key]);
    return [...rows].sort((a, b) => compareValues(get(a), get(b), dir));
  }

  function updateSortHeaders(tableId, sortState) {
    const table =
      tableId === 'fleet'
        ? $('fleet-table')
        : tableId === 'batches'
          ? $('batches-table')
          : $('jobs-table');
    if (!table) return;
    table.querySelectorAll('th[data-sort-key]').forEach((th) => {
      const key = th.dataset.sortKey;
      if (key === sortState.key) {
        th.setAttribute('aria-sort', sortState.dir === 'asc' ? 'ascending' : 'descending');
      } else {
        th.setAttribute('aria-sort', 'none');
      }
    });
  }

  function handleSortClick(tableId, key) {
    const sortState =
      tableId === 'fleet' ? state.fleetSort : tableId === 'batches' ? state.batchesSort : state.jobsSort;
    if (sortState.key === key) {
      sortState.dir = sortState.dir === 'asc' ? 'desc' : 'asc';
    } else {
      sortState.key = key;
      sortState.dir = tableId === 'batches' && ['started_at', 'completed_at', 'estimated_end_at'].includes(key) ? 'desc' : 'asc';
    }
    updateSortHeaders(tableId, sortState);
    if (tableId === 'fleet') renderFleet();
    else if (tableId === 'batches') renderBatchesTable();
    else renderJobsTable();
  }

  function eligibleCount() {
    return state.fleet.filter((r) => r.qa_eligible).length;
  }

  function filteredFleet() {
    const q = state.search.trim().toLowerCase();
    return state.fleet.filter((row) => {
      if (state.eligibleOnly && !row.qa_eligible) return false;
      if (!q) return true;
      const hay = [row.username, String(row.server_id), row.pair_id].join(' ').toLowerCase();
      return hay.includes(q);
    });
  }

  function updateHeader() {
    if (!state.syncedAt) {
      els.headerMeta.textContent = state.batchRunning && state.batchTiming?.remaining
        ? `Soak running · ${state.batchTiming.remaining}`
        : 'Not synced';
      return;
    }
    const n = eligibleCount();
    const total = state.fleet.length;
    const synced = new Date(state.syncedAt);
    const ageH = (Date.now() - synced.getTime()) / 3600000;
    let text = `Eligible ${n}/${total} · synced ${synced.toLocaleTimeString()}`;
    if (state.batchRunning && state.batchTiming?.remaining) {
      text += ` · soak running · ${state.batchTiming.remaining}`;
    }
    if (ageH > 1) {
      text += ' · stale';
      els.headerMeta.innerHTML = text.replace('stale', '<span class="stale-warning">stale (&gt;1h)</span>');
    } else {
      els.headerMeta.textContent = text;
    }
  }

  function renderFleet() {
    const hasFleet = state.fleet.length > 0;
    els.fleetEmpty.classList.toggle('hidden', hasFleet);
    els.fleetContent.classList.toggle('hidden', !hasFleet);
    if (!hasFleet) return;

    const rows = sortRows(filteredFleet(), state.fleetSort.key, state.fleetSort.dir, fleetSortGetters);
    els.fleetFilterEmpty.classList.toggle('hidden', rows.length > 0 || !state.search);
    updateSortHeaders('fleet', state.fleetSort);

    els.fleetTbody.innerHTML = rows
      .map((row) => {
        const sid = row.server_id;
        const checked = state.selected.has(sid);
        const disabled = !row.qa_eligible;
        const muted = !row.qa_eligible ? ' class="muted"' : '';
        const elig = row.qa_eligible
          ? '<span class="tag pass">yes</span>'
          : `<span class="tag muted" title="${esc(row.ineligible_reason)}">no</span>${
              row.ineligible_reason
                ? `<span class="reason-hint">${esc(shortReason(row.ineligible_reason))}</span>`
                : ''
            }`;
        const cats = (row.categories || []).join(', ') || '—';
        const ariaLabel = `Select ${row.username || 'subaccount'} server ${sid}`;
        return `<tr${muted}>
          <td class="col-check"><input type="checkbox" data-sid="${sid}" aria-label="${esc(ariaLabel)}" ${checked ? 'checked' : ''} ${disabled ? 'disabled' : ''}></td>
          <td>${esc(row.username)}</td>
          <td>${sid || '—'}</td>
          <td>${esc(row.pair_id || '—')}</td>
          <td>${elig}</td>
          <td>${esc(shortHash(row.deployed_image_hash))}</td>
          <td>${esc(cats)}</td>
        </tr>`;
      })
      .join('');

    const sel = state.selected.size;
    els.selectionLabel.textContent = `${sel} selected`;
    els.btnStart.disabled = sel === 0;
    updateHeader();
  }

  function esc(s) {
    const d = document.createElement('div');
    d.textContent = s;
    return d.innerHTML;
  }

  function shortReason(s) {
    if (!s) return '';
    return s.length > 40 ? s.slice(0, 37) + '…' : s;
  }

  function shortReason(s) {
    if (!s) return '';
    return s.length > 40 ? s.slice(0, 37) + '…' : s;
  }

  function statusTag(status, overall) {
    if (overall === 'PASS') return '<span class="tag pass">PASS</span>';
    if (overall === 'FAIL') return '<span class="tag fail">FAIL</span>';
    if (status === 'running') return '<span class="tag run">running</span>';
    if (status === 'queued') return '<span class="tag warn">queued</span>';
    if (status === 'skipped') return '<span class="tag muted">skipped</span>';
    if (status === 'complete') return '<span class="tag muted">complete</span>';
    return `<span class="tag muted">${esc(status || '—')}</span>`;
  }

  function highlightRecentBatch() {
    els.recentBatches.querySelectorAll('.recent-item').forEach((btn) => {
      btn.classList.toggle('active', btn.dataset.batch === state.activeBatchId);
    });
  }

  async function syncFleet() {
    els.btnSync.disabled = true;
    els.btnSync.textContent = 'Syncing…';
    try {
      const data = await api('/api/fleet/sync', { method: 'POST' });
      state.fleet = data.rows || [];
      state.syncedAt = data.synced_at;
      state.selected.clear();
      renderFleet();
      await loadRecentBatches();
      showToast('Fleet synced');
    } catch (e) {
      let msg = e.message;
      if (msg.includes('403') || msg.toLowerCase().includes('jwt')) {
        msg = 'Sync failed: check MANAGER_BEARER_TOKEN';
      }
      showToast('Sync failed: ' + msg, true);
    } finally {
      els.btnSync.disabled = false;
      els.btnSync.textContent = 'Sync fleet';
    }
  }

  async function loadFleetCached() {
    try {
      const data = await api('/api/fleet');
      state.fleet = data.rows || [];
      state.syncedAt = data.synced_at;
      renderFleet();
    } catch {
      /* not synced yet */
    }
  }

  function parseDuration(s) {
    if (!s || typeof s !== 'string') return null;
    const m = s.trim().match(/^(\d+(?:\.\d+)?)(h|m|s)$/i);
    if (!m) return null;
    const n = parseFloat(m[1]);
    const unit = m[2].toLowerCase();
    if (unit === 'h') return n * 3600000;
    if (unit === 'm') return n * 60000;
    if (unit === 's') return n * 1000;
    return null;
  }

  function formatDateTime(iso) {
    if (!iso) return '—';
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return '—';
    return d.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
  }

  function formatDateTimeTitle(iso) {
    if (!iso) return '';
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return '';
    return d.toLocaleString();
  }

  function shortBatchId(id) {
    if (!id) return 'Batch';
    const m = id.match(/^batch-(\d{4})(\d{2})(\d{2})T(\d{2})(\d{2})(\d{2})Z?/);
    if (!m) return id.length > 24 ? id.slice(0, 24) + '…' : id;
    const d = new Date(Date.UTC(+m[1], +m[2] - 1, +m[3], +m[4], +m[5], +m[6]));
    return d.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
  }

  function estimateBatchEndMs(startedIso, jobCount, durationStr) {
    const started = new Date(startedIso).getTime();
    const durMs = parseDuration(durationStr);
    if (Number.isNaN(started) || !durMs) return null;
    const n = Math.max(1, jobCount || 1);
    return started + (n - 1) * JOB_STAGGER_MS + durMs;
  }

  function batchEndMs(b) {
    if (b.estimated_end_at) return new Date(b.estimated_end_at).getTime();
    if (b.completed_at) return new Date(b.completed_at).getTime();
    const running = b.running || b.status === 'running';
    if (running && b.started_at && b.duration) {
      return estimateBatchEndMs(b.started_at, b.job_count, b.duration);
    }
    return null;
  }

  function formatRemaining(endMs) {
    if (!endMs) return '—';
    const diff = endMs - Date.now();
    if (diff <= 0) return 'finishing…';
    const sec = Math.ceil(diff / 1000);
    if (sec < 60) return sec + 's left';
    const min = Math.ceil(sec / 60);
    if (min < 60) return min + 'm left';
    const hr = Math.floor(min / 60);
    const rem = min % 60;
    return rem ? hr + 'h ' + rem + 'm left' : hr + 'h left';
  }

  function formatRelativePast(iso) {
    if (!iso) return '—';
    const diff = Date.now() - new Date(iso).getTime();
    if (diff < 0) return formatRemaining(new Date(iso).getTime());
    const sec = Math.floor(diff / 1000);
    if (sec < 60) return 'just now';
    const min = Math.floor(sec / 60);
    if (min < 60) return min + 'm ago';
    const hr = Math.floor(min / 60);
    if (hr < 24) return hr + 'h ago';
    const day = Math.floor(hr / 24);
    return day === 1 ? 'yesterday' : day + 'd ago';
  }

  function formatBatchTiming(batch, opts) {
    opts = opts || {};
    const running = opts.running ?? batch.running ?? batch.status === 'running';
    const jobCount = opts.jobCount ?? batch.job_count ?? (batch.server_ids || []).length ?? 0;
    const status = batch.status || (running ? 'running' : '');
    const started = batch.started_at;
    const completed = batch.completed_at;
    const duration = batch.duration || '';

    let endMs = null;
    let endIso = completed || null;
    let endIsEstimate = false;

    if (batch.estimated_end_at) {
      endMs = new Date(batch.estimated_end_at).getTime();
      endIso = batch.estimated_end_at;
      endIsEstimate = running;
    } else if (running && started && duration) {
      endMs = estimateBatchEndMs(started, jobCount, duration);
      endIso = endMs ? new Date(endMs).toISOString() : null;
      endIsEstimate = true;
    } else if (completed) {
      endMs = new Date(completed).getTime();
    }

    let remaining;
    if (status === 'cancelled') {
      remaining = 'cancelled';
    } else if (running) {
      remaining = formatRemaining(endMs);
    } else if (completed) {
      remaining = formatRelativePast(completed);
    } else {
      remaining = '—';
    }

    const endLabel = running
      ? endMs
        ? formatDateTime(endIso)
        : duration
          ? 'duration unknown'
          : '—'
      : formatDateTime(completed);

    return {
      start: formatDateTime(started),
      startTitle: formatDateTimeTitle(started),
      end: endLabel,
      endTitle: endIsEstimate
        ? 'Estimated end (includes 30s stagger between servers)'
        : formatDateTimeTitle(completed || endIso),
      endIsEstimate: running && endIsEstimate,
      duration: duration || '—',
      remaining,
      profile: batch.profile || '',
      concurrency: batch.concurrency,
    };
  }

  function fleetUsername(serverId) {
    const row = state.fleet.find((r) => r.server_id === serverId);
    return row?.username || '—';
  }

  function formatStartedAt(iso) {
    return formatDateTime(iso);
  }

  function batchStatusTag(b) {
    const st = b.status || (b.running ? 'running' : 'unknown');
    if (st === 'running' || b.running) return '<span class="tag run">running</span>';
    if (st === 'complete') return '<span class="tag pass">complete</span>';
    if (st === 'cancelled') return '<span class="tag warn">cancelled</span>';
    return `<span class="tag muted">${esc(st)}</span>`;
  }

  function renderSidebarBatches(batches) {
    const slice = batches.slice(0, 20);
    if (!slice.length) {
      els.recentBatches.innerHTML = '<p class="nav-empty">No batches yet</p>';
      return;
    }
    els.recentBatches.innerHTML = slice
      .map((b) => {
        const st = b.status || (b.running ? 'running' : 'unknown');
        const timing = formatBatchTiming(b, { running: b.running, jobCount: b.job_count });
        const label = shortBatchId(b.id);
        const running = b.running || st === 'running';
        const tail = running
          ? `${esc(st)} · ${esc(timing.remaining)}`
          : `${esc(st)} · ${b.pass_count ?? 0} pass · ${b.fail_count ?? 0} fail`;
        return `<button type="button" class="recent-item" data-batch="${esc(b.id)}">${esc(label)} · ${tail}</button>`;
      })
      .join('');
    highlightRecentBatch();
  }

  function renderBatchesTable() {
    const batches = sortRows(state.allBatches, state.batchesSort.key, state.batchesSort.dir, batchesSortGetters);
    updateSortHeaders('batches', state.batchesSort);
    els.batchesCount.textContent = batches.length ? `${batches.length} run${batches.length === 1 ? '' : 's'}` : '';

    els.batchesTbody.innerHTML = batches
      .map((b) => {
        const running = b.running || b.status === 'running';
        const timing = formatBatchTiming(b, { running, jobCount: b.job_count });
        const rowClass = [];
        if (b.id === state.activeBatchId) rowClass.push('row-active');
        if (running) rowClass.push('row-running');
        const cls = rowClass.length ? ` class="${rowClass.join(' ')}"` : '';
        const pass = b.pass_count ?? 0;
        const fail = b.fail_count ?? 0;
        const shortId = b.id.length > 20 ? b.id.slice(0, 20) + '…' : b.id;
        const endClass = timing.endIsEstimate ? ' class="col-time end-estimate"' : ' class="col-time"';
        return `<tr${cls}>
          <td>${batchStatusTag(b)}</td>
          <td class="col-time">${esc(timing.remaining)}</td>
          <td class="col-time" title="${esc(timing.startTitle)}">${esc(timing.start)}</td>
          <td${endClass} title="${esc(timing.endTitle)}">${esc(timing.end)}</td>
          <td class="col-time">${esc(timing.duration)}</td>
          <td class="batch-id-cell" title="${esc(b.id)}">${esc(shortId)}</td>
          <td><span class="tag pass">${pass}</span></td>
          <td><span class="tag fail">${fail}</span></td>
          <td>
            <button type="button" class="btn-link open-batch" data-batch="${esc(b.id)}">Open</button>
          </td>
        </tr>`;
      })
      .join('');
  }

  async function loadAllBatches() {
    if (state.view === 'batches') {
      els.batchesLoading.classList.remove('hidden');
      els.batchesEmpty.classList.add('hidden');
      els.batchesContent.classList.add('hidden');
    }
    try {
      const data = await api('/api/batches?limit=100');
      const batches = data.batches || [];
      state.allBatches = batches;
      state.recentBatches = batches.slice(0, 20);
      renderSidebarBatches(batches);

      if (state.view === 'batches') {
        els.batchesLoading.classList.add('hidden');
        if (!batches.length) {
          els.batchesEmpty.classList.remove('hidden');
          els.batchesContent.classList.add('hidden');
        } else {
          els.batchesEmpty.classList.add('hidden');
          els.batchesContent.classList.remove('hidden');
          renderBatchesTable();
        }
      }
    } catch {
      if (state.view === 'batches') {
        els.batchesLoading.classList.add('hidden');
        els.batchesEmpty.classList.remove('hidden');
        els.batchesEmpty.querySelector('p').textContent = 'Could not load soak runs.';
      }
      els.recentBatches.innerHTML = '<p class="nav-empty">Could not load batches</p>';
    }
  }

  async function loadRecentBatches() {
    await loadAllBatches();
  }

  function pickDefaultBatch() {
    if (state.activeBatchId) return;
    const running = state.recentBatches.find((b) => b.status === 'running' || b.running);
    if (running) state.activeBatchId = running.id;
    else if (state.recentBatches.length) state.activeBatchId = state.recentBatches[0].id;
  }

  function updateBatchPanelsVisibility() {
    const hasBatch = !!state.activeBatchId;
    els.batchEmpty.classList.toggle('hidden', hasBatch);
    els.batchContent.classList.toggle('hidden', !hasBatch);
  }

  function setBatchSubview(sub, opts) {
    opts = opts || {};
    state.batchSubview = sub;
    const isDetail = sub === 'detail';
    els.batchDetailPanel.classList.toggle('hidden', !isDetail);
    els.batchDetailPanel.setAttribute('aria-hidden', isDetail ? 'false' : 'true');
    if (isDetail) els.batchDetailPanel.removeAttribute('inert');
    else els.batchDetailPanel.setAttribute('inert', '');

    els.batchReportPanel.classList.toggle('hidden', isDetail);
    els.batchReportPanel.setAttribute('aria-hidden', isDetail ? 'true' : 'false');
    if (isDetail) els.batchReportPanel.setAttribute('inert', '');
    else els.batchReportPanel.removeAttribute('inert');

    if (!opts.skipHash) updateHash();
  }

  function updateHash() {
    let hash = state.view;
    if (state.view === 'batch' && state.batchSubview === 'report' && state.report) {
      hash = `batch/report/${state.report.serverId}`;
    }
    if (location.hash !== '#' + hash) {
      location.hash = hash;
    }
  }

  function parseHash() {
    const raw = (location.hash || '#fleet').slice(1);
    const parts = raw.split('/').filter(Boolean);
    if (parts[0] === 'jobs' && parts[1] === 'report' && parts[2]) {
      return { view: 'batch', batchSubview: 'report', serverId: parseInt(parts[2], 10), legacy: true };
    }
    if (parts[0] === 'jobs') {
      return { view: 'batch', batchSubview: 'detail', legacy: true };
    }
    if (parts[0] === 'report' && parts[1]) {
      return { view: 'batch', batchSubview: 'report', serverId: parseInt(parts[1], 10), legacy: true };
    }
    if (parts[0] === 'batch' && parts[1] === 'report' && parts[2]) {
      return { view: 'batch', batchSubview: 'report', serverId: parseInt(parts[2], 10) };
    }
    const view = ['fleet', 'batches', 'batch'].includes(parts[0]) ? parts[0] : 'fleet';
    return { view, batchSubview: 'detail' };
  }

  function setView(name, opts) {
    opts = opts || {};
    state.view = name;
    document.querySelectorAll('.nav-link[data-view]').forEach((a) => {
      a.classList.toggle('active', a.dataset.view === name);
    });
    document.querySelectorAll('.view').forEach((v) => {
      const isActive = v.id === 'view-' + name;
      v.classList.toggle('active', isActive);
      v.setAttribute('aria-hidden', isActive ? 'false' : 'true');
      if (isActive) v.removeAttribute('inert');
      else v.setAttribute('inert', '');
    });
    if (name === 'batch') pickDefaultBatch();
    if (name === 'batches') loadAllBatches();
    if (name === 'batch' && !opts.keepBatchSubview) setBatchSubview('detail', { skipHash: true });
    if (!opts.skipHash) updateHash();
  }

  function renderJobsTable() {
    if (!state.activeBatchId) return;
    updateSortHeaders('jobs', state.jobsSort);

    const jobs = sortRows(state.jobs, state.jobsSort.key, state.jobsSort.dir, jobsSortGetters);
    els.jobsTbody.innerHTML = jobs
      .map((job) => {
        const actions = jobActionsCell(job);
        return `<tr>
          <td>${esc(fleetUsername(job.server_id))}</td>
          <td>${job.server_id}</td>
          <td>${esc(job.pair_id || '—')}</td>
          <td>${statusTag(job.status, null)}</td>
          <td>${job.samples ?? '—'}</td>
          <td>${job.last_bus_drops ?? '—'}</td>
          <td>${job.overall ? statusTag(null, job.overall) : '—'}</td>
          <td class="col-actions">${actions}</td>
        </tr>`;
      })
      .join('');
  }

  function renderBatchTimeline(batch, timing, running) {
    const endLabel = running ? 'Ends ~' : 'Ended';
    const profileBits = [timing.profile, batch.concurrency ? `c=${batch.concurrency}` : ''].filter(Boolean);
    const rows = [
      ['Batch ID', batch.id || state.activeBatchId || '—', batch.id || state.activeBatchId || ''],
      ['Started', timing.start, timing.startTitle],
      ['Duration', timing.duration, ''],
      [endLabel, timing.end, timing.endTitle],
    ];
    if (profileBits.length) {
      rows.push(['Profile', profileBits.join(' · '), '']);
    }
    els.batchTimeline.innerHTML = rows
      .map(([label, value, title]) => {
        const titleAttr = title ? ` title="${esc(title)}"` : '';
        let valueClass = '';
        if (label === 'Batch ID') valueClass = 'batch-id-value';
        else if (label.startsWith('Ends') && timing.endIsEstimate) valueClass = 'end-estimate';
        const classAttr = valueClass ? ` class="${valueClass}"` : '';
        return `<dt>${esc(label)}</dt><dd${classAttr}${titleAttr}>${esc(value)}</dd>`;
      })
      .join('');
  }

  function applyBatchData(data) {
    const batch = data.batch || {};
    const jobs = data.jobs || [];
    const running = data.running || batch.status === 'running';
    state.jobs = jobs;
    state.batchRunning = running;
    state.batchProfile = batch.profile || 'wss-only';

    const jobCount = jobs.length || (batch.server_ids || []).length;
    const timing = formatBatchTiming(
      {
        ...batch,
        started_at: batch.started_at,
        completed_at: batch.completed_at,
        duration: batch.duration,
        profile: batch.profile,
        concurrency: batch.concurrency,
        status: batch.status,
        running,
      },
      { running, jobCount }
    );
    state.batchTiming = timing;

    const friendlyTitle = shortBatchId(state.activeBatchId);
    els.batchTitle.textContent = friendlyTitle + ' soak';
    els.batchTitle.title = state.activeBatchId || '';
    els.batchStatusBadge.textContent = batch.status || (running ? 'running' : '—');
    els.batchStatusBadge.className = 'tag ' + (running ? 'run' : batch.status === 'complete' ? 'pass' : 'muted');
    els.batchUpdated.textContent = 'Last updated ' + new Date().toLocaleTimeString();

    const pass = data.pass_count ?? 0;
    const fail = data.fail_count ?? 0;
    const skip = data.skipped_count ?? 0;
    const total = jobCount;
    const done = jobs.filter((j) => ['complete', 'skipped', 'failed'].includes(j.status)).length;

    renderBatchTimeline(batch, timing, running);
    els.batchJobsLabel.textContent = `${done}/${total}`;
    els.batchTimeLabel.textContent = running ? timing.remaining : timing.remaining;
    els.batchOutcomes.innerHTML = `
      <span class="tag pass">PASS ${pass}</span>
      <span class="tag fail">FAIL ${fail}</span>
      <span class="tag muted">SKIPPED ${skip}</span>`;

    els.batchProgress.style.width = total ? `${(done / total) * 100}%` : '0%';
    els.btnCancel.classList.toggle('hidden', !running);

    renderJobsTable();
    highlightRecentBatch();
    updateHeader();
    if (state.view === 'batches') renderBatchesTable();
  }

  async function refreshBatch() {
    pickDefaultBatch();
    updateBatchPanelsVisibility();

    if (!state.activeBatchId) {
      state.jobs = [];
      state.batchRunning = false;
      state.batchTiming = null;
      state.batchProfile = 'wss-only';
      updateHeader();
      return;
    }

    try {
      const data = await api('/api/batches/' + encodeURIComponent(state.activeBatchId));
      applyBatchData(data);

      const batch = data.batch || {};
      const running = data.running || batch.status === 'running';
      if (!running && batch.status !== 'running') {
        stopPolling();
      }
    } catch (e) {
      if (e.message.includes('not found')) {
        showToast('Batch not found', true);
        state.activeBatchId = null;
        state.jobs = [];
        stopPolling();
        updateBatchPanelsVisibility();
      }
    }
  }

  async function startBatch() {
    const ids = [...state.selected];
    if (!ids.length) return;

    const profile = els.soakProfile.value;
    if (profile === 'lifecycle') {
      const dlg = els.lifecycleDialog;
      const ok = await new Promise((resolve) => {
        dlg.addEventListener(
          'close',
          () => resolve(dlg.returnValue === 'confirm'),
          { once: true }
        );
        dlg.showModal();
      });
      if (!ok) {
        els.soakProfile.value = 'wss-only';
        return;
      }
    }

    els.btnStart.disabled = true;
    els.btnStart.textContent = 'Starting…';
    try {
      const res = await api('/api/batches', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          server_ids: ids,
          duration: els.soakDuration.value,
          interval: '5m',
          profile,
        }),
      });
      state.activeBatchId = res.batch_id;
      setView('batch');
      await refreshBatch();
      startPolling();
      await loadRecentBatches();
      showToast('Batch started: ' + res.batch_id);
    } catch (e) {
      showToast('Start failed: ' + e.message, true);
    } finally {
      els.btnStart.textContent = 'Start soak';
      els.btnStart.disabled = state.selected.size === 0;
    }
  }

  function stopPolling() {
    if (state.pollTimer) {
      clearInterval(state.pollTimer);
      state.pollTimer = null;
    }
  }

  function startPolling() {
    stopPolling();
    state.pollTimer = setInterval(() => {
      if (state.activeBatchId) refreshBatch();
      if (state.view === 'batches') loadAllBatches();
    }, 10000);
  }

  function findJob(serverId) {
    return state.jobs.find((j) => j.server_id === serverId);
  }

  function renderReportMarkdown(md, batchId, serverId) {
    const overall = parseOverall(md, null);
    els.reportBanner.textContent = 'OVERALL: ' + overall;
    els.reportBanner.className =
      'report-banner ' + (overall === 'PASS' ? 'pass' : overall === 'FAIL' ? 'fail' : 'unknown');
    els.reportBody.innerHTML = typeof marked !== 'undefined' ? marked.parse(md) : '<pre>' + esc(md) + '</pre>';

    const arts = ['metrics.tsv', 'soak.log', 'issues.log', 'run.env'];
    const base = '/api/batches/' + encodeURIComponent(batchId) + '/jobs/' + serverId + '/artifacts/';
    els.artifactLinks.innerHTML = arts
      .map((a) => `<a href="${base + a}" target="_blank" rel="noopener">${a}</a>`)
      .join('');
  }

  async function fetchReportMarkdown(batchId, serverId) {
    return api('/api/batches/' + encodeURIComponent(batchId) + '/jobs/' + serverId + '/report');
  }

  function setReportRefreshBusy(busy) {
    if (!els.btnReportRefresh) return;
    els.btnReportRefresh.disabled = busy;
    els.btnReportRefresh.classList.toggle('is-busy', busy);
    els.btnReportRefresh.setAttribute('aria-busy', busy ? 'true' : 'false');
  }

  async function reanalyzeJob(batchId, serverId, { refreshReportView = false } = {}) {
    const job = findJob(serverId);
    if (!job?.run_dir) {
      showToast('Cannot re-analyze: run directory missing for this job', true);
      return;
    }
    if (state.reanalyzingServerId != null) return;

    state.reanalyzingServerId = serverId;
    renderJobsTable();
    if (refreshReportView) {
      setReportRefreshBusy(true);
      els.reportBody.innerHTML = '<p class="muted">Re-analyzing…</p>';
    }

    try {
      const result = await api('/api/analyze', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          run_dir: job.run_dir,
          profile: state.batchProfile || 'wss-only',
        }),
      });
      await refreshBatch();
      if (refreshReportView && state.report?.serverId === serverId) {
        const md = await fetchReportMarkdown(batchId, serverId);
        renderReportMarkdown(md, batchId, serverId);
      }
      showToast(`Server ${serverId} re-analyzed: ${result.overall || 'done'}`);
    } catch (e) {
      showToast('Re-analyze failed: ' + e.message, true);
      if (refreshReportView && state.report?.serverId === serverId) {
        try {
          const md = await fetchReportMarkdown(batchId, serverId);
          renderReportMarkdown(md, batchId, serverId);
        } catch {
          els.reportBody.innerHTML = `<p>${esc('Re-analyze failed: ' + e.message)}</p>`;
        }
      }
    } finally {
      state.reanalyzingServerId = null;
      setReportRefreshBusy(false);
      renderJobsTable();
    }
  }

  async function reanalyzeReport() {
    if (!state.report || !state.activeBatchId) return;
    await reanalyzeJob(state.activeBatchId, state.report.serverId, { refreshReportView: true });
  }

  async function openReport(batchId, serverId) {
    state.report = { batchId, serverId };
    state.activeBatchId = batchId;
    await refreshBatch();
    setView('batch', { keepBatchSubview: true, skipHash: true });
    setBatchSubview('report', { skipHash: true });
    updateBatchPanelsVisibility();
    updateHash();

    els.reportTitle.textContent = `Server ${serverId}`;
    els.reportBody.innerHTML = '<p class="muted">Loading…</p>';
    els.reportBanner.textContent = '';
    els.reportBanner.className = 'report-banner';
    els.artifactLinks.innerHTML = '';
    setReportRefreshBusy(false);

    try {
      const md = await fetchReportMarkdown(batchId, serverId);
      renderReportMarkdown(md, batchId, serverId);
    } catch (e) {
      const msg =
        e.message.includes('still running') || e.message.includes('409')
          ? 'Report not ready — job still running'
          : 'Could not load report: ' + e.message;
      els.reportBody.innerHTML = `<p>${esc(msg)}</p><p><button type="button" class="btn-link" id="report-retry-back">Back to batch</button></p>`;
      $('report-retry-back')?.addEventListener('click', () => {
        setBatchSubview('detail');
        updateHash();
      });
    }
  }

  function selectBatch(batchId) {
    state.activeBatchId = batchId;
    highlightRecentBatch();
    refreshBatch();
    api('/api/batches/' + encodeURIComponent(batchId)).then((d) => {
      if (d.running || d.batch?.status === 'running') startPolling();
      else stopPolling();
    });
  }

  function bindEvents() {
    els.btnSync.addEventListener('click', syncFleet);
    els.btnSyncEmpty.addEventListener('click', syncFleet);

    els.fleetSearch.addEventListener('input', (e) => {
      state.search = e.target.value;
      renderFleet();
    });

    els.chipEligible.addEventListener('click', () => {
      state.eligibleOnly = !state.eligibleOnly;
      els.chipEligible.classList.toggle('on', state.eligibleOnly);
      renderFleet();
    });
    els.chipEligible.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        els.chipEligible.click();
      }
    });

    els.btnSelectAll.addEventListener('click', () => {
      state.selected.clear();
      state.fleet.filter((r) => r.qa_eligible).forEach((r) => {
        if (r.server_id) state.selected.add(r.server_id);
      });
      renderFleet();
    });

    els.fleetTbody.addEventListener('change', (e) => {
      const cb = e.target;
      if (cb.type !== 'checkbox') return;
      const sid = parseInt(cb.dataset.sid, 10);
      if (cb.checked) state.selected.add(sid);
      else state.selected.delete(sid);
      renderFleet();
    });

    els.btnStart.addEventListener('click', startBatch);

    els.btnCancel.addEventListener('click', async () => {
      if (!state.activeBatchId) return;
      try {
        await api('/api/batches/' + encodeURIComponent(state.activeBatchId) + '/cancel', {
          method: 'POST',
        });
        showToast('Cancelling batch…');
        await refreshBatch();
        await loadAllBatches();
      } catch (e) {
        showToast('Cancel failed: ' + e.message, true);
      }
    });

    els.jobsTbody.addEventListener('click', (e) => {
      const reanalyzeBtn = e.target.closest('.reanalyze-job');
      if (reanalyzeBtn && state.activeBatchId) {
        reanalyzeJob(state.activeBatchId, parseInt(reanalyzeBtn.dataset.sid, 10), {
          refreshReportView:
            state.batchSubview === 'report' &&
            state.report?.serverId === parseInt(reanalyzeBtn.dataset.sid, 10),
        });
        return;
      }
      const btn = e.target.closest('.view-report');
      if (!btn || !state.activeBatchId) return;
      openReport(state.activeBatchId, parseInt(btn.dataset.sid, 10));
    });

    els.batchesTbody.addEventListener('click', (e) => {
      const openBatch = e.target.closest('.open-batch');
      if (!openBatch) return;
      selectBatch(openBatch.dataset.batch);
      setView('batch');
    });

    els.btnBackBatch.addEventListener('click', () => {
      setBatchSubview('detail');
      updateHash();
    });

    els.btnReportRefresh?.addEventListener('click', () => {
      reanalyzeReport();
    });

    document.querySelectorAll('th[data-sort-table]').forEach((th) => {
      th.querySelector('.th-sort')?.addEventListener('click', () => {
        handleSortClick(th.dataset.sortTable, th.dataset.sortKey);
      });
    });

    document.querySelectorAll('.nav-link[data-view]').forEach((a) => {
      a.addEventListener('click', (e) => {
        e.preventDefault();
        const view = a.dataset.view;
        setView(view);
        if (view === 'batch') refreshBatch();
      });
    });

    document.querySelectorAll('[data-goto]').forEach((a) => {
      a.addEventListener('click', (e) => {
        e.preventDefault();
        const view = a.dataset.goto;
        setView(view);
        if (view === 'batch') refreshBatch();
      });
    });

    els.recentBatches.addEventListener('click', (e) => {
      const btn = e.target.closest('[data-batch]');
      if (!btn) return;
      selectBatch(btn.dataset.batch);
      setView('batch');
    });

    window.addEventListener('hashchange', () => {
      const parsed = parseHash();
      if (parsed.view === 'batch' && parsed.batchSubview === 'report' && parsed.serverId) {
        if (state.activeBatchId) {
          openReport(state.activeBatchId, parsed.serverId);
        } else {
          pickDefaultBatch();
          if (state.activeBatchId) openReport(state.activeBatchId, parsed.serverId);
          else setView('batch');
        }
        return;
      }
      setView(parsed.view, { keepBatchSubview: parsed.view === 'batch' && parsed.batchSubview === 'report' });
      if (parsed.view === 'batch') refreshBatch();
    });
  }

  async function init() {
    bindEvents();
    updateSortHeaders('fleet', state.fleetSort);
    updateSortHeaders('batches', state.batchesSort);
    updateSortHeaders('jobs', state.jobsSort);

    const parsed = parseHash();
    state.view = parsed.view;

    await loadFleetCached();
    await loadRecentBatches();

    if (parsed.view === 'batch' && parsed.batchSubview === 'report' && parsed.serverId) {
      pickDefaultBatch();
      if (state.activeBatchId) {
        await openReport(state.activeBatchId, parsed.serverId);
      } else {
        setView('batch');
      }
    } else {
      setView(state.view, { skipHash: true });
      if (state.view === 'batches') {
        await loadAllBatches();
      }
      if (state.view === 'batch') {
        await refreshBatch();
        const batch = state.recentBatches.find((b) => b.id === state.activeBatchId);
        if (batch?.status === 'running' || batch?.running) startPolling();
      }
    }

    if (!location.hash) location.hash = state.view;
    else if (parsed.legacy) updateHash();

    try {
      await api('/api/health');
      els.headerMeta.textContent = els.headerMeta.textContent || 'Connected';
    } catch {
      showToast('API health check failed', true);
    }
  }

  init();
})();
