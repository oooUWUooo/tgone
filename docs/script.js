/* ─────────────────────────────────────────────────────────────
   HABR INFOSEC — script.js
   ───────────────────────────────────────────────────────────── */
'use strict';

// ── Config ────────────────────────────────────────────────────

const DEFAULT_API = '/api';

function getApiBase() { return localStorage.getItem('habrInfosec_api') || DEFAULT_API; }
function setApiBase(v) { localStorage.setItem('habrInfosec_api', v.trim() || DEFAULT_API); }

// ── App state ─────────────────────────────────────────────────

const state = {
  hubs:        [],        // [{id, name, emoji}]
  currentHub:  'infosec',
  articles:    {},        // hubID → Article[]
  bookmarks:   [],        // Article[]
  sseSource:   null,
  searchMode:  false,
};

// ── DOM helpers ───────────────────────────────────────────────

const $ = id => document.getElementById(id);
const show = el => el && el.classList.remove('hidden');
const hide = el => el && el.classList.add('hidden');

// ── Boot ──────────────────────────────────────────────────────

document.addEventListener('DOMContentLoaded', () => {
  loadBookmarks();
  startClock();
  initSettings();
  loadHubs().then(() => loadHub(state.currentHub));
  connectSSE();
  bindKeyboard();
  bindSearch();
  updateBookmarkBadge();
});

// ── Clock ─────────────────────────────────────────────────────

function startClock() {
  const el = $('tickerTime');
  const tick = () => el.textContent = new Date().toLocaleTimeString('ru-RU');
  tick(); setInterval(tick, 1000);
}

// ── Hub loading ───────────────────────────────────────────────

async function loadHubs() {
  try {
    const res = await fetch(`${getApiBase()}/hubs`);
    if (!res.ok) throw new Error();
    state.hubs = await res.json();
    renderHubTabs();
    renderSidebarHubs();
  } catch {
    // If API is unavailable, use defaults so the UI still renders
    state.hubs = [
      {id:'infosec',     name:'Информационная безопасность', emoji:'🔐'},
      {id:'devops',      name:'DevOps',                      emoji:'⚙️'},
      {id:'webdev',      name:'Веб-разработка',              emoji:'🌐'},
      {id:'programming', name:'Программирование',            emoji:'💻'},
      {id:'sysadm',      name:'Системное администрирование', emoji:'🖥'},
      {id:'linux',       name:'Linux',                       emoji:'🐧'},
    ];
    renderHubTabs();
    renderSidebarHubs();
  }
}

function renderHubTabs() {
  const bar = $('hubTabsBar');
  const container = $('hubTabs');
  bar.style.display = '';
  container.innerHTML = '';
  state.hubs.forEach((h, i) => {
    const tab = document.createElement('button');
    tab.className = 'hub-tab' + (h.id === state.currentHub ? ' active' : '');
    tab.dataset.hub = h.id;
    tab.title = `${h.name} (${i + 1})`;
    tab.innerHTML = `${h.emoji} ${h.name} <span class="tab-count" id="tab-count-${h.id}">—</span>`;
    tab.onclick = () => { state.searchMode = false; loadHub(h.id); };
    container.appendChild(tab);
  });
}

function renderSidebarHubs() {
  const c = $('sidebarHubs');
  c.innerHTML = '';
  state.hubs.forEach(h => {
    const btn = document.createElement('button');
    btn.className = 'sidebar-hub-btn' + (h.id === state.currentHub ? ' active' : '');
    btn.dataset.hub = h.id;
    btn.innerHTML = `<span>${h.emoji}</span><span class="shb-name">${h.name}</span>`;
    btn.onclick = () => { state.searchMode = false; loadHub(h.id); };
    c.appendChild(btn);
  });
}

function setActiveHub(hubID) {
  state.currentHub = hubID;
  document.querySelectorAll('.hub-tab').forEach(t => t.classList.toggle('active', t.dataset.hub === hubID));
  document.querySelectorAll('.sidebar-hub-btn').forEach(b => b.classList.toggle('active', b.dataset.hub === hubID));
}

// ── Article loading ───────────────────────────────────────────

async function loadHub(hubID) {
  setActiveHub(hubID);
  state.searchMode = false;
  hide($('searchResults'));
  show($('articlesGrid'));
  $('feedLabel').textContent = `// ЛЕНТА`;
  $('searchInput').value = '';

  const cached = state.articles[hubID];
  if (cached) { renderArticles(cached, hubID); return; }

  showLoading();
  setStatus('loading', 'Загрузка...');

  try {
    const res = await fetch(`${getApiBase()}/articles?hub=${encodeURIComponent(hubID)}`);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const articles = await res.json();
    state.articles[hubID] = articles;
    renderArticles(articles, hubID);
    updateTabCount(hubID, articles.length);
    updateSidebar(hubID, articles.length);
    setStatus('online', 'Онлайн');
  } catch (err) {
    showError(err.message);
    setStatus('offline', 'Нет связи');
  }
}

async function refreshAll() {
  const btn = $('refreshBtn');
  btn.classList.add('spinning');
  // Invalidate all cached hub data
  state.articles = {};
  // Reload current hub
  try {
    await loadHub(state.currentHub);
    showToast('✓ Лента обновлена');
  } finally {
    btn.classList.remove('spinning');
  }
}

// ── Render ────────────────────────────────────────────────────

function showLoading() {
  show($('loadingState'));
  hide($('articlesGrid'));
  hide($('searchResults'));
  hide($('errorState'));
}

function showError(msg) {
  hide($('loadingState'));
  hide($('articlesGrid'));
  show($('errorState'));
  $('errorMsg').textContent = msg || 'Проверьте соединение или настройки API.';
  setStatus('offline', 'Нет связи');
}

function renderArticles(articles, hubID) {
  hide($('loadingState'));
  hide($('errorState'));
  const grid = $('articlesGrid');
  show(grid);

  const hub = state.hubs.find(h => h.id === hubID) || {name: hubID};
  $('feedLabel').textContent = `// ${hub.emoji || ''} ${hub.name.toUpperCase()}`;
  $('articleCount').textContent = `${articles.length} статей`;
  $('sLoaded').textContent = articles.length;

  grid.innerHTML = '';
  articles.forEach((a, i) => grid.appendChild(buildCard(a, i)));
  updateTicker(articles);
}

function buildCard(article, index) {
  const card = document.createElement('div');
  card.className = 'article-card';
  card.style.opacity = '0';
  card.style.animation = `card-in 0.35s ease-out ${index * 45}ms forwards`;

  const isNew = (Date.now() - new Date(article.date)) < 86400000;
  const isBm  = isBookmarked(article.link);
  const date  = formatDate(article.date);
  const hub   = state.hubs.find(h => h.id === article.hub);
  const tag   = hub ? hub.name : (article.hub || 'HABR');

  const imgHtml = article.image
    ? `<img src="${sanitize(article.image)}" alt="" class="card-img" loading="lazy">`
    : `<div class="card-img-placeholder"><svg width="36" height="36" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1"><rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="8.5" cy="8.5" r="1.5"/><polyline points="21 15 16 10 5 21"/></svg></div>`;

  card.innerHTML = `
    ${isNew ? '<span class="card-new-badge">NEW</span>' : ''}
    <div class="card-img-wrap">${imgHtml}</div>
    <div class="card-body">
      <div class="card-meta">
        <span class="card-tag">${esc(tag)}</span>
        <span class="card-date">${esc(date)}</span>
        ${article.reading_time ? `<span class="card-read-time">${article.reading_time} мин</span>` : ''}
      </div>
      <h2 class="card-title">${esc(article.title)}</h2>
      <p class="card-summary">${esc(article.summary || '')}</p>
      <div class="card-footer">
        <a class="card-read-link" href="${sanitize(article.link)}" target="_blank" rel="noopener" onclick="event.stopPropagation()">
          Читать →
        </a>
        <button class="card-bookmark-btn ${isBm ? 'saved' : ''}" title="${isBm ? 'Удалить' : 'Сохранить'}" onclick="event.stopPropagation(); toggleBookmark(this, ${JSON.stringify(article).replace(/"/g, '&quot;')})">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="${isBm ? 'currentColor' : 'none'}" stroke="currentColor" stroke-width="2"><path d="M19 21l-7-5-7 5V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2z"/></svg>
        </button>
      </div>
    </div>`;

  card.addEventListener('click', () => openModal(article));

  if (!window._cardAnimDefined) {
    window._cardAnimDefined = true;
    const s = document.createElement('style');
    s.textContent = '@keyframes card-in{from{opacity:0;transform:translateY(14px)}to{opacity:1;transform:none}}';
    document.head.appendChild(s);
  }
  return card;
}

function updateTabCount(hubID, count) {
  const el = $(`tab-count-${hubID}`);
  if (el) el.textContent = count;
}

function updateSidebar(hubID, count) {
  $('sApiStatus').textContent = 'OK';
  $('sLastUpdate').textContent = new Date().toLocaleTimeString('ru-RU');
  $('sLoaded').textContent = count;
  // fetch subscribers count from health endpoint
  fetch(`${getApiBase()}/health`).then(r => r.json()).then(d => {
    $('sSubs').textContent = d.subscribers ?? '—';
  }).catch(() => {});
}

// ── Ticker ────────────────────────────────────────────────────

function updateTicker(articles) {
  const track = $('tickerContent');
  if (!articles?.length) return;
  const items = articles.map(a => `<span>◆ ${esc(a.title)}</span>`).join('');
  track.innerHTML = items + items;
}

// ── Search ────────────────────────────────────────────────────

function bindSearch() {
  const input = $('searchInput');
  let timer;

  input.addEventListener('input', () => {
    clearTimeout(timer);
    const q = input.value.trim();
    if (!q) { exitSearch(); return; }
    timer = setTimeout(() => runSearch(q), 320);
  });
}

async function runSearch(query) {
  state.searchMode = true;
  hide($('articlesGrid'));
  showLoading();

  try {
    const res = await fetch(`${getApiBase()}/search?q=${encodeURIComponent(query)}`);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const results = await res.json();
    const grid = $('searchResults');
    hide($('loadingState'));
    show(grid);
    grid.innerHTML = '';

    $('feedLabel').textContent = `// ПОИСК: ${query.toUpperCase()}`;
    $('articleCount').textContent = `${results.length} результатов`;

    if (!results.length) {
      grid.innerHTML = `<div style="grid-column:1/-1;padding:60px;text-align:center;color:var(--text-muted);font-family:var(--mono);font-size:12px;">НИЧЕгО НЕ НАЙДЕНО</div>`;
    } else {
      results.forEach((a, i) => grid.appendChild(buildCard(a, i)));
    }
  } catch (err) {
    showError(err.message);
  }
}

function exitSearch() {
  if (!state.searchMode) return;
  state.searchMode = false;
  hide($('searchResults'));
  const cached = state.articles[state.currentHub];
  if (cached) renderArticles(cached, state.currentHub);
  else loadHub(state.currentHub);
}

// ── Bookmarks ─────────────────────────────────────────────────

function loadBookmarks() {
  try { state.bookmarks = JSON.parse(localStorage.getItem('habrBookmarks') || '[]'); }
  catch { state.bookmarks = []; }
}

function saveBookmarks() {
  localStorage.setItem('habrBookmarks', JSON.stringify(state.bookmarks));
  updateBookmarkBadge();
}

function isBookmarked(link) { return state.bookmarks.some(b => b.link === link); }

function toggleBookmark(btn, article) {
  if (isBookmarked(article.link)) {
    state.bookmarks = state.bookmarks.filter(b => b.link !== article.link);
    btn.classList.remove('saved');
    btn.querySelector('svg').setAttribute('fill', 'none');
    btn.title = 'Сохранить';
    showToast('Закладка удалена');
  } else {
    state.bookmarks.unshift(article);
    btn.classList.add('saved');
    btn.querySelector('svg').setAttribute('fill', 'currentColor');
    btn.title = 'Удалить';
    showToast('✓ Добавлено в закладки');
  }
  saveBookmarks();
  // Update modal bookmark button if open
  syncModalBookmarkBtn(article.link);
}

function syncModalBookmarkBtn(link) {
  const btn = $('modalBookmarkBtn');
  if (!btn || btn.dataset.link !== link) return;
  const saved = isBookmarked(link);
  btn.classList.toggle('active', saved);
  btn.innerHTML = saved ? svgBookmarkFill() : svgBookmarkEmpty();
}

function svgBookmarkEmpty() { return `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M19 21l-7-5-7 5V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2z"/></svg>`; }
function svgBookmarkFill()  { return `<svg width="15" height="15" viewBox="0 0 24 24" fill="currentColor" stroke="currentColor" stroke-width="2"><path d="M19 21l-7-5-7 5V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2z"/></svg>`; }

function updateBookmarkBadge() {
  const badge = $('bookmarkBadge');
  const n = state.bookmarks.length;
  badge.textContent = n;
  badge.style.display = n > 0 ? '' : 'none';
}

function renderBookmarksView() {
  const grid = $('bookmarksGrid');
  const empty = $('bookmarksEmpty');
  const clearBtn = $('clearBmBtn');
  if (!state.bookmarks.length) {
    hide(grid); show(empty); hide(clearBtn); return;
  }
  hide(empty); show(grid); show(clearBtn);
  grid.innerHTML = '';
  state.bookmarks.forEach((a, i) => grid.appendChild(buildCard(a, i)));
}

function clearBookmarks() {
  if (!confirm('Удалить все закладки?')) return;
  state.bookmarks = [];
  saveBookmarks();
  renderBookmarksView();
}

// ── Stats ─────────────────────────────────────────────────────

async function loadStats() {
  try {
    const res = await fetch(`${getApiBase()}/stats`);
    if (!res.ok) throw new Error();
    const data = await res.json();
    renderStats(data);
  } catch {
    $('statsHubs').innerHTML = `<p style="color:var(--text-muted);font-family:var(--mono);font-size:12px;padding:20px 0">Загрузите ленту чтобы увидеть статистику.</p>`;
  }
}

function renderStats(data) {
  const summary = $('statsSummary');
  const total = data.hubs?.reduce((s, h) => s + h.count, 0) || 0;
  const maxCount = Math.max(...(data.hubs?.map(h => h.count) || [1]));

  summary.innerHTML = `
    <div class="stat-card"><div class="stat-card-num">${total}</div><div class="stat-card-label">Статей всего</div></div>
    <div class="stat-card"><div class="stat-card-num">${data.hubs?.length || 0}</div><div class="stat-card-label">Хабов</div></div>
    <div class="stat-card"><div class="stat-card-num">${data.subscribers || 0}</div><div class="stat-card-label">Подписчиков</div></div>
    <div class="stat-card"><div class="stat-card-num">${state.bookmarks.length}</div><div class="stat-card-label">Закладок</div></div>`;

  const hubsEl = $('statsHubs');
  hubsEl.innerHTML = '';
  (data.hubs || []).forEach(h => {
    const pct = maxCount > 0 ? Math.round((h.count / maxCount) * 100) : 0;
    const row = document.createElement('div');
    row.className = 'hub-stat-row';
    row.innerHTML = `
      <span class="hub-stat-emoji">${h.emoji}</span>
      <div class="hub-stat-info">
        <div class="hub-stat-name">${esc(h.name)}</div>
        <div class="hub-stat-bar-wrap"><div class="hub-stat-bar" style="width:0%" data-pct="${pct}"></div></div>
      </div>
      <span class="hub-stat-count">${h.count}</span>`;
    hubsEl.appendChild(row);
    // Animate bar after insertion
    requestAnimationFrame(() => {
      const bar = row.querySelector('.hub-stat-bar');
      setTimeout(() => { bar.style.width = pct + '%'; }, 50);
    });
  });
}

// ── SSE live updates ──────────────────────────────────────────

function connectSSE() {
  if (typeof EventSource === 'undefined') return;
  if (state.sseSource) state.sseSource.close();

  const es = new EventSource(`${getApiBase()}/events`);
  state.sseSource = es;

  es.onmessage = (e) => {
    try {
      const msg = JSON.parse(e.data);
      if (msg.type === 'refresh') {
        // Invalidate caches and silently reload current hub
        state.articles = {};
        loadHub(state.currentHub);
        showToast('⟳ Новые статьи');
      }
    } catch {}
  };

  es.onerror = () => {
    es.close();
    state.sseSource = null;
    // Retry after 30s
    setTimeout(connectSSE, 30000);
  };
}

// ── View switching ────────────────────────────────────────────

function switchView(name) {
  document.querySelectorAll('.view').forEach(v => v.classList.remove('active'));
  document.querySelectorAll('.nav-btn').forEach(b => b.classList.remove('active'));

  const view = $(`view${name[0].toUpperCase()}${name.slice(1)}`);
  if (view) view.classList.add('active');
  const btn = document.querySelector(`[data-view="${name}"]`);
  if (btn) btn.classList.add('active');

  $('hubTabsBar').style.display = name === 'feed' ? '' : 'none';

  if (name === 'bookmarks') renderBookmarksView();
  if (name === 'stats')     loadStats();
  if (name === 'settings')  initSettings();
}

// ── Modal ─────────────────────────────────────────────────────

function openModal(article) {
  const overlay = $('modalOverlay');
  const hub = state.hubs.find(h => h.id === article.hub);
  const saved = isBookmarked(article.link);

  $('modalHubTag').textContent = hub ? `${hub.emoji} ${hub.name}` : (article.hub || 'HABR');
  $('modalTitle').textContent = article.title;
  $('modalSummary').textContent = article.summary || '';
  $('modalLink').href = sanitize(article.link);
  $('modalOpenLink').href = sanitize(article.link);
  $('modalMeta').innerHTML = `
    <span class="card-tag">${esc(hub?.name || article.hub || '')}</span>
    <span class="card-date">${esc(formatDate(article.date))}</span>
    ${article.reading_time ? `<span class="card-date">${article.reading_time} мин</span>` : ''}`;

  const imgWrap = $('modalImg');
  imgWrap.innerHTML = article.image
    ? `<img src="${sanitize(article.image)}" alt="" style="width:100%;max-height:240px;object-fit:cover;display:block;filter:saturate(0.65)">`
    : '';

  const bmBtn = $('modalBookmarkBtn');
  bmBtn.dataset.link = article.link;
  bmBtn.className = 'icon-btn' + (saved ? ' active' : '');
  bmBtn.innerHTML = saved ? svgBookmarkFill() : svgBookmarkEmpty();
  bmBtn.title = saved ? 'Удалить закладку' : 'Добавить в закладки';
  bmBtn.onclick = () => {
    // Find card bookmark button(s) in grid and sync them
    document.querySelectorAll('.card-bookmark-btn').forEach(btn => {
      const card = btn.closest('.article-card');
      if (card && card.querySelector('.card-title')?.textContent === article.title) {
        toggleBookmark(btn, article);
      }
    });
    // Update modal button itself
    const nowSaved = isBookmarked(article.link);
    bmBtn.className = 'icon-btn' + (nowSaved ? ' active' : '');
    bmBtn.innerHTML = nowSaved ? svgBookmarkFill() : svgBookmarkEmpty();
    bmBtn.title = nowSaved ? 'Удалить закладку' : 'Добавить в закладки';
  };

  show(overlay);
  document.body.style.overflow = 'hidden';
}

function closeModal() {
  hide($('modalOverlay'));
  document.body.style.overflow = '';
}

// ── Keyboard shortcuts ────────────────────────────────────────

function bindKeyboard() {
  document.addEventListener('keydown', e => {
    const tag = document.activeElement.tagName;
    const typing = (tag === 'INPUT' || tag === 'TEXTAREA');

    if (e.key === 'Escape') {
      if (!$('modalOverlay').classList.contains('hidden')) { closeModal(); return; }
      if (typing) { document.activeElement.blur(); exitSearch(); return; }
    }

    if (typing) return;

    if (e.key === '/') {
      e.preventDefault();
      $('searchInput').focus();
    } else if (e.key.toLowerCase() === 'r') {
      refreshAll();
    } else if (e.key.toLowerCase() === 'b') {
      switchView('bookmarks');
    } else if (e.key >= '1' && e.key <= '6') {
      const idx = parseInt(e.key) - 1;
      if (state.hubs[idx]) { switchView('feed'); loadHub(state.hubs[idx].id); }
    }
  });
}

// ── Settings ──────────────────────────────────────────────────

function initSettings() {
  $('apiUrlInput').value = getApiBase();
  $('settingsCurrent').textContent = getApiBase();
  hide($('settingsSaved'));
  hide($('testResult'));
}

function saveSettings() {
  const val = $('apiUrlInput').value.trim();
  setApiBase(val);
  $('settingsCurrent').textContent = getApiBase();
  show($('settingsSaved'));
  setTimeout(() => hide($('settingsSaved')), 2500);
  // Reconnect SSE with new base
  state.articles = {};
  connectSSE();
}

function setPreset(url) {
  $('apiUrlInput').value = url;
  saveSettings();
}

async function testConnection() {
  const result = $('testResult');
  result.className = 'test-result';
  show(result);
  result.textContent = 'Проверяю...';

  const base = ($('apiUrlInput').value.trim() || getApiBase());
  try {
    const t0 = performance.now();
    const res = await fetch(`${base}/health`);
    const ms  = Math.round(performance.now() - t0);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const data = await res.json();
    result.className = 'test-result success';
    result.textContent = `✓ OK — ${data.bot || '?'} — ${ms}ms — ${data.subscribers || 0} подписчиков`;
  } catch (e) {
    result.className = 'test-result fail';
    result.textContent = `✗ Ошибка: ${e.message}`;
  }
}

// ── Toast ─────────────────────────────────────────────────────

let toastTimer;
function showToast(msg) {
  const el = $('toast');
  el.textContent = msg;
  show(el);
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => hide(el), 2500);
}

// ── Helpers ───────────────────────────────────────────────────

function esc(str) {
  const d = document.createElement('div');
  d.textContent = String(str ?? '');
  return d.innerHTML;
}

function sanitize(url) {
  try {
    const u = new URL(url, location.href);
    return (u.protocol === 'https:' || u.protocol === 'http:') ? u.href : '#';
  } catch { return '#'; }
}

function formatDate(iso) {
  try { return new Date(iso).toLocaleDateString('ru-RU', {day:'numeric', month:'short', year:'numeric'}); }
  catch { return ''; }
}

// ── Global exposure ───────────────────────────────────────────

window.switchView      = switchView;
window.loadHub         = loadHub;
window.refreshAll      = refreshAll;
window.openModal       = openModal;
window.closeModal      = closeModal;
window.toggleBookmark  = toggleBookmark;
window.clearBookmarks  = clearBookmarks;
window.saveSettings    = saveSettings;
window.setPreset       = setPreset;
window.testConnection  = testConnection;
