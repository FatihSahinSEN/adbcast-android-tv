let activeServerPort = '9468';
let activeHost = '127.0.0.1';
let SERVER_URL = `http://${activeHost}:${activeServerPort}`;
let currentTab = null;
let isBackendOnline = false;
let selectedMediaFiles = [];

document.addEventListener('DOMContentLoaded', async () => {
  const statusDiv = document.getElementById('status');

  // Load saved backend port safely
  try {
    if (typeof chrome !== 'undefined' && chrome.storage && chrome.storage.local) {
      const stored = await chrome.storage.local.get(['activeServerPort', 'activeHost']);
      if (stored && stored.activeServerPort) {
        activeServerPort = stored.activeServerPort;
      }
      if (stored && stored.activeHost) {
        activeHost = stored.activeHost;
      }
    }
  } catch (err) {
    console.warn('Storage read error:', err);
  }

  SERVER_URL = `http://${activeHost}:${activeServerPort}`;
  const srvInput = document.getElementById('serverPortInput');
  if (srvInput) srvInput.value = activeServerPort;

  // Load cached TV devices from storage
  await loadCachedDevices();

  // 1. Tab Navigation Handlers
  setupTabs();

  // 2. Context detection (YouTube vs Web Browser vs Invalid Page)
  try {
    if (typeof chrome !== 'undefined' && chrome.tabs) {
      let [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
      currentTab = tab;
    }
  } catch (err) {
    console.warn('Tab query error:', err);
  }

  const youtubeSection = document.getElementById('youtubeSection');
  const browserSection = document.getElementById('browserSection');
  const noWebPageNotice = document.getElementById('noWebPageNotice');

  if (currentTab && currentTab.url) {
    const isYouTube = currentTab.url.includes("youtube.com/watch");
    const isWebPage = currentTab.url.startsWith("http");

    if (isYouTube) {
      if (youtubeSection) youtubeSection.style.display = 'block';
      if (browserSection) browserSection.style.display = 'none';
      if (noWebPageNotice) noWebPageNotice.style.display = 'none';
    } else if (isWebPage) {
      if (browserSection) browserSection.style.display = 'block';
      if (youtubeSection) youtubeSection.style.display = 'none';
      if (noWebPageNotice) noWebPageNotice.style.display = 'none';
    } else {
      if (youtubeSection) youtubeSection.style.display = 'none';
      if (browserSection) browserSection.style.display = 'none';
      if (noWebPageNotice) noWebPageNotice.style.display = 'block';
    }
  } else {
    if (youtubeSection) youtubeSection.style.display = 'none';
    if (browserSection) browserSection.style.display = 'none';
    if (noWebPageNotice) noWebPageNotice.style.display = 'block';
  }

  // 3. Media Cast listeners
  const selectMediaBtn = document.getElementById('selectMediaBtn');
  const mediaFileInput = document.getElementById('mediaFileInput');
  const sendMediaBtn = document.getElementById('sendMediaBtn');
  const clearMediaBtn = document.getElementById('clearMediaBtn');

  if (selectMediaBtn && mediaFileInput) {
    selectMediaBtn.addEventListener('click', () => {
      mediaFileInput.click();
    });

    mediaFileInput.addEventListener('change', (e) => {
      const files = Array.from(e.target.files);
      let invalidCount = 0;

      files.forEach(file => {
        const isImg = file.type.startsWith('image/') || /\.(jpg|jpeg|png|gif|webp|bmp)$/i.test(file.name);
        const isVid = file.type.startsWith('video/') || /\.(mp4|mkv|avi|mov|webm|3gp)$/i.test(file.name);

        if (isImg || isVid) {
          if (!selectedMediaFiles.some(f => f.name === file.name && f.size === file.size)) {
            selectedMediaFiles.push(file);
          }
        } else {
          invalidCount++;
        }
      });

      if (invalidCount > 0) {
        statusDiv.innerHTML = `<span class="error-msg">⚠️ ${invalidCount} unsupported file(s) skipped. Only photos and videos are allowed!</span>`;
      } else {
        statusDiv.innerHTML = '';
      }

      renderMediaList();
      mediaFileInput.value = '';
    });
  }

  if (sendMediaBtn) {
    sendMediaBtn.addEventListener('click', sendMediaToTV);
  }

  if (clearMediaBtn) {
    clearMediaBtn.addEventListener('click', async () => {
      if (!isBackendOnline) return;
      if (statusDiv) statusDiv.innerHTML = '<span>⏳ Clearing media from PC and TV...</span>';

      try {
        let res = await fetch(`${SERVER_URL}/clear_media`, { method: 'POST' });
        if (res.ok) {
          selectedMediaFiles = [];
          renderMediaList();
          loadServerMediaList();
          if (statusDiv) statusDiv.innerHTML = '<span class="success-msg">✅ All media cleared from PC & TV!</span>';
        } else {
          if (statusDiv) statusDiv.innerHTML = '<span class="error-msg">❌ Failed to clear media.</span>';
        }
      } catch (err) {
        if (statusDiv) statusDiv.innerHTML = '<span class="error-msg">❌ Server connection error!</span>';
      }
    });
  }

  const startSlideshowBtn = document.getElementById('startSlideshowBtn');
  const stopSlideshowBtn = document.getElementById('stopSlideshowBtn');

  if (startSlideshowBtn) {
    startSlideshowBtn.addEventListener('click', async () => {
      if (!isBackendOnline) return;
      const intervalSelect = document.getElementById('slideshowInterval');
      const interval = intervalSelect ? intervalSelect.value : '5';

      if (statusDiv) statusDiv.innerHTML = '<span>⏳ Starting slideshow loop...</span>';

      try {
        let res = await fetch(`${SERVER_URL}/start_slideshow?interval=${interval}`, { method: 'POST' });
        if (res.ok) {
          let json = await res.json();
          if (statusDiv) statusDiv.innerHTML = `<span class="success-msg">🔄 Slideshow loop started (${json.total_files} files, ${json.interval}s interval)!</span>`;
        } else {
          let errText = await res.text();
          if (statusDiv) statusDiv.innerHTML = `<span class="error-msg">❌ ${errText}</span>`;
        }
      } catch (err) {
        if (statusDiv) statusDiv.innerHTML = '<span class="error-msg">❌ Connection error!</span>';
      }
    });
  }

  if (stopSlideshowBtn) {
    stopSlideshowBtn.addEventListener('click', async () => {
      if (!isBackendOnline) return;

      try {
        let res = await fetch(`${SERVER_URL}/stop_slideshow`, { method: 'POST' });
        if (res.ok) {
          if (statusDiv) statusDiv.innerHTML = '<span class="success-msg">⏹ Slideshow loop stopped!</span>';
        }
      } catch (err) {
        if (statusDiv) statusDiv.innerHTML = '<span class="error-msg">❌ Connection error!</span>';
      }
    });
  }

  const closeAllAppsBtn = document.getElementById('closeAllAppsBtn');
  if (closeAllAppsBtn) {
    closeAllAppsBtn.addEventListener('click', async () => {
      if (!isBackendOnline) return;
      if (statusDiv) statusDiv.innerHTML = '<span>⏳ Closing all TV apps & going Home...</span>';

      try {
        let res = await fetch(`${SERVER_URL}/close_all_apps`, { method: 'POST' });
        if (res.ok) {
          if (statusDiv) statusDiv.innerHTML = '<span class="success-msg">✅ All TV apps closed & returned to Home screen!</span>';
        } else {
          if (statusDiv) statusDiv.innerHTML = '<span class="error-msg">❌ Failed to close TV apps.</span>';
        }
      } catch (err) {
        if (statusDiv) statusDiv.innerHTML = '<span class="error-msg">❌ Server connection error!</span>';
      }
    });
  }

  const deviceSelect = document.getElementById('deviceSelect');
  if (deviceSelect) {
    deviceSelect.addEventListener('change', (e) => {
      const selectedIP = e.target.value;
      const selectedOpt = e.target.options[e.target.selectedIndex];
      const port = selectedOpt ? (selectedOpt.getAttribute('data-port') || '5555') : '5555';
      if (selectedIP) {
        selectDevice(selectedIP, port);
      }
    });
  }

  const scanNetworkBtn = document.getElementById('scanNetworkBtn');
  if (scanNetworkBtn) {
    scanNetworkBtn.addEventListener('click', scanNetworkForTVs);
  }

  const selectAllTvsBtn = document.getElementById('selectAllTvsBtn');
  const deselectAllTvsBtn = document.getElementById('deselectAllTvsBtn');

  if (selectAllTvsBtn) {
    selectAllTvsBtn.addEventListener('click', () => {
      document.querySelectorAll('.tv-cb').forEach(cb => cb.checked = true);
      updateDeviceSelectionFromCheckboxes();
    });
  }

  if (deselectAllTvsBtn) {
    deselectAllTvsBtn.addEventListener('click', () => {
      document.querySelectorAll('.tv-cb').forEach(cb => cb.checked = false);
      updateDeviceSelectionFromCheckboxes();
    });
  }

  // 4. Health check, Port Discovery & Fetch Config
  await checkBackendAndLoadConfig();
});

// Device Management & Storage Helper Functions (Multi-Select Checkboxes)
async function loadCachedDevices() {
  const subnetInput = document.getElementById('subnetInput');
  try {
    if (typeof chrome !== 'undefined' && chrome.storage && chrome.storage.local) {
      const stored = await chrome.storage.local.get(['discoveredDevices', 'selectedDeviceIPs', 'subnetPrefix']);
      const devices = stored.discoveredDevices || [];
      const selectedIPs = stored.selectedDeviceIPs || [];

      if (subnetInput && stored.subnetPrefix) {
        subnetInput.value = stored.subnetPrefix;
      }

      populateDeviceCheckboxes(devices, selectedIPs);
    }
  } catch (e) {
    console.warn('Load cached devices error:', e);
  }
}

function populateDeviceCheckboxes(devices, selectedIPs) {
  const container = document.getElementById('tvDeviceListContainer');
  const countBadge = document.getElementById('selectedTvCount');
  const ipInput = document.getElementById('ipInput');
  if (!container) return;

  container.innerHTML = '';

  if (!devices || devices.length === 0) {
    container.innerHTML = `
      <div style="text-align: center; padding: 12px; color: var(--text-muted); font-size: 11px;">
        ⚠️ No TV devices found on LAN.<br>Click <b>Scan Network for TVs</b> button above.
      </div>
    `;
    if (countBadge) countBadge.innerText = '0 Selected';
    return;
  }

  if (!selectedIPs || selectedIPs.length === 0) {
    selectedIPs = devices.map(d => d.ip);
  }

  let activeCount = 0;

  devices.forEach(d => {
    const isChecked = selectedIPs.includes(d.ip);
    if (isChecked) activeCount++;

    const item = document.createElement('div');
    item.className = 'media-item';
    item.style.cursor = 'pointer';
    item.style.padding = '8px 10px';

    item.innerHTML = `
      <div style="display: flex; align-items: center; gap: 8px; width: 100%;">
        <input type="checkbox" class="tv-cb" value="${d.ip}" ${isChecked ? 'checked' : ''} style="cursor: pointer; width: 14px; height: 14px;">
        <div style="flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">
          <div style="font-weight: bold; color: var(--text-color); font-size: 11px;">${d.name || d.model || 'Android TV'}</div>
          <div style="font-size: 9px; color: var(--text-muted);">${d.ip}:${d.port || '5555'}</div>
        </div>
        <span class="media-badge badge-video" style="background: #22c55e; color: white; font-size: 8px; padding: 2px 6px;">Online</span>
      </div>
    `;

    const cb = item.querySelector('.tv-cb');
    item.addEventListener('click', (e) => {
      if (e.target !== cb) {
        cb.checked = !cb.checked;
      }
      updateDeviceSelectionFromCheckboxes();
    });

    cb.addEventListener('change', () => {
      updateDeviceSelectionFromCheckboxes();
    });

    container.appendChild(item);
  });

  if (countBadge) countBadge.innerText = `${activeCount} / ${devices.length} Selected`;

  if (ipInput && devices.length > 0) {
    ipInput.value = devices[0].ip;
  }

  selectDevicesMulti(selectedIPs);
}

function updateDeviceSelectionFromCheckboxes() {
  const checkboxes = document.querySelectorAll('.tv-cb');
  const countBadge = document.getElementById('selectedTvCount');
  const checkedIPs = [];

  checkboxes.forEach(cb => {
    if (cb.checked) {
      checkedIPs.push(cb.value);
    }
  });

  if (countBadge) countBadge.innerText = `${checkedIPs.length} / ${checkboxes.length} Selected`;
  saveStorage({ selectedDeviceIPs: checkedIPs });
  selectDevicesMulti(checkedIPs);
}

async function selectDevicesMulti(checkedIPs) {
  if (!isBackendOnline) return;
  const statusDiv = document.getElementById('status');
  const ipsParam = checkedIPs ? checkedIPs.join(',') : '';

  try {
    let res = await fetch(`${SERVER_URL}/select_devices?ips=${encodeURIComponent(ipsParam)}`, { method: 'POST' });
    if (res.ok) {
      if (statusDiv && checkedIPs.length > 0) {
        statusDiv.innerHTML = `<span class="success-msg">✅ Commands active for ${checkedIPs.length} TV device(s)</span>`;
      }
    }
  } catch (e) {
    console.warn('Select devices API error:', e);
  }
}

async function scanNetworkForTVs() {
  const statusDiv = document.getElementById('status');
  const scanBtn = document.getElementById('scanNetworkBtn');
  const subnetInput = document.getElementById('subnetInput');
  const subnet = subnetInput ? subnetInput.value.trim() : '';

  if (!isBackendOnline) return;

  if (scanBtn) scanBtn.disabled = true;
  if (statusDiv) statusDiv.innerHTML = `<span>🔍 Scanning local network ${subnet ? '(' + subnet + '0/24)' : ''}...</span>`;

  try {
    let url = `${SERVER_URL}/scan_network`;
    if (subnet) {
      url += `?subnet=${encodeURIComponent(subnet)}`;
    }
    let res = await fetch(url);
    if (res.ok) {
      let devices = await res.json();
      saveStorage({ discoveredDevices: devices, subnetPrefix: subnet });

      if (devices && devices.length > 0) {
        const allIPs = devices.map(d => d.ip);
        saveStorage({ selectedDeviceIPs: allIPs });
        populateDeviceCheckboxes(devices, allIPs);
        if (statusDiv) statusDiv.innerHTML = `<span class="success-msg">✅ Found ${devices.length} TV device(s) on network!</span>`;
      } else {
        populateDeviceCheckboxes([], []);
        if (statusDiv) statusDiv.innerHTML = '<span class="error-msg">⚠️ No Android TV devices found on port 5555.</span>';
      }
    } else {
      if (statusDiv) statusDiv.innerHTML = '<span class="error-msg">❌ Network scan failed.</span>';
    }
  } catch (err) {
    if (statusDiv) statusDiv.innerHTML = '<span class="error-msg">❌ Server connection error!</span>';
  } finally {
    if (scanBtn) scanBtn.disabled = false;
  }
}

function setupTabs() {
  const tabBtns = document.querySelectorAll('.tab-btn');
  const tabContents = document.querySelectorAll('.tab-content');

  tabBtns.forEach(btn => {
    btn.addEventListener('click', () => {
      const targetTabId = btn.getAttribute('data-tab');

      tabBtns.forEach(b => b.classList.remove('active'));
      tabContents.forEach(c => c.classList.remove('active'));

      btn.classList.add('active');
      const targetEl = document.getElementById(targetTabId);
      if (targetEl) targetEl.classList.add('active');
    });
  });
}

function renderMediaList() {
  const mediaList = document.getElementById('mediaList');
  const sendMediaBtn = document.getElementById('sendMediaBtn');

  if (!mediaList || !sendMediaBtn) return;
  mediaList.innerHTML = '';

  if (selectedMediaFiles.length === 0) {
    sendMediaBtn.style.display = 'none';
    return;
  }

  sendMediaBtn.style.display = 'flex';
  sendMediaBtn.innerText = `📺 Cast to TV (${selectedMediaFiles.length} ${selectedMediaFiles.length === 1 ? 'File' : 'Files'})`;

  selectedMediaFiles.forEach((file, index) => {
    const isVid = file.type.startsWith('video/') || /\.(mp4|mkv|avi|mov|webm|3gp)$/i.test(file.name);
    const item = document.createElement('div');
    item.className = 'media-item';

    const badgeClass = isVid ? 'badge-video' : 'badge-image';
    const badgeText = isVid ? 'Video' : 'Photo';
    const formattedSize = (file.size / (1024 * 1024)).toFixed(1) + ' MB';

    item.innerHTML = `
      <div class="media-info" title="${file.name}">
        <span class="media-badge ${badgeClass}">${badgeText}</span>
        <span>${file.name} (${formattedSize})</span>
      </div>
      <button class="remove-btn" title="Remove">✕</button>
    `;

    item.querySelector('.remove-btn').addEventListener('click', (e) => {
      e.stopPropagation();
      selectedMediaFiles.splice(index, 1);
      renderMediaList();
    });

    mediaList.appendChild(item);
  });
}

async function sendMediaToTV() {
  if (!isBackendOnline) return;
  if (selectedMediaFiles.length === 0) return;

  const statusDiv = document.getElementById('status');
  const sendMediaBtn = document.getElementById('sendMediaBtn');
  
  sendMediaBtn.disabled = true;
  statusDiv.innerHTML = '<span>⏳ Transferring media to TV (ADB Push)...</span>';

  try {
    const formData = new FormData();
    selectedMediaFiles.forEach(file => {
      formData.append('media', file);
    });

    const response = await fetch(`${SERVER_URL}/cast_media`, {
      method: 'POST',
      body: formData
    });

    if (response.ok) {
      statusDiv.innerHTML = `<span class="success-msg">✅ ${selectedMediaFiles.length} media file(s) launched on TV!</span>`;
      selectedMediaFiles = [];
      renderMediaList();
      loadServerMediaList();
    } else {
      const errText = await response.text();
      statusDiv.innerHTML = `<span class="error-msg">❌ Media transfer error: ${errText}</span>`;
    }
  } catch (err) {
    statusDiv.innerHTML = '<span class="error-msg">❌ Server connection error!</span>';
  } finally {
    sendMediaBtn.disabled = false;
  }
}

async function loadServerMediaList() {
  const section = document.getElementById('serverMediaSection');
  const container = document.getElementById('serverMediaList');
  const statusDiv = document.getElementById('status');

  if (!section || !container || !isBackendOnline) return;

  try {
    let res = await fetch(`${SERVER_URL}/list_uploaded_media`);
    if (res.ok) {
      let files = await res.json();
      container.innerHTML = '';

      if (!files || files.length === 0) {
        section.style.display = 'none';
        return;
      }

      section.style.display = 'block';

      files.forEach(file => {
        const item = document.createElement('div');
        item.className = 'media-item';

        const isVid = file.type === 'video';
        const badgeClass = isVid ? 'badge-video' : 'badge-image';
        const badgeText = isVid ? 'Video' : 'Photo';
        const formattedSize = (file.size / (1024 * 1024)).toFixed(1) + ' MB';

        item.innerHTML = `
          <div class="media-info" title="${file.filename}">
            <span class="media-badge ${badgeClass}">${badgeText}</span>
            <span>${file.filename} (${formattedSize})</span>
          </div>
          <div style="display: flex; gap: 4px; align-items: center;">
            <button class="play-btn" style="background: var(--primary-accent); color: white; border: none; border-radius: 4px; padding: 2px 6px; font-size: 9px; cursor: pointer;" title="Play on TV">▶</button>
            <button class="delete-btn" style="background: none; border: none; color: var(--error-red); cursor: pointer; font-size: 11px; font-weight: bold;" title="Delete File">✕</button>
          </div>
        `;

        item.querySelector('.play-btn').addEventListener('click', async (e) => {
          e.stopPropagation();
          if (statusDiv) statusDiv.innerText = `Launching ${file.filename} on TV...`;
          try {
            let pRes = await fetch(`${SERVER_URL}/play_media?filename=${encodeURIComponent(file.filename)}`);
            if (pRes.ok) {
              if (statusDiv) statusDiv.innerHTML = `<span class="success-msg">✅ Opened ${file.filename} on TV!</span>`;
            } else {
              if (statusDiv) statusDiv.innerHTML = '<span class="error-msg">❌ Play failed!</span>';
            }
          } catch (err) {
            if (statusDiv) statusDiv.innerHTML = '<span class="error-msg">❌ Connection error!</span>';
          }
        });

        item.querySelector('.delete-btn').addEventListener('click', async (e) => {
          e.stopPropagation();
          if (statusDiv) statusDiv.innerText = `Deleting ${file.filename}...`;
          try {
            let dRes = await fetch(`${SERVER_URL}/delete_media?filename=${encodeURIComponent(file.filename)}`);
            if (dRes.ok) {
              if (statusDiv) statusDiv.innerHTML = `<span class="success-msg">✅ Deleted ${file.filename}!</span>`;
              loadServerMediaList();
            }
          } catch (err) {
            if (statusDiv) statusDiv.innerHTML = '<span class="error-msg">❌ Delete failed!</span>';
          }
        });

        container.appendChild(item);
      });
    }
  } catch (err) {
    if (section) section.style.display = 'none';
  }
}

async function syncSlideshowStatus() {
  if (!isBackendOnline) return;
  const statusDiv = document.getElementById('status');
  try {
    let res = await fetch(`${SERVER_URL}/slideshow_status`);
    if (res.ok) {
      let st = await res.json();
      if (st.is_running) {
        if (statusDiv) {
          statusDiv.innerHTML = `<span class="success-msg">🔄 Slideshow active on TV (${st.total_files} files, ${st.interval}s interval)</span>`;
        }
      }
    }
  } catch (err) {
    // Ignore warning
  }
}

async function checkBackendAndLoadConfig() {
  const statusDot = document.getElementById('statusDot');
  const statusText = document.getElementById('statusText');
  const statusDiv = document.getElementById('status');
  const ipInput = document.getElementById('ipInput');
  const portInput = document.getElementById('portInput');

  statusDot.className = 'dot';
  statusText.innerText = 'Checking...';

  // 1. Try saved host & port first (127.0.0.1 or localhost)
  let success = await tryConnectServer(activeHost, activeServerPort, ipInput, portInput);
  
  // 2. If saved combination failed, try fallback host on same port
  if (!success) {
    let altHost = (activeHost === '127.0.0.1') ? 'localhost' : '127.0.0.1';
    success = await tryConnectServer(altHost, activeServerPort, ipInput, portInput);
    if (success) {
      activeHost = altHost;
    }
  }

  // 3. If port failed, perform fast parallel auto-discovery scan across candidate ports
  if (!success) {
    statusText.innerText = 'Scanning...';
    let discovered = await autoDiscoverFast();
    if (discovered) {
      activeHost = discovered.host;
      activeServerPort = discovered.port;
      SERVER_URL = `http://${activeHost}:${activeServerPort}`;
      
      const srvInput = document.getElementById('serverPortInput');
      if (srvInput) srvInput.value = activeServerPort;
      
      saveStorage({ activeServerPort, activeHost });
      success = await tryConnectServer(activeHost, activeServerPort, ipInput, portInput);
    }
  }

  if (success) {
    isBackendOnline = true;
    statusDot.className = 'dot online';
    statusText.innerText = `Online (Port ${activeServerPort})`;
    toggleInputs(true);
    loadServerMediaList();
    syncSlideshowStatus();

    if (currentTab && currentTab.url && currentTab.url.startsWith("http") && !currentTab.url.includes("youtube.com/watch")) {
      loadTVBrowsers();
    }
  } else {
    isBackendOnline = false;
    statusDot.className = 'dot offline';
    statusText.innerText = 'Offline';
    toggleInputs(false);
    if (statusDiv) {
      statusDiv.innerHTML = '<span class="error-msg">❌ Go backend server offline (Port ' + activeServerPort + ').</span>';
    }
  }
}

async function tryConnectServer(host, port, ipInput, portInput) {
  try {
    let url = `http://${host}:${port}/get_config`;
    let response = await fetch(url, { signal: AbortSignal.timeout(1000) });
    if (response.ok) {
      let cfg = await response.json();
      if (ipInput) ipInput.value = cfg.ip_address || '';
      if (portInput) portInput.value = cfg.port || '5555';
      if (cfg.server_port) {
        activeServerPort = cfg.server_port;
        activeHost = host;
        SERVER_URL = `http://${activeHost}:${activeServerPort}`;
        const srvInput = document.getElementById('serverPortInput');
        if (srvInput) srvInput.value = activeServerPort;
        saveStorage({ activeServerPort, activeHost });
      }
      return true;
    }
  } catch (err) {
    return false;
  }
  return false;
}

// Fast Parallel Auto-Discovery Scanner
async function autoDiscoverFast() {
  const candidatePorts = [activeServerPort, '9468', '9469', '9470', '9471', '9472', '9473', '9474', '9475', '8080', '9000'];
  const uniquePorts = [...new Set(candidatePorts)];
  const hosts = ['127.0.0.1', 'localhost'];

  const fetchPromises = [];

  for (let port of uniquePorts) {
    for (let host of hosts) {
      fetchPromises.push(
        fetch(`http://${host}:${port}/get_config`, { signal: AbortSignal.timeout(600) })
          .then(res => res.ok ? res.json() : null)
          .then(json => json && (json.ip_address || json.port || json.server_port) ? { host, port } : null)
          .catch(() => null)
      );
    }
  }

  const results = await Promise.all(fetchPromises);
  const found = results.find(r => r !== null);
  return found || null;
}

function saveStorage(data) {
  try {
    if (typeof chrome !== 'undefined' && chrome.storage && chrome.storage.local) {
      chrome.storage.local.set(data);
    }
  } catch (e) {
    console.warn('Storage save warning:', e);
  }
}

function toggleInputs(enabled) {
  const btnYoutube = document.getElementById('sendYoutubeBtn');
  const btnBrowser = document.getElementById('sendBrowserBtn');
  const btnSave = document.getElementById('saveBtn');
  const btnSelectMedia = document.getElementById('selectMediaBtn');
  const btnSendMedia = document.getElementById('sendMediaBtn');

  if (btnYoutube) btnYoutube.disabled = !enabled;
  if (btnBrowser) btnBrowser.disabled = !enabled;
  if (btnSave) btnSave.disabled = !enabled;
  if (btnSelectMedia) btnSelectMedia.disabled = !enabled;
  if (btnSendMedia) btnSendMedia.disabled = !enabled;
}

async function loadTVBrowsers() {
  const select = document.getElementById('browserSelect');
  if (!select) return;
  try {
    let res = await fetch(`${SERVER_URL}/get_browsers`);
    if (res.ok) {
      let browsers = await res.json();
      select.innerHTML = '';
      browsers.forEach(b => {
        let opt = document.createElement('option');
        opt.value = b.package;
        opt.innerText = b.name;
        select.appendChild(opt);
      });
    }
  } catch (err) {
    select.innerHTML = '<option value="default">Default System Browser</option>';
  }
}

// Action Handlers
document.addEventListener('DOMContentLoaded', () => {
  const sendYoutubeBtn = document.getElementById('sendYoutubeBtn');
  const sendBrowserBtn = document.getElementById('sendBrowserBtn');
  const saveBtn = document.getElementById('saveBtn');

  if (sendYoutubeBtn) {
    sendYoutubeBtn.addEventListener('click', async () => {
      if (!isBackendOnline) return;
      const statusDiv = document.getElementById('status');
      if (statusDiv) statusDiv.innerText = "Launching YouTube TV...";

      try {
        let res = await fetch(`${SERVER_URL}/launch_intent?type=youtube&url=${encodeURIComponent(currentTab ? currentTab.url : '')}`);
        if (res.ok) {
          if (statusDiv) statusDiv.innerHTML = '<span class="success-msg">✅ Opened on TV!</span>';
        } else {
          if (statusDiv) statusDiv.innerHTML = '<span class="error-msg">❌ Launch failed! Check ADB connection.</span>';
        }
      } catch (err) {
        if (statusDiv) statusDiv.innerHTML = '<span class="error-msg">❌ Server connection lost!</span>';
      }
    });
  }

  if (sendBrowserBtn) {
    sendBrowserBtn.addEventListener('click', async () => {
      if (!isBackendOnline) return;
      const statusDiv = document.getElementById('status');
      const selectedPkg = document.getElementById('browserSelect').value;
      if (statusDiv) statusDiv.innerText = "Launching TV Browser...";

      try {
        let res = await fetch(`${SERVER_URL}/launch_intent?type=browser&url=${encodeURIComponent(currentTab ? currentTab.url : '')}&package=${encodeURIComponent(selectedPkg)}`);
        if (res.ok) {
          if (statusDiv) statusDiv.innerHTML = '<span class="success-msg">✅ Page opened on TV!</span>';
        } else {
          if (statusDiv) statusDiv.innerHTML = '<span class="error-msg">❌ Launch failed! Check ADB connection.</span>';
        }
      } catch (err) {
        if (statusDiv) statusDiv.innerHTML = '<span class="error-msg">❌ Server connection lost!</span>';
      }
    });
  }

  const saveSubnetBtn = document.getElementById('saveSubnetBtn');
  if (saveSubnetBtn) {
    saveSubnetBtn.addEventListener('click', async () => {
      const statusDiv = document.getElementById('status');
      const subnetInput = document.getElementById('subnetInput');
      const prefix = subnetInput ? subnetInput.value.trim() : '';

      saveStorage({ subnetPrefix: prefix });
      if (statusDiv) {
        statusDiv.innerHTML = `<span class="success-msg">✅ Target Subnet IP Block saved: ${prefix || 'Auto-detect'}</span>`;
      }
    });
  }

  if (saveBtn) {
    saveBtn.addEventListener('click', async () => {
      const statusDiv = document.getElementById('status');
      const newServerPort = document.getElementById('serverPortInput').value.trim() || '9468';

      try {
        if (statusDiv) statusDiv.innerText = "Saving configuration...";
        
        activeServerPort = newServerPort;
        SERVER_URL = `http://${activeHost}:${activeServerPort}`;
        saveStorage({ activeServerPort, activeHost });

        if (statusDiv) statusDiv.innerHTML = '<span class="success-msg">✅ Go Server port configuration saved!</span>';
        await checkBackendAndLoadConfig();
      } catch (err) {
        if (statusDiv) statusDiv.innerHTML = '<span class="error-msg">❌ Server connection error!</span>';
      }
    });
  }

  const iycWebLink = document.getElementById('iycWebLink');
  if (iycWebLink) {
    iycWebLink.addEventListener('click', (e) => {
      e.preventDefault();
      chrome.tabs.create({ url: 'https://www.iyc.com.tr/' });
    });
  }
});