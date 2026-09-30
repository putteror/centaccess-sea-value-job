/**
 * Sea Value Integration Agent - Multi-Page Web Portal Logic
 */

let activePage = 'dashboard';
let schedules = [];
let activeScheduleFilter = 'all';
let currentRawData = null;

let devices = [];
let activeDeviceFilter = 'all';
let deviceSearchQuery = '';

// 1. Multi-Page Navigation System (Sidebar + Topbar SPA Routing)
function switchPage(pageId) {
  activePage = pageId;

  const pages = ['dashboard', 'devices', 'schedules', 'logs'];
  const titles = {
    dashboard: { title: 'Dashboard & Sync', subtitle: 'Overview of manual sync operations and core service metrics' },
    devices: { title: 'Device Registry', subtitle: 'Manage access control scanners, OT gate classification & sync watermarks' },
    schedules: { title: 'Cron Schedules', subtitle: 'Configure automated background intervals and cron tasks' },
    logs: { title: 'Activity Console', subtitle: 'Live activity feed, API communications and background execution logs' }
  };

  pages.forEach((p) => {
    const viewEl = document.getElementById(`view-${p}`);
    const navBtn = document.getElementById(`nav-${p}`);

    if (viewEl) {
      if (p === pageId) {
        viewEl.classList.remove('hidden');
      } else {
        viewEl.classList.add('hidden');
      }
    }

    if (navBtn) {
      if (p === pageId) {
        navBtn.className = 'w-full px-3 py-2.5 rounded-xl font-semibold text-xs text-white bg-blue-600/90 shadow-sm shadow-blue-600/30 flex items-center justify-between transition-all cursor-pointer group';
      } else {
        navBtn.className = 'w-full px-3 py-2.5 rounded-xl font-medium text-xs text-slate-300 hover:text-white hover:bg-slate-800/80 flex items-center justify-between transition-all cursor-pointer group';
      }
    }
  });

  // Update topbar titles
  const titleInfo = titles[pageId] || titles.dashboard;
  const topTitleEl = document.getElementById('topbarPageTitle');
  const topSubEl = document.getElementById('topbarPageSubtitle');
  if (topTitleEl) topTitleEl.textContent = titleInfo.title;
  if (topSubEl) topSubEl.textContent = titleInfo.subtitle;

  // Sync window hash
  window.location.hash = pageId;

  // Refresh page data if needed
  if (pageId === 'devices') {
    loadDevices();
  } else if (pageId === 'schedules') {
    renderSchedules();
  }

  // Close sidebar on mobile
  if (window.innerWidth < 768) {
    const sidebar = document.getElementById('appSidebar');
    if (sidebar && !sidebar.classList.contains('-translate-x-full')) {
      sidebar.classList.add('-translate-x-full');
    }
  }
}

// Toggle mobile sidebar drawer
function toggleSidebar() {
  const sidebar = document.getElementById('appSidebar');
  if (sidebar) {
    sidebar.classList.toggle('-translate-x-full');
  }
}

// Live Bangkok Clock
function startLiveClock() {
  const update = () => {
    const clockEl = document.getElementById('liveClock');
    if (clockEl) {
      const now = new Date();
      clockEl.textContent = now.toLocaleTimeString('en-GB', { timeZone: 'Asia/Bangkok' });
    }
  };
  update();
  setInterval(update, 1000);
}


// Format Date object to YYYY-MM-DDTHH:mm for datetime-local input
function formatDateTimeForInput(d) {
  if (!d || isNaN(d.getTime())) return '';
  const pad = (num) => (num < 10 ? '0' : '') + num;
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

// Safely parse ISO / RFC3339 string to datetime-local input value
function parseToDateTimeInput(dateStr) {
  if (!dateStr || typeof dateStr !== 'string' || !dateStr.trim()) return '';
  try {
    const d = new Date(dateStr);
    if (isNaN(d.getTime())) return '';
    return formatDateTimeForInput(d);
  } catch (e) {
    return '';
  }
}

// Initialize datetime inputs to today's start and current time
function initModalDateTime() {
  const now = new Date();
  const startOfDay = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 0, 0, 0);

  const startEl = document.getElementById('modal_sync_start');
  const endEl = document.getElementById('modal_sync_end');
  if (startEl && !startEl.value) {
    startEl.value = formatDateTimeForInput(startOfDay);
  }
  if (endEl && !endEl.value) {
    endEl.value = formatDateTimeForInput(now);
  }
}

// Preset button clicked inside Time Range Modal
function setModalDatePreset(preset) {
  const now = new Date();
  let start = new Date();
  let end = new Date();

  if (preset === 'today') {
    start = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 0, 0, 0);
    end = now;
  } else if (preset === 'yesterday') {
    start = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1, 0, 0, 0);
    end = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1, 23, 59, 59);
  } else if (preset === 'last3days') {
    start = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 3, 0, 0, 0);
    end = now;
  } else if (preset === 'last7days') {
    start = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 7, 0, 0, 0);
    end = now;
  } else if (preset === 'thismonth') {
    start = new Date(now.getFullYear(), now.getMonth(), 1, 0, 0, 0);
    end = now;
  }

  document.getElementById('modal_sync_start').value = formatDateTimeForInput(start);
  document.getElementById('modal_sync_end').value = formatDateTimeForInput(end);
}

// Switch Tab in Sync Modal (Independent vs Continue)
function switchSyncModalTab(tab) {
  document.getElementById('trModal_syncTab').value = tab;
  const tabInd = document.getElementById('tabBtnIndependent');
  const tabCont = document.getElementById('tabBtnContinue');
  const contentInd = document.getElementById('tabContentIndependent');
  const contentCont = document.getElementById('tabContentContinue');
  const submitBtn = document.getElementById('trModalSubmitBtn');
  const submitText = document.getElementById('trModalSubmitText');

  if (tab === 'independent') {
    if (tabInd) tabInd.className = 'py-1.5 px-3 rounded-md text-xs font-bold transition-all cursor-pointer bg-white text-blue-700 shadow-xs text-center';
    if (tabCont) tabCont.className = 'py-1.5 px-3 rounded-md text-xs font-medium transition-all cursor-pointer text-slate-600 hover:text-slate-900 text-center';
    if (contentInd) contentInd.classList.remove('hidden');
    if (contentCont) contentCont.classList.add('hidden');
    if (submitBtn) submitBtn.className = 'px-4 py-1.5 rounded-lg bg-blue-600 hover:bg-blue-700 text-white text-xs font-semibold shadow-xs cursor-pointer transition-colors';
    if (submitText) submitText.textContent = 'Export TXT';
  } else {
    if (tabInd) tabInd.className = 'py-1.5 px-3 rounded-md text-xs font-medium transition-all cursor-pointer text-slate-600 hover:text-slate-900 text-center';
    if (tabCont) tabCont.className = 'py-1.5 px-3 rounded-md text-xs font-bold transition-all cursor-pointer bg-white text-purple-700 shadow-xs text-center';
    if (contentInd) contentInd.classList.add('hidden');
    if (contentCont) contentCont.classList.remove('hidden');
    if (submitBtn) submitBtn.className = 'px-4 py-1.5 rounded-lg bg-purple-600 hover:bg-purple-700 text-white text-xs font-semibold shadow-xs cursor-pointer transition-colors';
    if (submitText) submitText.textContent = 'Start Continue Sync';
    updateContinueSummaryCards();
  }
}

function onExportServiceTypeChange(val) {
  // Filter devices if needed
}

function renderExportDeviceList() {
  const container = document.getElementById('exportDeviceListContainer');
  if (!container) return;
  if (!devices || devices.length === 0) {
    container.innerHTML = '<div class="text-center py-4 text-xs text-slate-400">ไม่พบรายการเครื่อง</div>';
    return;
  }

  container.innerHTML = devices.map(d => {
    const isOt = d.isOt;
    const otBadge = isOt 
      ? '<span class="px-1.5 py-0.2 rounded text-[10px] font-bold bg-purple-50 text-purple-700 border border-purple-200">OT Gate</span>'
      : '<span class="px-1.5 py-0.2 rounded text-[10px] font-medium bg-slate-100 text-slate-600">Standard</span>';
    
    return `
      <label class="export-dev-item flex items-center justify-between p-1.5 rounded hover:bg-white transition-colors cursor-pointer text-xs" data-name="${(d.name||'').toLowerCase()}" data-id="${(d.id||'').toLowerCase()}">
        <div class="flex items-center gap-2">
          <input type="checkbox" value="${d.id || d.name}" class="export-dev-checkbox rounded border-slate-300 text-blue-600 focus:ring-blue-500 w-3.5 h-3.5" checked onchange="updateExportDeviceCountBadge()">
          <div class="flex items-center gap-2">
            <span class="font-medium text-slate-900">${d.name || 'Unnamed Device'}</span>
            ${otBadge}
            <span class="text-[10px] text-slate-400 font-mono">(${d.id || 'N/A'})</span>
          </div>
        </div>
        <div class="text-right">
          ${d.attendanceLastSyncTime ? `<span class="text-[10px] font-mono text-slate-400">${d.attendanceLastSyncTime.replace('T', ' ').substring(0, 19)}</span>` : '<span class="text-[10px] text-slate-300 font-mono">Never</span>'}
        </div>
      </label>
    `;
  }).join('');

  updateExportDeviceCountBadge();
}

function updateExportDeviceCountBadge() {
  const checkboxes = document.querySelectorAll('.export-dev-checkbox');
  const checked = document.querySelectorAll('.export-dev-checkbox:checked');
  const badge = document.getElementById('exportDeviceCountBadge');
  if (badge) {
    if (checkboxes.length === 0) {
      badge.textContent = 'ไม่มีเครื่อง';
    } else if (checked.length === checkboxes.length) {
      badge.textContent = `ทั้งหมด (${checked.length})`;
    } else {
      badge.textContent = `เลือก ${checked.length}/${checkboxes.length}`;
    }
  }
}

function filterExportDeviceList() {
  const search = (document.getElementById('exportDeviceSearchInput')?.value || '').toLowerCase().trim();
  const items = document.querySelectorAll('.export-dev-item');
  items.forEach(item => {
    const name = item.getAttribute('data-name') || '';
    const id = item.getAttribute('data-id') || '';
    if (!search || name.includes(search) || id.includes(search)) {
      item.style.display = 'flex';
    } else {
      item.style.display = 'none';
    }
  });
}

function toggleAllExportDevices() {
  const checkboxes = document.querySelectorAll('.export-dev-checkbox');
  const anyUnchecked = Array.from(checkboxes).some(cb => !cb.checked);
  checkboxes.forEach(cb => cb.checked = anyUnchecked);
  updateExportDeviceCountBadge();
}

function updateContinueSummaryCards() {
  const attEl = document.getElementById('continueAttendanceWatermarkText');
  const helpEl = document.getElementById('continueHelpWatermarkText');
  const nowEl = document.getElementById('continueCurrentTimeText');

  if (nowEl) {
    const now = new Date();
    nowEl.textContent = formatDateTimeDisplay(now);
  }

  if (!devices || devices.length === 0) {
    if (attEl) attEl.textContent = '00:00 (Today)';
    if (helpEl) helpEl.textContent = '00:00 (Today)';
    return;
  }

  let earliestAtt = null;
  let earliestHelp = null;

  devices.filter(d => d.enabled !== false).forEach(d => {
    if (d.attendanceLastSyncTime) {
      const t = new Date(d.attendanceLastSyncTime);
      if (!isNaN(t.getTime()) && (!earliestAtt || t < earliestAtt)) {
        earliestAtt = t;
      }
    }
    if (d.helpLastSyncTime) {
      const t = new Date(d.helpLastSyncTime);
      if (!isNaN(t.getTime()) && (!earliestHelp || t < earliestHelp)) {
        earliestHelp = t;
      }
    }
  });

  if (attEl) {
    attEl.textContent = earliestAtt ? formatDateTimeDisplay(earliestAtt) : '00:00 Today';
  }
  if (helpEl) {
    helpEl.textContent = earliestHelp ? formatDateTimeDisplay(earliestHelp) : '00:00 Today';
  }
}

function formatDateTimeDisplay(d) {
  if (!d || isNaN(d.getTime())) return '-';
  const pad = (n) => String(n).padStart(2, '0');
  const yyyy = d.getFullYear();
  const mm = pad(d.getMonth() + 1);
  const dd = pad(d.getDate());
  const hh = pad(d.getHours());
  const min = pad(d.getMinutes());
  const ss = pad(d.getSeconds());
  return `${yyyy}-${mm}-${dd} ${hh}:${min}:${ss}`;
}

// Open Time Range Selector Modal for Test Fetch or Manual Sync
function openTimeRangeModal(mode, service = 'all') {
  initModalDateTime();
  document.getElementById('trModal_mode').value = mode;
  document.getElementById('trModal_service').value = service;

  const titleEl = document.getElementById('trModalTitle');
  const subtitleEl = document.getElementById('trModalSubtitle');
  const tabsContainer = document.getElementById('syncModeTabsContainer');
  const testContent = document.getElementById('tabContentTest');
  const indContent = document.getElementById('tabContentIndependent');
  const contContent = document.getElementById('tabContentContinue');

  setModalDatePreset('today');

  if (mode === 'test') {
    if (titleEl) titleEl.textContent = 'Test API Preview';
    if (subtitleEl) subtitleEl.textContent = `ทดสอบดึงข้อมูล API (${service})`;

    if (tabsContainer) tabsContainer.classList.add('hidden');
    if (indContent) indContent.classList.add('hidden');
    if (contContent) contContent.classList.add('hidden');
    if (testContent) testContent.classList.remove('hidden');

    const submitBtn = document.getElementById('trModalSubmitBtn');
    const submitText = document.getElementById('trModalSubmitText');
    if (submitBtn) submitBtn.className = 'px-4 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-900 text-white text-xs font-semibold shadow-xs cursor-pointer transition-colors';
    if (submitText) submitText.textContent = 'Run Test';

    // Copy start/end to test inputs
    const startVal = document.getElementById('modal_sync_start')?.value;
    const endVal = document.getElementById('modal_sync_end')?.value;
    if (document.getElementById('modal_test_start') && startVal) document.getElementById('modal_test_start').value = startVal;
    if (document.getElementById('modal_test_end') && endVal) document.getElementById('modal_test_end').value = endVal;

  } else {
    // Mode === 'sync'
    if (titleEl) titleEl.textContent = 'Manual Sync';
    if (subtitleEl) subtitleEl.textContent = 'เลือกรูปแบบการซิงค์ข้อมูล';

    if (tabsContainer) tabsContainer.classList.remove('hidden');
    if (testContent) testContent.classList.add('hidden');

    // Select initial service in export radio
    const exportRadio = document.querySelector(`input[name="exportServiceType"][value="${service}"]`);
    if (exportRadio) {
      exportRadio.checked = true;
    } else {
      const defaultRadio = document.querySelector('input[name="exportServiceType"][value="attendance"]');
      if (defaultRadio) defaultRadio.checked = true;
    }

    // Select initial service in continue dropdown
    const contSelect = document.getElementById('continueServiceSelect');
    if (contSelect) {
      if (service === 'attendance' || service === 'help' || service === 'people' || service === 'all') {
        contSelect.value = service;
      } else {
        contSelect.value = 'all';
      }
    }

    renderExportDeviceList();
    switchSyncModalTab('independent');
  }

  document.getElementById('timeRangeModal').classList.remove('hidden');
}

function closeTimeRangeModal() {
  document.getElementById('timeRangeModal').classList.add('hidden');
}

function submitTimeRangeModal() {
  const mode = document.getElementById('trModal_mode').value;
  const service = document.getElementById('trModal_service').value;

  if (mode === 'test') {
    const startVal = document.getElementById('modal_test_start')?.value || document.getElementById('modal_sync_start')?.value;
    const endVal = document.getElementById('modal_test_end')?.value || document.getElementById('modal_sync_end')?.value;
    closeTimeRangeModal();
    executeTestFetch(service, startVal, endVal);
    return;
  }

  // Sync mode
  const syncTab = document.getElementById('trModal_syncTab').value;

  if (syncTab === 'independent') {
    const selectedRadio = document.querySelector('input[name="exportServiceType"]:checked');
    const targetService = selectedRadio ? selectedRadio.value : 'attendance';
    const startVal = document.getElementById('modal_sync_start').value;
    const endVal = document.getElementById('modal_sync_end').value;

    const checkedBoxes = document.querySelectorAll('.export-dev-checkbox:checked');
    const selectedDeviceIds = Array.from(checkedBoxes).map(cb => cb.value);

    closeTimeRangeModal();
    executeIndependentExport(targetService, startVal, endVal, selectedDeviceIds);
  } else {
    // Continue sync
    const contSelect = document.getElementById('continueServiceSelect');
    const targetService = contSelect ? contSelect.value : 'all';

    closeTimeRangeModal();
    executeContinueSync(targetService);
  }
}

// Execute Independent TXT Export (Direct Browser Download, No Server Save, No Watermark change)
async function executeIndependentExport(service, startVal, endVal, deviceIds) {
  const submitBtn = document.getElementById('trModalSubmitBtn');
  const origHTML = submitBtn ? submitBtn.innerHTML : '';
  if (submitBtn) {
    submitBtn.disabled = true;
    submitBtn.innerHTML = '<span>Exporting...</span>';
  }

  const rangeLabel = startVal && endVal ? ` (${startVal.replace('T', ' ')} - ${endVal.replace('T', ' ')})` : '';
  log(`Exporting .txt for ${service}${rangeLabel}...`, 'running');

  try {
    const payload = {
      service: service,
      start: startVal,
      end: endVal,
      deviceIds: deviceIds
    };

    const res = await fetch('/api/sync/export-txt', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });

    if (!res.ok) {
      const errText = await res.text();
      throw new Error(errText || `HTTP ${res.status}`);
    }

    let filename = `${service}_export.txt`;
    const disposition = res.headers.get('Content-Disposition');
    if (disposition && disposition.includes('filename=')) {
      const match = disposition.match(/filename="?([^";]+)"?/);
      if (match && match[1]) {
        filename = match[1];
      }
    }

    const blob = await res.blob();
    const blobUrl = window.URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = blobUrl;
    link.download = filename;
    document.body.appendChild(link);
    link.click();
    window.URL.revokeObjectURL(blobUrl);
    link.remove();

    const sizeKB = (blob.size / 1024).toFixed(1);
    log(`Downloaded ${filename} (${sizeKB} KB) successfully.`, 'success');
    showToast(`Downloaded ${filename}`, true);
  } catch (err) {
    log(`Export error: ${err.message}`, 'error');
    showToast(`Export failed: ${err.message}`, false);
  } finally {
    if (submitBtn) {
      submitBtn.disabled = false;
      submitBtn.innerHTML = origHTML;
    }
  }
}

// Execute Continue Sync (Uses device lastSyncTime, writes server files, updates watermarks)
async function executeContinueSync(service) {
  const submitBtn = document.getElementById('trModalSubmitBtn');
  const origHTML = submitBtn ? submitBtn.innerHTML : '';
  if (submitBtn) {
    submitBtn.disabled = true;
    submitBtn.innerHTML = '<span>Syncing...</span>';
  }

  log(`Starting Continue Sync for ${service}...`, 'running');

  try {
    const res = await fetch('/api/sync/continue', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ service: service })
    });
    const data = await res.json();

    if (res.ok && data.success) {
      log(`Continue Sync completed: ${data.message}`, 'success');
      if (data.results && Array.isArray(data.results)) {
        data.results.forEach(r => log(`  → ${r}`, 'success'));
      }
      showToast('Continue Sync completed.', true);
      loadDevices();
    } else {
      log(`Continue Sync error: ${data.error || data.message || 'Server error'}`, 'error');
      showToast(`Continue Sync failed: ${data.error || 'Server error'}`, false);
    }
  } catch (err) {
    log(`Network error: ${err.message}`, 'error');
    showToast(`Network error: ${err.message}`, false);
  } finally {
    if (submitBtn) {
      submitBtn.disabled = false;
      submitBtn.innerHTML = origHTML;
    }
  }
}

// 2. Initial Data Loading
async function loadInitialData() {
  initModalDateTime();
  startLiveClock();

  // Check URL hash for direct page navigation
  const hash = window.location.hash.replace('#', '');
  if (['dashboard', 'devices', 'schedules', 'logs'].includes(hash)) {
    switchPage(hash);
  } else {
    switchPage('dashboard');
  }

  try {
    const res = await fetch('/api/config');
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const data = await res.json();
    schedules = data.schedules || [];
    renderSchedules();

    // Update nav schedule badge
    const schBadge = document.getElementById('navSchCountBadge');
    if (schBadge) {
      schBadge.textContent = schedules.filter(s => s.enabled).length;
    }

    loadDevices();
  } catch (err) {
    log(`Failed to fetch schedules: ${err.message}`, 'error');
  }
}

// 3. Device Registry Management
async function loadDevices() {
  try {
    const res = await fetch('/api/devices');
    const data = await res.json();
    if (data.success && data.devices) {
      devices = data.devices;

      // Populate keywords & extra IDs inputs
      const kwInput = document.getElementById('devRegistry_keywords');
      if (kwInput && data.targetDeviceNames) {
        kwInput.value = data.targetDeviceNames.join(', ');
      }

      // Populate lookback & limit settings
      const initLbInput = document.getElementById('job_initial_lookback_hours');
      if (initLbInput && data.initialLookbackHours !== undefined) {
        initLbInput.value = data.initialLookbackHours;
      }
      const maxLbInput = document.getElementById('job_max_lookback_days');
      if (maxLbInput && data.maxLookbackDays !== undefined) {
        maxLbInput.value = data.maxLookbackDays;
      }

      renderDeviceRegistry();
    }
  } catch (err) {
    log(`Failed to load device registry: ${err.message}`, 'error');
  }
}

function renderDeviceRegistry() {
  const container = document.getElementById('deviceRegistryContainer');
  if (!container) return;

  const query = deviceSearchQuery.toLowerCase().trim();
  const filtered = devices.filter((d) => {
    if (activeDeviceFilter === 'ot' && !d.isOt) return false;
    if (activeDeviceFilter === 'normal' && d.isOt) return false;

    if (!query) return true;
    const nameMatch = d.name && d.name.toLowerCase().includes(query);
    const idMatch = d.id && d.id.toLowerCase().includes(query);
    const ipMatch = d.ipAddress && d.ipAddress.toLowerCase().includes(query);
    return nameMatch || idMatch || ipMatch;
  });

  // Calculate stats
  const total = devices.length;
  const otCount = devices.filter(d => d.isOt).length;
  const normalCount = devices.filter(d => !d.isOt).length;
  const activeCount = devices.filter(d => d.enabled !== false).length;

  document.getElementById('stat-total-devices').textContent = total;
  document.getElementById('stat-ot-devices').textContent = otCount;
  document.getElementById('stat-normal-devices').textContent = normalCount;
  document.getElementById('stat-active-devices').textContent = activeCount;

  document.getElementById('dev-tab-count-all').textContent = total;
  document.getElementById('dev-tab-count-ot').textContent = otCount;
  document.getElementById('dev-tab-count-normal').textContent = normalCount;

  document.getElementById('devSummaryBadge').textContent = `${otCount} OT Gates / ${total} Total Registered`;

  const navDevBadge = document.getElementById('navDevCountBadge');
  if (navDevBadge) {
    navDevBadge.textContent = `${otCount} OT`;
  }

  if (filtered.length === 0) {
    container.innerHTML = `
      <div class="p-8 text-center bg-slate-50 border border-dashed border-slate-200 rounded-xl space-y-1.5">
        <div class="text-sm font-semibold text-slate-700">No devices found</div>
        <p class="text-xs text-slate-500">Click 'Sync from CentAccess' to discover devices from the API.</p>
      </div>`;
    return;
  }

  container.innerHTML = filtered.map((d) => {
    const isOt = d.isOt;
    const isEnabled = d.enabled !== false;
    const statusBadge = d.status 
      ? `<span class="text-[10px] text-emerald-700 font-medium bg-emerald-50 border border-emerald-200 px-1.5 py-0.2 rounded">${d.status}</span>` 
      : '';

    let attSyncFormatted = 'Never';
    const attTime = d.attendanceLastSyncTime || d.lastSyncTime;
    if (attTime) {
      try {
        const dt = new Date(attTime);
        attSyncFormatted = dt.toLocaleTimeString('en-GB') + ' (' + dt.toLocaleDateString('en-CA') + ')';
      } catch (e) {
        attSyncFormatted = attTime;
      }
    }

    let helpSyncFormatted = 'Never';
    if (d.helpLastSyncTime) {
      try {
        const dt = new Date(d.helpLastSyncTime);
        helpSyncFormatted = dt.toLocaleTimeString('en-GB') + ' (' + dt.toLocaleDateString('en-CA') + ')';
      } catch (e) {
        helpSyncFormatted = d.helpLastSyncTime;
      }
    }

    const otBadge = isOt
      ? '<span class="text-[10px] font-semibold px-2 py-0.5 rounded bg-purple-50 text-purple-700 border border-purple-200">OT Gate</span>'
      : '<span class="text-[10px] font-semibold px-2 py-0.5 rounded bg-slate-100 text-slate-600 border border-slate-200">Standard</span>';

    return `
      <div class="bg-white hover:border-slate-300 border border-slate-200 rounded-xl p-4 transition-colors shadow-xs flex flex-col md:flex-row md:items-center justify-between gap-4 select-none ${!isEnabled ? 'opacity-60 bg-slate-50/50' : ''}">
        
        <div class="flex items-start gap-3 min-w-0">
          <div class="w-9 h-9 rounded-lg ${isOt ? 'bg-purple-50 text-purple-700 border-purple-200' : 'bg-slate-100 text-slate-600 border-slate-200'} border flex items-center justify-center shrink-0 mt-0.5">
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z"></path>
            </svg>
          </div>

          <div class="space-y-1 min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <span class="font-bold text-sm text-slate-900 truncate">${d.name || 'Unnamed Device'}</span>
              ${otBadge}
              ${statusBadge}
            </div>

            <div class="flex flex-wrap items-center gap-2 text-[11px] text-slate-500 font-mono">
              <span class="bg-slate-100 px-1.5 py-0.2 rounded border border-slate-200 text-slate-700">${d.id || 'No ID'}</span>
              ${d.ipAddress ? `<span class="text-slate-400">IP: ${d.ipAddress}</span>` : ''}
            </div>

            <!-- Watermarks with interactive click-to-edit -->
            <div class="flex flex-wrap items-center gap-3 text-[11px] text-slate-500 pt-0.5 font-mono">
              <span class="cursor-pointer hover:text-blue-600 transition-colors" onclick="openEditWatermarkModal('${d.id}')" title="คลิกเพื่อปรับแก้ Attendance Watermark">
                <span class="font-sans font-medium text-slate-400">Attendance:</span> <span class="font-semibold text-blue-700">${attSyncFormatted}</span>
              </span>
              <span class="cursor-pointer hover:text-purple-600 transition-colors" onclick="openEditWatermarkModal('${d.id}')" title="คลิกเพื่อปรับแก้ Help Watermark">
                <span class="font-sans font-medium text-slate-400">Help:</span> <span class="font-semibold text-purple-700">${helpSyncFormatted}</span>
              </span>
            </div>
          </div>
        </div>

        <div class="flex items-center gap-2 self-end md:self-center shrink-0">
          <!-- Adjust Watermark Button -->
          <button type="button" onclick="openEditWatermarkModal('${d.id}')" class="px-2.5 py-1 rounded-lg bg-blue-50 hover:bg-blue-100 text-blue-700 text-xs font-medium border border-blue-200 cursor-pointer transition-colors flex items-center gap-1 shadow-2xs" title="Adjust Last Sync Watermark">
            <svg class="w-3.5 h-3.5 text-blue-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"></path>
            </svg>
            <span>Adjust Sync</span>
          </button>

          <!-- OT Switch -->
          <label class="flex items-center gap-1.5 px-2.5 py-1 rounded-lg border text-xs font-medium cursor-pointer ${
            isOt ? 'bg-purple-50 border-purple-200 text-purple-800' : 'bg-slate-50 border-slate-200 text-slate-600 hover:bg-slate-100'
          }">
            <input type="checkbox" onchange="toggleDeviceOT('${d.id}', this.checked)" class="rounded text-purple-600 focus:ring-purple-500" ${isOt ? 'checked' : ''}>
            <span>Is OT</span>
          </label>

          <!-- Enable Switch -->
          <label class="relative inline-flex items-center cursor-pointer px-1" title="Enable/Disable this device in sync">
            <input type="checkbox" onchange="toggleDeviceEnabled('${d.id}', this.checked)" class="sr-only peer" ${isEnabled ? 'checked' : ''}>
            <div class="w-8 h-4.5 bg-slate-300 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[3px] after:bg-white after:rounded-full after:h-3.5 after:w-3.5 after:transition-all peer-checked:bg-emerald-600"></div>
          </label>

          <!-- Reset Watermark Button -->
          <button type="button" onclick="resetDeviceWatermark('${d.id}')" class="px-2 py-1 rounded-lg bg-white hover:bg-slate-100 text-slate-500 text-xs border border-slate-200 cursor-pointer transition-colors" title="Reset Watermark to 00:00">
            Reset
          </button>
        </div>

      </div>`;
  }).join('');
}

function filterDeviceRegistry(query) {
  deviceSearchQuery = query || '';
  renderDeviceRegistry();
}

function filterDeviceTab(tab) {
  activeDeviceFilter = tab;
  ['all', 'ot', 'normal'].forEach(t => {
    const el = document.getElementById(`dev-tab-${t}`);
    if (el) {
      if (t === tab) {
        el.className = 'px-2.5 py-1 rounded-md font-bold bg-slate-100 text-slate-900 cursor-pointer transition-all';
      } else {
        el.className = 'px-2.5 py-1 rounded-md font-medium text-slate-600 hover:text-slate-900 cursor-pointer transition-all';
      }
    }
  });
  renderDeviceRegistry();
}

function toggleDeviceOT(devId, isOt) {
  const d = devices.find(item => item.id === devId);
  if (d) {
    d.isOt = isOt;
    renderDeviceRegistry();
  }
}

function toggleDeviceEnabled(devId, enabled) {
  const d = devices.find(item => item.id === devId);
  if (d) {
    d.enabled = enabled;
    renderDeviceRegistry();
  }
}

function bulkSetOT(isOt) {
  devices.forEach(d => {
    d.isOt = isOt;
  });
  renderDeviceRegistry();
}

async function syncDevicesFromAPI() {
  const btn = document.getElementById('btn-sync-devices-api');
  btn.disabled = true;
  btn.innerHTML = '<span>Syncing CentAccess...</span>';
  log('Fetching live device master list from CentAccess API...', 'running');

  try {
    const res = await fetch('/api/devices/sync', { method: 'POST' });
    const data = await res.json();
    if (data.success && data.devices) {
      devices = data.devices;
      renderDeviceRegistry();
      log(data.message || 'Synced devices from CentAccess successfully.', 'success');
      showToast('Devices synced and updated.', true);
    } else {
      log(`Failed to sync devices: ${data.error || 'Unknown error'}`, 'error');
      showToast(data.error || 'Sync failed', false);
    }
  } catch (err) {
    log(`Network error: ${err.message}`, 'error');
    showToast(`Network error: ${err.message}`, false);
  } finally {
    btn.disabled = false;
    btn.innerHTML = `
      <svg class="w-3.5 h-3.5 text-slate-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"></path>
      </svg>
      <span>Sync from CentAccess</span>
    `;
  }
}

async function saveDeviceRegistry() {
  const saveBtn = document.getElementById('btn-save-device-registry');
  saveBtn.disabled = true;
  saveBtn.innerHTML = '<span>Saving...</span>';

  const kwInput = document.getElementById('devRegistry_keywords').value;
  const keywords = kwInput
    .split(',')
    .map(k => k.trim())
    .filter(k => k.length > 0);

  const extraIdsInput = document.getElementById('devRegistry_extra_ids').value;
  const extraIds = extraIdsInput
    .split(',')
    .map(id => id.trim())
    .filter(id => id.length > 0);

  const otIds = new Set(extraIds);
  devices.forEach(d => {
    if (d.isOt && d.id) otIds.add(d.id);
  });

  const initLbVal = parseInt(document.getElementById('job_initial_lookback_hours')?.value, 10);
  const maxLbVal = parseInt(document.getElementById('job_max_lookback_days')?.value, 10);

  const payload = {
    targetDeviceNames: keywords,
    targetDeviceIds: Array.from(otIds),
    devices: devices,
    initialLookbackHours: isNaN(initLbVal) ? 24 : initLbVal,
    maxLookbackDays: isNaN(maxLbVal) ? 7 : maxLbVal
  };

  log(`Saving device registry (${devices.length} devices, ${otIds.size} OT gates)...`, 'running');

  try {
    const res = await fetch('/api/config/devices', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    const data = await res.json();

    if (res.ok) {
      log('Device registry and OT configuration saved successfully.', 'success');
      showToast('Device registry saved.', true);
      loadDevices();
    } else {
      log(`Error saving devices: ${data.message || data.error || 'Server error'}`, 'error');
      showToast('Failed to save device configuration', false);
    }
  } catch (err) {
    log(`Network error: ${err.message}`, 'error');
    showToast(`Network error: ${err.message}`, false);
  } finally {
    saveBtn.disabled = false;
    saveBtn.innerHTML = '<span>Save Changes</span>';
  }
}

async function resetDeviceWatermark(devId, type = 'all') {
  const typeLabel = type === 'all' ? 'All (Attendance & Help)' : type;
  if (!confirm(`Reset sync watermark (${typeLabel}) for device ${devId}? The next sync will pull from 00:00 today.`)) return;

  try {
    const res = await fetch('/api/devices/reset-watermark', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ deviceId: devId, type: type })
    });
    if (res.ok) {
      showToast('Device sync watermark reset!', true);
      loadDevices();
    } else {
      showToast('Failed to reset watermark', false);
    }
  } catch (err) {
    showToast(`Error: ${err.message}`, false);
  }
}

// Open Watermark Adjustment Modal for a single device
function openEditWatermarkModal(devId) {
  const modal = document.getElementById('watermarkModal');
  if (!modal) return;

  const d = devices.find(x => x.id === devId);
  if (!d) {
    showToast('Device not found', false);
    return;
  }

  document.getElementById('wmModal_deviceId').value = devId;
  document.getElementById('wmModalTitle').innerHTML = `<span>Adjust Sync Watermark</span>`;
  
  const badge = document.getElementById('wmModalDeviceBadge');
  if (badge) {
    badge.textContent = d.isOt ? 'OT Gate Device' : 'Standard Device';
    badge.className = d.isOt 
      ? 'text-[11px] font-semibold px-2 py-0.5 rounded bg-purple-50 text-purple-800 border border-purple-200' 
      : 'text-[11px] font-semibold px-2 py-0.5 rounded bg-blue-50 text-blue-800 border border-blue-200';
  }

  document.getElementById('wmModalSubtitle').textContent = 'ปรับตั้งค่าเวลา Last Sync ของเครื่องนี้สำหรับการดึงข้อมูลรอบถัดไป';
  document.getElementById('wmModalDeviceInfoCard').classList.remove('hidden');
  document.getElementById('wmModalDeviceName').textContent = d.name || 'Unnamed Device';
  document.getElementById('wmModalDeviceIP').textContent = d.ipAddress ? `IP: ${d.ipAddress}` : '';
  document.getElementById('wmModalDeviceID').textContent = `Device ID: ${d.id}`;

  const attTime = d.attendanceLastSyncTime || d.lastSyncTime || '';
  const helpTime = d.helpLastSyncTime || '';

  document.getElementById('wmModal_attTime').value = parseToDateTimeInput(attTime);
  document.getElementById('wmModal_helpTime').value = parseToDateTimeInput(helpTime);

  modal.classList.remove('hidden');
}

// Open Watermark Adjustment Modal for ALL devices in bulk
function openBulkWatermarkModal() {
  const modal = document.getElementById('watermarkModal');
  if (!modal) return;

  document.getElementById('wmModal_deviceId').value = '';
  document.getElementById('wmModalTitle').innerHTML = `<span>Adjust All Watermarks (ทุกเครื่อง)</span>`;
  
  const badge = document.getElementById('wmModalDeviceBadge');
  if (badge) {
    badge.textContent = `All ${devices.length} Devices`;
    badge.className = 'text-[11px] font-semibold px-2 py-0.5 rounded bg-amber-50 text-amber-800 border border-amber-200';
  }

  document.getElementById('wmModalSubtitle').textContent = 'ปรับตั้งค่าเวลา Last Sync พร้อมกันทุกเครื่องที่ลงทะเบียนไว้';
  document.getElementById('wmModalDeviceInfoCard').classList.remove('hidden');
  document.getElementById('wmModalDeviceName').textContent = `All Registered Devices (${devices.length} Total)`;
  document.getElementById('wmModalDeviceIP').textContent = 'Global Watermark Update';
  document.getElementById('wmModalDeviceID').textContent = 'บันทึกเวลา Last Sync ให้เครื่องสแกนทุกเครื่องพร้อมกัน รวมถึงค่ากลางใน config.json';

  setWmField('both', 'today');

  modal.classList.remove('hidden');
}

function closeWatermarkModal() {
  const modal = document.getElementById('watermarkModal');
  if (modal) modal.classList.add('hidden');
}

function setWmField(target, preset) {
  const now = new Date();
  let val = '';
  if (preset === 'today') {
    val = formatDateTimeForInput(new Date(now.getFullYear(), now.getMonth(), now.getDate(), 0, 0, 0));
  } else if (preset === 'yesterday') {
    val = formatDateTimeForInput(new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1, 0, 0, 0));
  } else if (preset === '24h') {
    val = formatDateTimeForInput(new Date(now.getTime() - 24 * 60 * 60 * 1000));
  } else if (preset === 'now') {
    val = formatDateTimeForInput(now);
  } else if (preset === 'clear') {
    val = '';
  }

  if (target === 'attendance' || target === 'both') {
    const el = document.getElementById('wmModal_attTime');
    if (el) el.value = val;
  }
  if (target === 'help' || target === 'both') {
    const el = document.getElementById('wmModal_helpTime');
    if (el) el.value = val;
  }
}

async function submitWatermarkModal() {
  const devId = document.getElementById('wmModal_deviceId').value;
  const attVal = document.getElementById('wmModal_attTime').value;
  const helpVal = document.getElementById('wmModal_helpTime').value;

  const btn = document.getElementById('btn-save-watermark');
  const origHTML = btn ? btn.innerHTML : '';
  if (btn) {
    btn.disabled = true;
    btn.innerHTML = '<span>Saving...</span>';
  }

  try {
    const payload = {
      deviceId: devId,
      attendanceLastSyncTime: attVal,
      helpLastSyncTime: helpVal
    };

    const res = await fetch('/api/devices/reset-watermark', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });

    const data = await res.json();
    if (res.ok && data.success) {
      showToast(data.message || 'Sync watermark updated successfully!', true);
      log(`Updated sync watermark for ${devId ? 'device ' + devId : 'all devices'} (Att: ${attVal || 'Cleared'}, Help: ${helpVal || 'Cleared'})`, 'success');
      closeWatermarkModal();
      await loadDevices();
    } else {
      showToast(data.message || data.error || 'Failed to update watermark', false);
      log(`Watermark update failed: ${data.message || data.error || 'Server error'}`, 'error');
    }
  } catch (err) {
    showToast(`Network error: ${err.message}`, false);
    log(`Network error while updating watermark: ${err.message}`, 'error');
  } finally {
    if (btn) {
      btn.disabled = false;
      btn.innerHTML = origHTML;
    }
  }
}


// 4. Cron Schedules Management
function humanizeCron(expr) {
  if (!expr) return '';
  const trimmed = expr.trim();
  if (trimmed === '0 * * * *') return 'Hourly (every hour at :00)';
  if (trimmed === '0 */2 * * *') return 'Every 2 hours';
  if (trimmed === '*/15 * * * *') return 'Every 15 minutes';
  if (trimmed === '*/30 * * * *') return 'Every 30 minutes';
  if (trimmed === '*/5 * * * *') return 'Every 5 minutes';
  if (trimmed === '30 5 * * *') return 'Daily at 05:30 AM';
  if (trimmed === '0 22 * * *') return 'Daily at 22:00 PM';
  if (trimmed === '0 23 * * *') return 'Daily at 23:00 PM';
  if (trimmed === '0 0 * * *') return 'Daily at Midnight (00:00)';
  return 'Custom Cron';
}

function updateCronPreview(val) {
  const hint = humanizeCron(val);
  const el = document.getElementById('cronHumanPreview');
  if (el) el.textContent = hint ? hint : '';
}

function setModalCron(expr) {
  document.getElementById('modal_sch_cron').value = expr;
  updateCronPreview(expr);
}

function renderSchedules() {
  const container = document.getElementById('schedulesContainer');
  if (!container) return;

  const filtered = schedules.filter((s) => {
    const type = (s.type || 'attendance').toLowerCase();
    if (activeScheduleFilter === 'all') return true;
    if (activeScheduleFilter === 'attendance') return type === 'attendance' || type === 'access_records' || type === 'overtime';
    if (activeScheduleFilter === 'help') return type === 'help';
    if (activeScheduleFilter === 'people') return type === 'people' || type === 'peopleimport';
    return true;
  });

  const totalCount = schedules.length;
  const attCount = schedules.filter(s => ['attendance', 'access_records', 'overtime'].includes((s.type || '').toLowerCase())).length;
  const helpCount = schedules.filter(s => (s.type || '').toLowerCase() === 'help').length;
  const peoCount = schedules.filter(s => ['people', 'peopleimport'].includes((s.type || '').toLowerCase())).length;
  const activeCount = schedules.filter(s => s.enabled).length;

  document.getElementById('count-all').textContent = totalCount;
  document.getElementById('count-att').textContent = attCount;
  const helpCountEl = document.getElementById('count-help');
  if (helpCountEl) helpCountEl.textContent = helpCount;
  document.getElementById('count-peo').textContent = peoCount;
  document.getElementById('activeCountBadge').textContent = `${activeCount} Active / ${totalCount} Total`;

  const schBadge = document.getElementById('navSchCountBadge');
  if (schBadge) {
    schBadge.textContent = activeCount;
  }

  if (filtered.length === 0) {
    container.innerHTML = `
      <div class="p-8 text-center bg-slate-50 border border-dashed border-slate-200 rounded-xl space-y-1.5">
        <div class="text-sm font-semibold text-slate-700">No schedules found</div>
        <p class="text-xs text-slate-500">Click '+ Add Schedule' to create a new cron schedule.</p>
      </div>`;
    return;
  }

  container.innerHTML = filtered.map((sch) => {
    const typeNormalized = (sch.type || 'attendance').toLowerCase();
    let typeBadge = 'Access Records';
    let typeIcon = `
      <svg class="w-4 h-4 text-blue-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"></path>
      </svg>`;
    let badgeColor = 'bg-blue-50 text-blue-700 border-blue-200';
    let runNowClick = `openTimeRangeModal('sync', 'attendance')`;

    if (typeNormalized === 'help') {
      typeIcon = `
        <svg class="w-4 h-4 text-purple-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
        </svg>`;
      typeBadge = 'Help Records';
      badgeColor = 'bg-purple-50 text-purple-700 border-purple-200';
      runNowClick = `openTimeRangeModal('sync', 'help')`;
    } else if (typeNormalized === 'people' || typeNormalized === 'peopleimport') {
      typeIcon = `
        <svg class="w-4 h-4 text-emerald-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"></path>
        </svg>`;
      typeBadge = 'People Import';
      badgeColor = 'bg-emerald-50 text-emerald-700 border-emerald-200';
      runNowClick = `executeSync('people')`;
    }

    const humanText = humanizeCron(sch.cronSchedule);
    const statusClass = sch.enabled 
      ? 'bg-emerald-50 text-emerald-700 border border-emerald-200' 
      : 'bg-slate-100 text-slate-500 border border-slate-200';
    const statusLabel = sch.enabled ? 'Active' : 'Disabled';
    const isChecked = sch.enabled ? 'checked' : '';

    return `
      <div class="bg-white hover:border-slate-300 border border-slate-200 rounded-xl p-4 transition-colors shadow-xs flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div class="flex items-start gap-3 min-w-0">
          <div class="w-9 h-9 rounded-lg bg-slate-100 border border-slate-200 flex items-center justify-center shrink-0 mt-0.5">
            ${typeIcon}
          </div>
          <div class="space-y-1 min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <span class="font-bold text-sm text-slate-900 truncate">${sch.name || 'Unnamed Schedule'}</span>
              <span class="text-[10px] font-semibold px-2 py-0.5 rounded border ${badgeColor}">
                ${typeBadge}
              </span>
              <span class="text-[10px] font-semibold px-2 py-0.5 rounded ${statusClass}">
                ${statusLabel}
              </span>
            </div>
            <div class="flex flex-wrap items-center gap-2 text-xs">
              <code class="text-blue-700 bg-blue-50 border border-blue-100 px-2 py-0.5 rounded font-mono font-medium">${sch.cronSchedule}</code>
              ${humanText ? `<span class="text-slate-500 text-[11px]">${humanText}</span>` : ''}
              ${sch.lastRunAt ? `<span class="text-slate-400 font-mono text-[10px]">Last Run: ${sch.lastRunAt}</span>` : ''}
            </div>
          </div>
        </div>
        <div class="flex items-center gap-2 self-end md:self-center shrink-0">
          <button type="button" onclick="${runNowClick}" class="px-2.5 py-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 text-slate-700 font-medium text-xs transition-colors cursor-pointer" title="Run task right now">
            Run Now
          </button>
          <label class="relative inline-flex items-center cursor-pointer px-1" title="Toggle Enabled">
            <input type="checkbox" onchange="toggleSchedule('${sch.id}')" class="sr-only peer" ${isChecked}>
            <div class="w-8 h-4.5 bg-slate-300 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[3px] after:bg-white after:rounded-full after:h-3.5 after:w-3.5 after:transition-all peer-checked:bg-blue-600"></div>
          </label>
          <button type="button" onclick="editSchedule('${sch.id}')" class="w-7 h-7 rounded-lg bg-white hover:bg-slate-100 text-slate-500 hover:text-slate-800 border border-slate-200 flex items-center justify-center transition-colors cursor-pointer" title="Edit Schedule">
            <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"></path>
            </svg>
          </button>
          <button type="button" onclick="deleteSchedule('${sch.id}')" class="w-7 h-7 rounded-lg bg-white hover:bg-rose-50 text-slate-400 hover:text-rose-600 border border-slate-200 hover:border-rose-200 flex items-center justify-center transition-colors cursor-pointer" title="Delete Schedule">
            <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path>
            </svg>
          </button>
        </div>
      </div>`;
  }).join('');
}

function filterSchedules(tab) {
  activeScheduleFilter = tab;
  const tabs = ['all', 'attendance', 'help', 'people'];
  tabs.forEach((t) => {
    const el = document.getElementById('tab-' + t);
    if (el) {
      if (t === tab) {
        el.className = 'px-3 py-1.5 rounded-lg font-bold bg-white text-slate-900 shadow-xs cursor-pointer transition-all';
      } else {
        el.className = 'px-3 py-1.5 rounded-lg font-medium text-slate-600 hover:text-slate-900 cursor-pointer transition-all';
      }
    }
  });
  renderSchedules();
}

function toggleSchedule(id) {
  const target = schedules.find((s) => s.id === id);
  if (target) {
    target.enabled = !target.enabled;
    markUnsaved();
    renderSchedules();
  }
}

function deleteSchedule(id) {
  const target = schedules.find((s) => s.id === id);
  if (!target) return;
  if (confirm(`Are you sure you want to delete schedule: "${target.name}"?`)) {
    schedules = schedules.filter((s) => s.id !== id);
    markUnsaved();
    renderSchedules();
    showToast('Schedule removed. Click "Save & Apply" to commit changes.', true);
  }
}

function setScheduleLastSync(action) {
  const el = document.getElementById('modal_sch_last_sync');
  if (!el) return;
  if (action === 'now') {
    el.value = formatDateTimeForInput(new Date());
  } else if (action === 'clear') {
    el.value = '';
  }
}

function openScheduleModal(sch = null) {
  document.getElementById('scheduleEditForm').reset();
  if (sch) {
    document.getElementById('scheduleModalTitle').textContent = 'Edit Schedule';
    document.getElementById('modal_sch_id').value = sch.id;
    document.getElementById('modal_sch_name').value = sch.name;
    document.getElementById('modal_sch_cron').value = sch.cronSchedule;
    document.getElementById('modal_sch_enabled').checked = sch.enabled;
    
    const lastSyncEl = document.getElementById('modal_sch_last_sync');
    if (lastSyncEl) {
      lastSyncEl.value = parseToDateTimeInput(sch.lastSyncTime || '');
    }

    const serviceRadios = document.getElementsByName('sch_service');
    let targetType = sch.type || 'attendance';
    if (targetType === 'peopleimport') targetType = 'people';
    if (targetType === 'overtime' || targetType === 'access_records') targetType = 'attendance';

    for (const radio of serviceRadios) {
      if (radio.value === targetType) {
        radio.checked = true;
      }
    }
    updateCronPreview(sch.cronSchedule);
  } else {
    document.getElementById('scheduleModalTitle').textContent = 'Add Cron Schedule';
    document.getElementById('modal_sch_id').value = '';
    document.getElementById('modal_sch_name').value = 'Access Records Sync';
    document.getElementById('modal_sch_cron').value = '0 */2 * * *';
    document.getElementById('modal_sch_enabled').checked = true;
    const lastSyncEl = document.getElementById('modal_sch_last_sync');
    if (lastSyncEl) lastSyncEl.value = '';
    updateCronPreview('0 */2 * * *');
  }

  document.getElementById('scheduleModal').classList.remove('hidden');
}

function editSchedule(id) {
  const sch = schedules.find((s) => s.id === id);
  if (sch) {
    openScheduleModal(sch);
  }
}

function closeScheduleModal() {
  document.getElementById('scheduleModal').classList.add('hidden');
}

function handleScheduleFormSubmit(e) {
  e.preventDefault();
  const id = document.getElementById('modal_sch_id').value;
  const name = document.getElementById('modal_sch_name').value.trim();
  const cronSchedule = document.getElementById('modal_sch_cron').value.trim();
  const enabled = document.getElementById('modal_sch_enabled').checked;

  let type = 'attendance';
  const serviceRadios = document.getElementsByName('sch_service');
  for (const r of serviceRadios) {
    if (r.checked) type = r.value;
  }

  if (id) {
    const idx = schedules.findIndex((s) => s.id === id);
    if (idx !== -1) {
      schedules[idx] = { ...schedules[idx], id, name, type, cronSchedule, enabled };
    }
  } else {
    const newId = 'sch_' + Date.now();
    schedules.push({ id: newId, name, type, cronSchedule, enabled });
  }

  closeScheduleModal();
  markUnsaved();
  renderSchedules();
  showToast('Schedule updated! Click "Save & Apply" to apply.', true);
}

function markUnsaved() {
  const ind = document.getElementById('unsavedIndicator');
  if (ind) ind.classList.remove('hidden');
}

async function saveSchedulesToServer() {
  const saveBtn = document.getElementById('btn-save-all');
  saveBtn.disabled = true;
  saveBtn.innerHTML = '<span>Saving...</span>';
  log('Saving schedules configuration to disk and hot-reloading Scheduler...', 'running');

  try {
    const res = await fetch('/api/config', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ schedules })
    });
    const data = await res.json();

    if (res.ok) {
      log(data.message || 'Schedules saved & reloaded successfully.', 'success');
      showToast('Schedules saved & reloaded.', true);
      const unsavedBar = document.getElementById('unsavedIndicator');
      if (unsavedBar) unsavedBar.classList.add('hidden');
      if (data.schedules) {
        schedules = data.schedules;
        renderSchedules();
      }
    } else {
      log(`Error saving config: ${data.message || 'Validation error'}`, 'error');
      showToast(data.message || 'Save failed', false);
    }
  } catch (err) {
    log(`Network connection error: ${err.message}`, 'error');
    showToast(`Network error: ${err.message}`, false);
  } finally {
    saveBtn.disabled = false;
    saveBtn.innerHTML = '<span>Save Changes</span>';
  }
}

// 5. Activity Logging & Toast
function log(msg, type = 'info') {
  const consoleBox = document.getElementById('consoleLog');
  if (!consoleBox) return;
  const time = new Date().toLocaleTimeString('en-GB');
  const line = document.createElement('div');
  
  let tagClass = 'text-slate-400 bg-slate-800 border-slate-700';
  let tagText = 'INFO';
  let msgColor = 'text-slate-300';

  if (type === 'success') {
    tagClass = 'text-emerald-400 bg-emerald-950/60 border-emerald-800/60';
    tagText = 'SUCCESS';
    msgColor = 'text-emerald-300';
  } else if (type === 'error') {
    tagClass = 'text-rose-400 bg-rose-950/60 border-rose-800/60';
    tagText = 'ERROR';
    msgColor = 'text-rose-300';
  } else if (type === 'running') {
    tagClass = 'text-cyan-400 bg-cyan-950/60 border-cyan-800/60';
    tagText = 'EXEC';
    msgColor = 'text-cyan-200';
  }

  line.className = 'flex items-start gap-2 py-0.5 text-xs font-mono';
  line.innerHTML = `
    <span class="text-slate-500 text-[11px] shrink-0">[${time}]</span>
    <span class="text-[9px] font-bold px-1.5 py-0.2 rounded border uppercase tracking-wider shrink-0 ${tagClass}">${tagText}</span>
    <span class="${msgColor} break-all">${msg}</span>
  `;
  consoleBox.appendChild(line);
  consoleBox.scrollTop = consoleBox.scrollHeight;
}

function clearLog() {
  const consoleBox = document.getElementById('consoleLog');
  if (consoleBox) {
    consoleBox.innerHTML = '<div class="text-slate-500">[System] Log cleared. Ready.</div>';
  }
}

function showToast(msg, isSuccess) {
  const t = document.getElementById('toast');
  if (!t) return;
  t.textContent = msg;
  t.className = `fixed bottom-6 right-6 px-4 py-2.5 rounded-lg text-xs font-semibold shadow-lg transition-all z-50 flex items-center gap-2 ${isSuccess ? 'bg-slate-900 text-emerald-400 border border-emerald-800' : 'bg-slate-900 text-rose-400 border border-rose-800'}`;
  t.classList.remove('hidden');
  setTimeout(() => t.classList.add('hidden'), 3500);
}

function openModal(title, subtitle, metaHTML, rawData) {
  document.getElementById('modalTitle').textContent = title;
  document.getElementById('modalSubtitle').textContent = subtitle;
  document.getElementById('modalMetaBar').innerHTML = metaHTML;
  document.getElementById('modalJson').textContent = JSON.stringify(rawData, null, 2);
  currentRawData = rawData;
  document.getElementById('dataModal').classList.remove('hidden');
}

function closeModal() {
  document.getElementById('dataModal').classList.add('hidden');
}

function copyModalData() {
  if (currentRawData) {
    navigator.clipboard.writeText(JSON.stringify(currentRawData, null, 2));
    showToast('Raw JSON copied to clipboard.', true);
  }
}

// 6. Test Fetch Data (with Custom Range)
async function executeTestFetch(service, startVal, endVal) {
  const targetService = (service === 'overtime') ? 'attendance' : service;
  const btn = document.getElementById(`btn-test-${service}`);
  const origHTML = btn ? btn.innerHTML : '';
  if (btn) {
    btn.disabled = true;
    btn.innerHTML = '<span>Testing...</span>';
  }

  let q = '';
  if (startVal && endVal) {
    q = `?startDateTime=${encodeURIComponent(startVal)}&endDateTime=${encodeURIComponent(endVal)}`;
  }
  const rangeLabel = startVal && endVal ? ` (${startVal.replace('T', ' ')} to ${endVal.replace('T', ' ')})` : '';
  log(`Fetching test preview from ${service} API${rangeLabel}...`, 'running');

  try {
    const res = await fetch(`/api/test/${targetService}${q}`);
    const data = await res.json();

    const statusBadge = data.statusCode >= 200 && data.statusCode < 300 
      ? `<span class="px-2 py-0.5 rounded bg-emerald-100 text-emerald-800 font-bold">HTTP ${data.statusCode} OK</span>`
      : `<span class="px-2 py-0.5 rounded bg-rose-100 text-rose-800 font-bold">HTTP ${data.statusCode} ERROR</span>`;

    let filterBadge = '';
    if (data.attendanceRecords !== undefined) {
      filterBadge += `<span class="px-2 py-0.5 rounded bg-blue-100 text-blue-800 font-bold">Normal Gate: ${data.attendanceRecords}</span>`;
    }
    if (data.matchedOTCount !== undefined) {
      filterBadge += `<span class="px-2 py-0.5 rounded bg-amber-100 text-amber-800 font-bold">OT Gate: ${data.matchedOTCount}</span>`;
    }

    const metaHTML = `
      ${statusBadge}
      <span class="text-slate-700 font-bold">${data.httpMethod || 'GET'} ${data.endpoint || ''}</span>
      <span class="text-slate-500">Latency: ${data.latencyMs || 0}ms</span>
      ${data.recordCount !== undefined ? `<span class="px-2 py-0.5 rounded bg-purple-100 text-purple-800 font-bold">Total Records: ${data.recordCount}</span>` : ''}
      ${data.totalAccessCount !== undefined ? `<span class="px-2 py-0.5 rounded bg-slate-200 text-slate-800 font-bold">Total Fetched: ${data.totalAccessCount}</span>` : ''}
      ${filterBadge}
      ${data.timeRange ? `<span class="text-slate-600">Range: ${data.timeRange}</span>` : ''}`;

    if (res.ok) {
      log(`Preview ${service} Success (HTTP ${data.statusCode}, Records: ${data.recordCount || data.totalAccessCount || 0})`, 'success');
      showToast('Preview loaded successfully.', true);
    } else {
      log(`Test ${service} returned HTTP ${data.statusCode}: ${data.error || 'Check response body'}`, 'error');
      showToast(`Test returned HTTP ${data.statusCode}`, false);
    }

    openModal(
      `${service.toUpperCase()} API Preview`,
      data.endpoint || 'API Request details',
      metaHTML,
      data.rawResponse || data
    );

  } catch (err) {
    log(`Network connection error: ${err.message}`, 'error');
    showToast(`Network error: ${err.message}`, false);
  } finally {
    if (btn) {
      btn.disabled = false;
      btn.innerHTML = origHTML;
    }
  }
}

// 7. Test Connection / Auth
async function testConnection() {
  const btn = document.getElementById('btn-test-conn');
  btn.disabled = true;
  btn.innerHTML = '<span>Testing...</span>';

  log('Testing connection & authentication with CentAccess App...', 'running');

  try {
    const res = await fetch('/api/test/connection');
    const data = await res.json();

    const statusBadge = data.statusCode >= 200 && data.statusCode < 300 
      ? `<span class="px-2 py-0.5 rounded bg-emerald-100 text-emerald-800 font-bold">HTTP ${data.statusCode} OK</span>`
      : `<span class="px-2 py-0.5 rounded bg-rose-100 text-rose-800 font-bold">HTTP ${data.statusCode} Failed</span>`;

    const metaHTML = `
      ${statusBadge}
      <span class="text-slate-700 font-bold">${data.httpMethod || 'POST'} ${data.endpoint || ''}</span>
      <span class="text-slate-500">Latency: ${data.latencyMs || 0}ms</span>
      ${data.username ? `<span class="text-slate-600">User: ${data.username}</span>` : ''}`;

    if (res.ok) {
      log(`Auth Connection OK (HTTP ${data.statusCode})`, 'success');
      showToast('Authentication Successful.', true);
    } else {
      log(`Auth Connection Failed (HTTP ${data.statusCode})`, 'error');
      showToast(`Auth Failed (HTTP ${data.statusCode})`, false);
    }

    openModal(
      'CentAccess Auth API Response',
      data.endpoint || 'POST /api/login',
      metaHTML,
      data.rawResponse || data
    );

  } catch (err) {
    log(`Network error: ${err.message}`, 'error');
    showToast(`Network error: ${err.message}`, false);
  } finally {
    btn.disabled = false;
    btn.innerHTML = `
      <svg class="w-3.5 h-3.5 text-slate-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"></path>
      </svg>
      <span>Test Auth</span>
    `;
  }
}

// 8. Manual Sync Trigger (with Custom Date-Time Range)
async function executeSync(action, startVal = '', endVal = '') {
  const targetAction = (action === 'overtime') ? 'attendance' : action;

  const btnMap = {
    'attendance': document.getElementById('btn-sync-attendance'),
    'help': document.getElementById('btn-sync-help'),
    'people': document.getElementById('btn-sync-people'),
    'all': document.getElementById('btn-sync-all')
  };

  const btn = btnMap[action] || btnMap[targetAction];
  const origText = btn ? btn.innerHTML : '';
  if (btn) {
    btn.disabled = true;
    btn.innerHTML = '<span>Syncing...</span>';
  }

  let q = '';
  if (startVal && endVal) {
    q = `?startDateTime=${encodeURIComponent(startVal)}&endDateTime=${encodeURIComponent(endVal)}`;
  }
  const rangeLabel = startVal && endVal ? ` (${startVal.replace('T', ' ')} to ${endVal.replace('T', ' ')})` : '';
  log(`Executing manual ${action} sync${rangeLabel}...`, 'running');

  try {
    const res = await fetch(`/api/sync/${targetAction}${q}`, { method: 'POST' });
    const data = await res.json();
    
    if (res.ok) {
      log(data.message || `${action} sync completed successfully.`, 'success');
      showToast(data.message || `${action} sync completed.`, true);
      if (data.time) {
        const timeSpan = document.getElementById('time-' + action);
        if (timeSpan) timeSpan.textContent = `Last: ${data.time}`;
      }
    } else {
      log(data.message || `Error during ${action} sync`, 'error');
      showToast(data.message || 'Sync failed', false);
    }
  } catch (err) {
    log(`Connection error: ${err.message}`, 'error');
    showToast(`Network error: ${err.message}`, false);
  } finally {
    if (btn) {
      btn.disabled = false;
      btn.innerHTML = origText;
    }
  }
}

// Init on page load & listen for hash changes
document.addEventListener('DOMContentLoaded', loadInitialData);
window.addEventListener('hashchange', () => {
  const hash = window.location.hash.replace('#', '');
  if (['dashboard', 'devices', 'schedules', 'logs'].includes(hash)) {
    switchPage(hash);
  }
});
