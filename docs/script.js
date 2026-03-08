/* ─────────────────────────────────────────────────────────────
   HABR INFOSEC — script.js
   ───────────────────────────────────────────────────────────── */
'use strict';

// ── Config ────────────────────────────────────────────────────

const DEFAULT_API_URL = '/api/articles';

function getApiUrl() {
  return localStorage.getItem('habrInfosec_apiUrl') || DEFAULT_API_URL;
}

function setApiUrl(url) {
  localStorage.setItem('habrInfosec_apiUrl', url.trim() || DEFAULT_API_URL);
}

// ── State ─────────────────────────────────────────────────────

let cachedArticles = [];
let isLoading = false;

// ── DOM refs ──────────────────────────────────────────────────

const $  = id => document.getElementById(id);
const $$ = sel => document.querySelectorAll(sel);

// ── Clock ─────────────────────────────────────────────────────

function startClock() {
  const el = $('tickerTime');
  const tick = () => {
    el.textContent = new Date().toLocaleTimeString('ru-RU', {
      hour: '2-digit', minute: '2-digit', second: '2-digit'
    });
  };
  tick();
  setInterval(tick, 1000);
}

// ── Ticker ────────────────────────────────────────────────────

function updateTicker(articles) {
  const track = $('tickerContent');
  if (!articles || articles.length === 0) {
    track.innerHTML = '<span>Нет данных для отображения</span>';
    return;
  }

  const items = articles.map(a => `<span>◆ ${escapeHtml(a.title)}</span>`).join('');
  // Duplicate for seamless loop
  track.innerHTML = items + items;
}

// ── View switching ────────────────────────────────────────────

function switchView(name) {
  $$('.view').forEach(v => v.classList.remove('active'));
  $$('.nav-btn').forEach(b => b.classList.remove('active'));

  const view = $(`view${name.charAt(0).toUpperCase() + name.slice(1)}`);
  if (view) view.classList.add('active');

  const btn = document.querySelector(`[data-view="${name}"]`);
  if (btn) btn.classList.add('active');

  if (name === 'settings') initSettingsView();
  if (name === 'stats')    updateStats();
}

// ── Status pill ───────────────────────────────────────────────

function setStatus(state, text) {
  const pill = $('statusPill');
  const label = $('statusText');
  pill.className = `status-pill ${state}`;
  label.textContent = text;
}

// ── Feed loading ──────────────────────────────────────────────

async function loadFeed() {
  if (isLoading) return;
  isLoading = true;

  const refreshBtn = document.querySelector('.refresh-btn');
  refreshBtn.classList.add('spinning');
  setStatus('loading', 'Загрузка...');

  showLoading();

  try {
    const url = getApiUrl();
    const res = await fetch(url, { cache: 'no-cache' });

    if (!res.ok) throw new Error(`HTTP ${res.status}: ${res.statusText}`);

    const articles = await res.json();

    if (!Array.isArray(articles)) throw new Error('Ответ API не является массивом');

    cachedArticles = articles;
    renderArticles(articles);
    updateTicker(articles);
    updateStats();

    setStatus('online', 'Онлайн');
    $('apiStatusVal').textContent = 'OK';
    $('lastUpdateVal').textContent = new Date().toLocaleTimeString('ru-RU');
    $('loadedCount').textContent = articles.length;

  } catch (err) {
    console.error('[HabrInfoSec] Fetch failed:', err);
    showError(err.message);
    setStatus('offline', 'Недоступен');
    $('apiStatusVal').textContent = 'ОШИБКА';
  } finally {
    isLoading = false;
    refreshBtn.classList.remove('spinning');
  }
}

// ── Render helpers ────────────────────────────────────────────

function showLoading() {
  $('loadingState').classList.remove('hidden');
  $('articlesGrid').classList.add('hidden');
  $('errorState').classList.add('hidden');
}

function showError(msg) {
  $('loadingState').classList.add('hidden');
  $('articlesGrid').classList.add('hidden');
  $('errorState').classList.remove('hidden');
  $('errorMsg').textContent = msg || 'Неизвестная ошибка';
}

function renderArticles(articles) {
  $('loadingState').classList.add('hidden');
  $('errorState').classList.add('hidden');
  $('articlesGrid').classList.remove('hidden');

  const grid = $('articlesGrid');
  grid.innerHTML = '';

  $('articleCount').textContent = `${articles.length} статей`;

  if (articles.length === 0) {
    grid.innerHTML = `<div style="grid-column:1/-1;text-align:center;padding:60px;color:var(--text-muted);font-family:var(--mono);font-size:12px;">НЕТ СТАТЕЙ</div>`;
    return;
  }

  articles.forEach((article, i) => {
    const card = buildCard(article, i);
    grid.appendChild(card);
  });
}

function buildCard(article, index) {
  const card = document.createElement('div');
  card.className = 'article-card';
  card.style.animationDelay = `${index * 60}ms`;
  card.style.opacity = '0';
  card.style.animation = `card-appear 0.4s ease-out ${index * 60}ms forwards`;

  const date = formatDate(article.date);

  const imgHtml = article.image
    ? `<img src="${sanitizeUrl(article.image)}" alt="" class="card-img" loading="lazy" onerror="this.closest('.card-img-wrap').innerHTML=getImgPlaceholder()">`
    : getImgPlaceholder();

  card.innerHTML = `
    <div class="card-img-wrap">${imgHtml}</div>
    <div class="card-body">
      <div class="card-meta">
        <span class="card-tag">INFOSEC</span>
        <span class="card-date">${escapeHtml(date)}</span>
      </div>
      <h2 class="card-title">${escapeHtml(article.title)}</h2>
      <p class="card-summary">${escapeHtml(article.summary || '')}</p>
      <div class="card-footer">
        <a class="card-read-link" href="${sanitizeUrl(article.link)}" target="_blank" rel="noopener" onclick="event.stopPropagation()">
          Читать на Хабре
          <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><line x1="5" y1="12" x2="19" y2="12"/><polyline points="12 5 19 12 12 19"/></svg>
        </a>
      </div>
    </div>`;

  card.addEventListener('click', () => openModal(article));

  // Add card-appear keyframes once
  if (!window._cardAnimAdded) {
    window._cardAnimAdded = true;
    const style = document.createElement('style');
    style.textContent = `@keyframes card-appear { from{opacity:0;transform:translateY(16px)} to{opacity:1;transform:translateY(0)} }`;
    document.head.appendChild(style);
  }

  return card;
}

function getImgPlaceholder() {
  return `<div class="card-img-placeholder">
    <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1">
      <rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="8.5" cy="8.5" r="1.5"/>
      <polyline points="21 15 16 10 5 21"/>
    </svg>
  </div>`;
}

// ── Modal ─────────────────────────────────────────────────────

function openModal(article) {
  const overlay = $('modalOverlay');

  $('modalTitle').textContent = article.title;
  $('modalSummary').textContent = article.summary || '';
  $('modalLink').href = sanitizeUrl(article.link);
  $('modalMeta').innerHTML = `
    <span class="card-tag">INFOSEC</span>
    <span class="card-date">${escapeHtml(formatDate(article.date))}</span>`;

  const imgWrap = $('modalImg');
  if (article.image) {
    imgWrap.innerHTML = `<img src="${sanitizeUrl(article.image)}" alt="" style="width:100%;max-height:260px;object-fit:cover;display:block;filter:saturate(0.7)">`;
  } else {
    imgWrap.innerHTML = '';
  }

  overlay.classList.remove('hidden');
  document.body.style.overflow = 'hidden';
}

function closeModal() {
  $('modalOverlay').classList.add('hidden');
  document.body.style.overflow = '';
}

document.addEventListener('keydown', e => {
  if (e.key === 'Escape') closeModal();
});

// ── Stats ─────────────────────────────────────────────────────

function updateStats() {
  if (!cachedArticles.length) {
    $('statsEmpty').classList.remove('hidden');
    return;
  }

  $('statsEmpty').classList.add('hidden');

  const now  = new Date();
  const day  = 24 * 3600 * 1000;
  const week = 7 * day;

  const today = cachedArticles.filter(a => now - new Date(a.date) < day).length;
  const wk    = cachedArticles.filter(a => now - new Date(a.date) < week).length;
  const imgs  = cachedArticles.filter(a => a.image).length;

  animNum('stat-total',    cachedArticles.length);
  animNum('stat-with-img', imgs);
  animNum('stat-today',    today);
  animNum('stat-week',     wk);
}

function animNum(id, target) {
  const el = $(id);
  const start = parseInt(el.textContent) || 0;
  const step = Math.ceil(Math.abs(target - start) / 20) || 1;
  let cur = start;
  const tick = () => {
    if (cur < target) {
      cur = Math.min(cur + step, target);
      el.textContent = cur;
      requestAnimationFrame(tick);
    } else {
      el.textContent = target;
    }
  };
  requestAnimationFrame(tick);
}

// ── Settings ──────────────────────────────────────────────────

function initSettingsView() {
  const saved = getApiUrl();
  $('apiUrlInput').value = saved;
  $('settingsCurrent').textContent = saved;
  $('settingsSaved').classList.add('hidden');
  $('testResult').classList.add('hidden');
}

function saveSettings() {
  const val = $('apiUrlInput').value.trim();
  setApiUrl(val);
  $('settingsCurrent').textContent = getApiUrl();
  $('settingsSaved').classList.remove('hidden');
  setTimeout(() => $('settingsSaved').classList.add('hidden'), 2500);
}

function setPreset(url) {
  $('apiUrlInput').value = url;
  saveSettings();
}

async function testConnection() {
  const result = $('testResult');
  result.className = 'test-result';
  result.classList.remove('hidden');
  result.textContent = 'Подключение...';

  try {
    const url = $('apiUrlInput').value.trim() || getApiUrl();
    const t0  = performance.now();
    const res = await fetch(url, { cache: 'no-cache' });
    const ms  = Math.round(performance.now() - t0);

    if (!res.ok) throw new Error(`HTTP ${res.status}`);

    const data = await res.json();
    const n = Array.isArray(data) ? data.length : '?';

    result.className = 'test-result success';
    result.textContent = `✓ OK — ${n} статей — ${ms}ms`;
  } catch (e) {
    result.className = 'test-result fail';
    result.textContent = `✗ Ошибка: ${e.message}`;
  }
}

// ── Utility ───────────────────────────────────────────────────

function escapeHtml(str) {
  const d = document.createElement('div');
  d.textContent = String(str ?? '');
  return d.innerHTML;
}

function sanitizeUrl(url) {
  try {
    const u = new URL(url, window.location.href);
    return (u.protocol === 'https:' || u.protocol === 'http:') ? u.href : '#';
  } catch { return '#'; }
}

function formatDate(iso) {
  try {
    return new Date(iso).toLocaleDateString('ru-RU', {
      day: 'numeric', month: 'short', year: 'numeric'
    });
  } catch { return ''; }
}

// ── Boot ──────────────────────────────────────────────────────

document.addEventListener('DOMContentLoaded', () => {
  startClock();
  initSettingsView();
  switchView('feed');
  loadFeed();
});

// Expose for console debugging
window.habrBot = { loadFeed, switchView, getApiUrl, setApiUrl };
