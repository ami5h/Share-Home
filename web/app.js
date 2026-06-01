(function() {
  'use strict';

  // --- Theme ---

  var THEME_KEY = 'share-home-theme';

  function applyTheme(t) {
    if (t === 'dark') {
      document.documentElement.classList.add('theme-dark');
      document.documentElement.classList.remove('theme-light');
    } else if (t === 'light') {
      document.documentElement.classList.add('theme-light');
      document.documentElement.classList.remove('theme-dark');
    } else {
      document.documentElement.classList.remove('theme-dark', 'theme-light');
    }
    // Update icons
    var sunIcon = document.querySelector('.icon-sun');
    var moonIcon = document.querySelector('.icon-moon');
    if (sunIcon) sunIcon.style.display = t === 'dark' ? 'none' : '';
    if (moonIcon) moonIcon.style.display = t === 'dark' ? '' : 'none';
  }

  var savedTheme = localStorage.getItem(THEME_KEY);
  if (savedTheme) {
    applyTheme(savedTheme);
  }

  document.getElementById('theme-btn').addEventListener('click', function() {
    var current = localStorage.getItem(THEME_KEY) || '';
    var isDark = document.documentElement.classList.contains('theme-dark');
    var isLight = document.documentElement.classList.contains('theme-light');
    var next;
    if (current === '') {
      // No saved preference — toggle from current visual state
      next = isDark ? 'light' : 'dark';
    } else if (current === 'dark') {
      next = 'light';
    } else {
      // current === 'light'
      next = 'dark';
    }
    if (next) localStorage.setItem(THEME_KEY, next);
    else localStorage.removeItem(THEME_KEY);
    applyTheme(next);
  });

  // --- Auth Token ---

  var TOKEN_KEY = 'share-home-token';

  function getToken() {
    return localStorage.getItem(TOKEN_KEY) || '';
  }
  function setToken(t) {
    if (t) localStorage.setItem(TOKEN_KEY, t);
    else localStorage.removeItem(TOKEN_KEY);
  }

  function needsAuth() {
    return window.__CONFIG__ && __CONFIG__.authRequired;
  }

  function showAuthModal() {
    var modal = document.getElementById('auth-modal');
    if (modal) modal.classList.remove('hidden');
  }
  function hideAuthModal() {
    var modal = document.getElementById('auth-modal');
    if (modal) modal.classList.add('hidden');
  }

  // Auth form submit
  document.getElementById('auth-form').addEventListener('submit', function(e) {
    e.preventDefault();
    var input = document.getElementById('auth-input');
    var token = input.value.trim();
    if (!token) return;
    setToken(token);
    hideAuthModal();
    toast('Token saved');
    input.value = '';
    document.getElementById('auth-error').classList.add('hidden');
    loadHistory();
  });

  function authHeader() {
    var t = getToken();
    return t ? '?token=' + encodeURIComponent(t) : '';
  }

  // Show modal on first load if auth needed and no token saved
  if (needsAuth() && !getToken()) {
    showAuthModal();
  }

  // --- QR Code (via qrcode-generator by Arase) ---

  var qrOverlay = null;
  function showQR(url) {
    if (!qrOverlay) {
      qrOverlay = document.createElement('div');
      qrOverlay.className = 'qr-overlay';
      qrOverlay.innerHTML = '<div class="qr-card"><canvas id="qr-canvas"></canvas><p class="qr-url"></p><button class="btn-primary" id="qr-close">Close</button></div>';
      document.body.appendChild(qrOverlay);
      qrOverlay.querySelector('#qr-close').addEventListener('click', hideQR);
      qrOverlay.addEventListener('click', function(e) { if (e.target === qrOverlay) hideQR(); });
    }
    try {
      var qr = qrcode(0, 'L');
      qr.addData(url);
      qr.make();
      var count = qr.getModuleCount();
      var canvas = qrOverlay.querySelector('#qr-canvas');
      var ctx = canvas.getContext('2d');
      var quiet = 4;
      var total = count + quiet * 2;
      var scale = Math.max(4, Math.ceil(240 / total));
      canvas.width = total * scale;
      canvas.height = total * scale;
      ctx.fillStyle = '#fff';
      ctx.fillRect(0, 0, canvas.width, canvas.height);
      ctx.fillStyle = '#000';
      for (var y = 0; y < count; y++) for (var x = 0; x < count; x++) {
        if (qr.isDark(y, x)) ctx.fillRect((x + quiet) * scale, (y + quiet) * scale, scale, scale);
      }
    } catch (e) { toast('URL too long for QR', 'error'); return; }
    qrOverlay.querySelector('.qr-url').textContent = url;
    qrOverlay.classList.remove('hidden');
  }

  function hideQR() {
    if (qrOverlay) qrOverlay.classList.add('hidden');
  }

  // QR button icon
  function iconQR() {
    return '<svg width="15" height="15" viewBox="0 0 15 15" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="2" width="3.5" height="3.5" rx="0.5"/><rect x="9.5" y="2" width="3.5" height="3.5" rx="0.5"/><rect x="2" y="9.5" width="3.5" height="3.5" rx="0.5"/><rect x="9.5" y="9.5" width="1.2" height="1.2" rx="0.2"/><rect x="11.8" y="9.5" width="1.2" height="1.2" rx="0.2"/><rect x="9.5" y="11.8" width="1.2" height="1.2" rx="0.2"/><rect x="11.8" y="11.8" width="1.2" height="1.2" rx="0.2"/></svg>';
  }

  // --- Toast ---

  function toast(msg, type, dur) {
    type = type || 'success';
    dur = dur || 3000;
    var el = document.createElement('div');
    el.className = 'toast';
    var ico = type === 'success'
      ? '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><polyline points="4 8.5 7 11.5 12 4.5"/></svg>'
      : '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><line x1="4" y1="4" x2="12" y2="12"/><line x1="12" y1="4" x2="4" y2="12"/></svg>';
    el.innerHTML = '<span class="toast-icon ' + type + '">' + ico + '</span><span>' + esc(msg) + '</span>';
    document.getElementById('toasts').appendChild(el);
    setTimeout(function() { el.classList.add('out'); setTimeout(function(){ el.remove(); }, 300); }, dur);
  }

  // --- Copy ---

  function copyText(t) {
    if (navigator.clipboard && navigator.clipboard.writeText) return navigator.clipboard.writeText(t);
    var a = document.createElement('textarea');
    a.value = t; a.style.position = 'fixed'; a.style.left = '-9999px';
    document.body.appendChild(a); a.select(); document.execCommand('copy'); document.body.removeChild(a);
    return Promise.resolve();
  }

  // --- Progress ---

  var progressEl = document.getElementById('upload-progress');
  var progressBar = progressEl ? progressEl.querySelector('.progress-bar') : null;
  var progressLabel = progressEl ? progressEl.querySelector('.progress-label') : null;

  function showProgress(label, pct) {
    progressEl.classList.remove('hidden');
    if (progressLabel) progressLabel.textContent = label || 'Uploading...';
    if (progressBar && pct != null) progressBar.style.width = Math.min(100, pct) + '%';
  }
  function hideProgress() { progressEl.classList.add('hidden'); }
  function setProgress(pct) { if (progressBar) progressBar.style.width = Math.min(100, pct) + '%'; }

  // --- Tabs ---

  var activeTab = 'files';
  document.querySelectorAll('.tab').forEach(function(btn) {
    btn.addEventListener('click', function() {
      activeTab = this.dataset.tab;
      document.querySelectorAll('.tab').forEach(function(b) { b.classList.remove('active'); });
      this.classList.add('active');
      document.querySelectorAll('.tab-panel').forEach(function(p) { p.classList.add('hidden'); });
      document.getElementById('tab-' + activeTab).classList.remove('hidden');
    });
  });

  // --- History Data ---

  var filesData = [];
  var clipboardData = [];
  var linksData = [];
  var initialLoadDone = false;

  // Generate skeleton HTML for a given count
  function skeletonHTML(n) {
    var rows = '';
    var widths = ['long', 'medium', 'short'];
    for (var i = 0; i < n; i++) {
      rows += '<div class="skeleton-row">' +
        '<div class="skeleton-icon"></div>' +
        '<div class="skeleton-text">' +
          '<div class="skeleton-line ' + widths[i % 3] + '"></div>' +
          '<div class="skeleton-line short"></div>' +
        '</div></div>';
    }
    return rows;
  }

  function showSkeleton(id) {
    var el = document.getElementById(id);
    if (el && el.innerHTML === '') el.innerHTML = skeletonHTML(3);
    if (el) el.classList.remove('hidden');
  }
  function hideSkeleton(id) {
    var el = document.getElementById(id);
    if (el) { el.classList.add('hidden'); el.innerHTML = ''; }
  }

  // Show initial loading skeletons
  showSkeleton('files-skeleton');
  showSkeleton('clipboard-skeleton');
  showSkeleton('links-skeleton');

  // --- Search Filters ---

  var searchTerms = { files: '', clipboard: '', links: '' };

  var selectedIds = {};

  function toggleSelect(type, id) {
    if (selectedIds[id]) delete selectedIds[id];
    else selectedIds[id] = true;
    updateBulkUI();
    renderHistory();
  }

  function selectAllVisible() {
    if (activeTab === 'files' && filesData.length) {
      filesData.forEach(function(f) { selectedIds[f.id] = true; });
    } else if (activeTab === 'clipboard' && clipboardData.length) {
      clipboardData.forEach(function(c) { selectedIds[c.id] = true; });
    } else if (activeTab === 'links' && linksData.length) {
      linksData.forEach(function(u) { selectedIds[u.code] = true; });
    }
    updateBulkUI();
    renderHistory();
  }

  function updateBulkUI() {
    var count = Object.keys(selectedIds).length;
    var toolbar = document.getElementById('bulk-toolbar');
    if (count === 0) { toolbar.classList.add('hidden'); return; }
    toolbar.classList.remove('hidden');
    document.getElementById('bulk-count').textContent = count + ' selected';
    // Only show ZIP button for files
    document.getElementById('bulk-zip-btn').style.display = activeTab === 'files' ? '' : 'none';
  }

  function bulkDelete() {
    var count = Object.keys(selectedIds).length;
    if (!count || !confirm('Delete ' + count + ' items?')) return;
    var ids = Object.keys(selectedIds);
    selectedIds = {};
    var idx = 0;
    function next() {
      if (idx >= ids.length) { updateBulkUI(); loadHistory(); return; }
      var id = ids[idx++];
      // Try all three endpoints
      var paths = ['/api/files/' + id, '/api/clipboard/' + id, '/api/urls/' + id];
      var pIdx = 0;
      function tryPath() {
        if (pIdx >= paths.length) { next(); return; }
        fetch(paths[pIdx] + authHeader(), { method: 'DELETE' }).then(function(r) {
          if (r.ok || r.status === 200 || r.status === 204) { next(); }
          else { pIdx++; tryPath(); }
        }).catch(function() { pIdx++; tryPath(); });
      }
      tryPath();
    }
    next();
  }

  function bulkDownload() {
    var fileIds = Object.keys(selectedIds);
    if (!fileIds.length) return;
    var body = JSON.stringify({ ids: fileIds });
    fetch('/api/download/zip' + authHeader(), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: body
    }).then(function(r) {
      if (!r.ok) throw new Error('ZIP failed');
      return r.blob();
    }).then(function(blob) {
      var a = document.createElement('a');
      a.href = URL.createObjectURL(blob);
      a.download = 'share-home-batch.zip';
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(a.href);
      toast('Downloading ZIP');
    }).catch(function() { toast('ZIP download failed', 'error'); });
  }

  function renderHistory() {
    var fFilter = searchTerms.files;
    var files = filesData;
    if (fFilter) {
      var q = fFilter.toLowerCase();
      files = filesData.filter(function(f) { return f.name.toLowerCase().indexOf(q) >= 0; });
    }
    // Files
    var fl = document.getElementById('files-list');
    var fe = document.getElementById('files-empty');
    if (files.length === 0) { fe.style.display = ''; fl.innerHTML = ''; }
    else { fe.style.display = 'none'; fl.innerHTML = files.map(function(f) {
      var dl = f.downloads || 0;
      var expires = f.expires ? (' <span style="color:var(--text-tertiary)">· ' + esc(f.expires) + '</span>') : '';
      var checked = selectedIds[f.id] ? 'checked' : '';
      var fullUrl = esc(window.location.origin + f.url);
      return '<li class="' + checked + '">' +
        '<input type="checkbox" class="list-check" data-action="toggle" data-type="file" data-id="' + esc(f.id) + '" ' + (checked ? 'checked' : '') + '>' +
        '<div class="list-icon">' + getFileIcon(f.mime) + '</div>' +
        '<div class="list-info">' +
          '<div class="list-name" data-action="open" data-url="' + esc(f.url) + '">' + esc(f.name) + '</div>' +
          '<div class="list-meta">' + esc(fmtSize(f.size)) + (dl ? ' · ' + dl + ' DL' : '') + expires + '</div>' +
        '</div>' +
        '<div class="list-actions">' +
          '<button class="btn-icon" data-action="copy" data-url="' + fullUrl + '" title="Copy">' + iconCopy() + '</button>' +
          '<button class="btn-icon" data-action="qr" data-url="' + fullUrl + '" title="QR Code">' + iconQR() + '</button>' +
          '<button class="btn-icon delete" data-action="delete" data-type="file" data-id="' + esc(f.id) + '" title="Delete">' + iconDel() + '</button>' +
        '</div></li>';
    }).join(''); }

    // Clipboard
    var cFilter = searchTerms.clipboard;
    var clips = clipboardData;
    if (cFilter) {
      var qc = cFilter.toLowerCase();
      clips = clipboardData.filter(function(c) { return c.type.indexOf(qc) >= 0 || (c.name||'').toLowerCase().indexOf(qc) >= 0; });
    }
    var cll = document.getElementById('clipboard-list');
    var ce = document.getElementById('clipboard-empty');
    if (clips.length === 0) { ce.style.display = ''; cll.innerHTML = ''; }
    else { ce.style.display = 'none'; cll.innerHTML = clips.map(function(c) {
      var checked = selectedIds[c.id] ? 'checked' : '';
      var fullUrl = esc(window.location.origin + c.url);
      return '<li class="' + checked + '">' +
        '<input type="checkbox" class="list-check" data-action="toggle" data-type="clipboard" data-id="' + esc(c.id) + '" ' + (checked ? 'checked' : '') + '>' +
        '<div class="list-icon">' + (c.type === 'image' ? iconImg() : iconText()) + '</div>' +
        '<div class="list-info">' +
          '<div class="list-name" data-action="open" data-url="' + esc(c.url) + '">' + (c.type === 'image' ? 'Image' : 'Text') + '</div>' +
          '<div class="list-meta">' + esc(fmtSize(c.size)) + '</div>' +
        '</div>' +
        '<div class="list-actions">' +
          '<button class="btn-icon" data-action="copy" data-url="' + fullUrl + '" title="Copy">' + iconCopy() + '</button>' +
          '<button class="btn-icon" data-action="qr" data-url="' + fullUrl + '" title="QR Code">' + iconQR() + '</button>' +
          '<button class="btn-icon delete" data-action="delete" data-type="clipboard" data-id="' + esc(c.id) + '" title="Delete">' + iconDel() + '</button>' +
        '</div></li>';
    }).join(''); }

    // Links
    var lFilter = searchTerms.links;
    var links = linksData;
    if (lFilter) {
      var ql = lFilter.toLowerCase();
      links = linksData.filter(function(u) { return u.short_url.toLowerCase().indexOf(ql) >= 0 || u.long_url.toLowerCase().indexOf(ql) >= 0; });
    }
    var ll = document.getElementById('links-list');
    var le = document.getElementById('links-empty');
    if (links.length === 0) { le.style.display = ''; ll.innerHTML = ''; }
    else { le.style.display = 'none'; ll.innerHTML = links.map(function(u) {
      var full = window.location.origin + u.short_url;
      var checked = selectedIds[u.code] ? 'checked' : '';
      return '<li class="' + checked + '">' +
        '<input type="checkbox" class="list-check" data-action="toggle" data-type="url" data-id="' + esc(u.code) + '" ' + (checked ? 'checked' : '') + '>' +
        '<div class="list-icon">' + iconLink() + '</div>' +
        '<div class="list-info">' +
          '<div class="list-name" data-action="open" data-url="' + esc(full) + '" title="' + esc(u.long_url) + '">' + esc(u.short_url) + '</div>' +
          '<div class="list-meta">' + esc(u.long_url.length > 50 ? u.long_url.substring(0,50) + '…' : u.long_url) + '</div>' +
        '</div>' +
        '<div class="list-actions">' +
          '<button class="btn-icon" data-action="copy" data-url="' + esc(full) + '" title="Copy">' + iconCopy() + '</button>' +
          '<button class="btn-icon" data-action="qr" data-url="' + esc(full) + '" title="QR Code">' + iconQR() + '</button>' +
          '<button class="btn-icon delete" data-action="delete" data-type="url" data-id="' + esc(u.code) + '" title="Delete">' + iconDel() + '</button>' +
        '</div></li>';
    }).join(''); }
  }

  // --- Event Delegation (replaces inline onclick, CSP-compatible) ---
  function handleListAction(e) {
    var target = e.target.closest('[data-action]');
    if (!target) return;
    var action = target.dataset.action;
    var id = target.dataset.id;
    var type = target.dataset.type;
    var url = target.dataset.url;
    if (action === 'toggle') { e.preventDefault(); toggleSelect(type, id); }
    else if (action === 'open') { window.open(url, '_blank'); }
    else if (action === 'copy') { copyLink(url); }
    else if (action === 'qr') { showQR(url); }
    else if (action === 'delete') { deleteItem(type, id); }
  }
  ['files-list', 'clipboard-list', 'links-list'].forEach(function(listId) {
    document.getElementById(listId).addEventListener('click', handleListAction);
  });

  // Bulk toolbar delegation
  (function() {
    var toolbar = document.getElementById('bulk-toolbar');
    if (toolbar) {
      toolbar.addEventListener('click', function(e) {
        var btn = e.target.closest('button');
        if (!btn) return;
        if (btn.textContent.indexOf('Select All') >= 0) selectAllVisible();
        else if (btn.textContent.indexOf('Delete') >= 0) bulkDelete();
        else if (btn.id === 'bulk-zip-btn') bulkDownload();
      });
    }
  })();

  // Paste preview delegation
  (function() {
    var pp = document.getElementById('paste-preview');
    if (pp) {
      pp.addEventListener('click', function(e) {
        var btn = e.target.closest('[data-action]');
        if (btn && btn.dataset.action === 'copy-paste') copyLink(btn.dataset.url);
      });
    }
  })();

  window.__search = function(tab, term) {
    searchTerms[tab] = term;
    renderHistory();
  };

  // Search input delegation
  document.getElementById('search-files').addEventListener('input', function() { window.__search('files', this.value); });
  document.getElementById('search-clipboard').addEventListener('input', function() { window.__search('clipboard', this.value); });
  document.getElementById('search-links').addEventListener('input', function() { window.__search('links', this.value); });

  // Service worker registration
  if ('serviceWorker' in navigator) navigator.serviceWorker.register('/sw.js').catch(function() {});

  function loadHistory() {
    if (!initialLoadDone) {
      showSkeleton('files-skeleton');
      showSkeleton('clipboard-skeleton');
      showSkeleton('links-skeleton');
    }
    Promise.all([
      fetch('/api/files' + authHeader()).then(r => r.ok ? r.json() : []),
      fetch('/api/clipboard' + authHeader()).then(r => r.ok ? r.json() : []),
      fetch('/api/urls' + authHeader()).then(r => r.ok ? r.json() : []),
    ]).then(function(res) {
      filesData = res[0];
      clipboardData = res[1];
      linksData = res[2];
      initialLoadDone = true;
      hideSkeleton('files-skeleton');
      hideSkeleton('clipboard-skeleton');
      hideSkeleton('links-skeleton');
      renderHistory();
    }).catch(function() {
      hideSkeleton('files-skeleton');
      hideSkeleton('clipboard-skeleton');
      hideSkeleton('links-skeleton');
    });
  }

  // --- Delete ---

  window.deleteItem = function(type, id) {
    var name = '';
    if (type === 'file') {
      var f = filesData.find(function(x){ return x.id === id; });
      name = f ? f.name : id;
    } else if (type === 'clipboard') {
      var c = clipboardData.find(function(x){ return x.id === id; });
      name = c ? (c.type === 'image' ? 'Image' : 'Text') + ' (' + id.slice(0,8) + ')' : id;
    } else {
      var u = linksData.find(function(x){ return x.code === id; });
      name = u ? u.long_url : id;
    }
    if (!confirm('Delete "' + name + '"?')) return;

    var url;
    if (type === 'file') {
      url = '/api/files/' + id;
    } else if (type === 'clipboard') {
      url = '/api/clipboard/' + id;
    } else {
      url = '/api/urls/' + id;
    }
    fetch(url + authHeader(), { method: 'DELETE' }).then(function(r) {
      if (r.ok || r.status === 204) {
        toast('Deleted');
        loadHistory();
      } else {
        toast('Delete failed', 'error');
      }
    }).catch(function() { toast('Delete failed', 'error'); });
  };

  // --- File Upload with Progress ---

  var dropZone = document.getElementById('drop-zone');
  var fileInput = document.getElementById('file-input');

  dropZone.addEventListener('dragover', function(e) { e.preventDefault(); dropZone.classList.add('dragover'); });
  dropZone.addEventListener('dragleave', function(e) { e.preventDefault(); dropZone.classList.remove('dragover'); });
  dropZone.addEventListener('drop', function(e) { e.preventDefault(); dropZone.classList.remove('dragover'); if (e.dataTransfer.files.length) uploadFiles(e.dataTransfer.files); });
  fileInput.addEventListener('change', function() { if (fileInput.files.length) uploadFiles(fileInput.files); fileInput.value = ''; });

  function uploadFiles(files) {
    if (!files.length) return;
    var total = files.length, uploaded = 0, active = 0, idx = 0;
    showProgress('Uploading 0 of ' + total + '...', 0);

    function next() {
      while (active < 3 && idx < total) {
        var ci = idx++, ca = ci;
        active++;
        uploadSingle(files[ca], function() {
          active--; uploaded++;
          showProgress('Uploaded ' + uploaded + ' of ' + total + '...', (uploaded/total)*100);
          if (uploaded >= total) { setTimeout(hideProgress, 800); }
          else { next(); }
        });
      }
    }
    next();
  }

  function uploadSingle(file, cb) {
    var fd = new FormData(); fd.append('file', file);
    var expires = document.getElementById('expires-select');
    if (expires && expires.value) fd.append('expires_at', expires.value);
    var xhr = new XMLHttpRequest();
    xhr.open('POST', '/api/upload' + authHeader(), true);
    xhr.upload.onprogress = function(e) { if (e.lengthComputable) setProgress(e.loaded/e.total*100); };
    xhr.onload = function() {
      if (xhr.status === 201) {
        try {
          var d = JSON.parse(xhr.responseText);
          filesData.unshift({ id: d.id, name: d.name, size: d.size, mime: d.mime, url: d.download_url });
          renderHistory();
          toast(d.name + ' uploaded');
        } catch(e) { toast('Upload complete'); }
      } else { toast('Failed: ' + file.name, 'error'); }
      cb();
    };
    xhr.onerror = function() { toast('Failed: ' + file.name, 'error'); cb(); };
    xhr.send(fd);
  }

  // --- Paste ---

  var pastePreview = document.getElementById('paste-preview');
  var lastPaste = 0;

  function handlePaste(data) {
    var now = Date.now();
    if (now - lastPaste < 500) return;
    lastPaste = now;
    if (!data) return;

    // Try image first from clipboard items
    var items = data.items || (data.files && data.files.length > 0 ? data : null);
    if (items && items.length) {
      for (var i = 0; i < items.length; i++) {
        var file = items[i].getAsFile ? items[i].getAsFile() : null;
        if (file && file.type && file.type.startsWith('image/')) {
          var fd = new FormData(); fd.append('file', file, 'pasted.png');
          var xhr = new XMLHttpRequest(); xhr.open('POST', '/api/clipboard' + authHeader(), true);
          xhr.onload = function() {
            if (xhr.status === 201) {
              try {
                var d = JSON.parse(xhr.responseText);
                var fu = window.location.origin + d.url;
                pastePreview.innerHTML = '<img src="' + esc(fu) + '" style="max-width:100%;max-height:300px;object-fit:contain;border-radius:10px;"><div class="preview-actions"><button class="btn-sm" data-action="copy-paste" data-url="' + esc(fu) + '">' + iconCopy() + ' Copy Link</button></div>';
                pastePreview.classList.remove('hidden');
                toast('Image saved');
                clipboardData.unshift({ id: d.id, type: 'image', url: d.url });
                renderHistory();
              } catch(e) {}
            }
          };
          xhr.send(fd);
          return;
        }
      }
    }

    // Text
    var text = '';
    try {
      if (typeof data.getData === 'function') {
        text = data.getData('text') || '';
      }
    } catch(e) {}
    if (!text && data.text) text = data.text;
    if (text && text.trim()) {
      text = text.trim();
      fetch('/api/clipboard' + authHeader(), { method: 'POST', headers: {'Content-Type':'application/json'}, body: JSON.stringify({type:'text',content:text}) })
      .then(function(r) { return r.json(); })
      .then(function(d) {
        var fu = window.location.origin + d.url;
        pastePreview.innerHTML = '<div style="white-space:pre-wrap;max-height:200px;overflow:auto;margin-bottom:8px">' + esc(text.substring(0,500)) + (text.length > 500 ? '…' : '') + '</div><div class="preview-actions"><button class="btn-sm" data-action="copy-paste" data-url="' + esc(fu) + '">' + iconCopy() + ' Copy Link</button></div>';
        pastePreview.classList.remove('hidden');
        toast('Text saved');
        clipboardData.unshift({ id: d.id, type: 'text', url: d.url });
        renderHistory();
      }).catch(function() { toast('Paste failed', 'error'); });
    }
  }

  // Desktop paste (not in input/textarea)
  document.addEventListener('paste', function(e) {
    var tag = e.target.tagName.toLowerCase();
    if (tag === 'input' || tag === 'textarea') return;
    var cd = e.clipboardData; if (!cd) return;
    var hasImg = false;
    var items = cd.items;
    if (items) {
      var count = items.length;
      for (var i = 0; i < count; i++) {
        var it = items[i] || items.item(i);
        if (it.type.indexOf('image/') === 0) { hasImg = true; break; }
      }
    }
    var hasTxt = (cd.getData('text') || '').trim().length > 0;
    if (!hasImg && !hasTxt) return;
    e.preventDefault();
    handlePaste(cd);
  });

  // Paste button
  document.getElementById('paste-btn').addEventListener('click', function() {
    var isMobile = /iPhone|iPad|iPod|Android/i.test(navigator.userAgent);
    if (isMobile || !(navigator.clipboard && navigator.clipboard.read)) {
      // Mobile or no Clipboard API — show paste target
      showPasteInput();
      return;
    }
    // Desktop with Clipboard API support
    navigator.clipboard.read().then(function(items) {
      for (var i = 0; i < items.length; i++) {
        var it = items[i];
        var imgType = it.types.find(function(t) { return t.startsWith('image/'); });
        if (imgType) {
          it.getType(imgType).then(function(blob) {
            handlePaste({ items: [{ getAsFile: function() { return blob; }, type: imgType }], files: [blob] });
          });
          return;
        }
      }
      if (items.length > 0 && items[0].types.includes('text/plain')) {
        items[0].getType('text/plain').then(function(b) { return b.text(); }).then(function(t) {
          if (t) handlePaste({ text: t }); else toast('Clipboard is empty', 'error');
        });
      } else { toast('Clipboard is empty', 'error'); }
    }).catch(function() {
      showPasteInput();
    });
  });

  // Show a contenteditable area for manual paste (Firefox, Safari fallback)
  function showPasteInput() {
    var container = document.getElementById('paste-input-container');
    if (!container) {
      container = document.createElement('div');
      container.id = 'paste-input-container';
      container.style.cssText = 'margin-top:12px;';

      var hint = document.createElement('p');
      hint.style.cssText = 'font-size:12px;color:var(--text-secondary);margin-bottom:8px;';
      hint.textContent = 'Tap below, then long-press and select "Paste" to paste from your clipboard.';
      container.appendChild(hint);

      var el = document.createElement('div');
      el.id = 'paste-input';
      el.contentEditable = true;
      el.className = 'preview-area';
      el.style.cssText = 'min-height:80px;outline:none;cursor:text;';

      el.addEventListener('paste', function(e) {
        var cd = e.clipboardData;
        if (!cd) return;
        e.preventDefault();
        container.classList.add('hidden');
        el.textContent = '';
        handlePaste(cd);
      });

      el.addEventListener('blur', function() {
        if (el.textContent.trim() === '') container.classList.add('hidden');
      });

      container.appendChild(el);
      var preview = document.getElementById('paste-preview');
      preview.parentNode.insertBefore(container, preview.nextSibling);
    }
    container.classList.remove('hidden');
    container.querySelector('#paste-input').textContent = '';
    container.querySelector('#paste-input').focus();
  }

  // --- URL Shortener ---

  document.getElementById('url-form').addEventListener('submit', function(e) {
    e.preventDefault();
    var input = document.getElementById('url-input');
    var result = document.getElementById('url-result');
    fetch('/api/url' + authHeader(), { method: 'POST', headers: {'Content-Type':'application/json'}, body: JSON.stringify({url:input.value}) })
    .then(function(r) { if (!r.ok) throw new Error('Failed'); return r.json(); })
    .then(function(d) {
      var fu = window.location.origin + d.short_url;
      result.querySelector('#url-result-link').href = fu;
      result.querySelector('#url-result-link').textContent = fu;
      result.classList.remove('hidden');
      input.value = '';
      toast('Link shortened');
      linksData.unshift({ code: d.code, short_url: d.short_url, long_url: input.value });
      renderHistory();
    }).catch(function() { toast('Failed to shorten', 'error'); });
  });

  document.getElementById('url-copy-btn').addEventListener('click', function() {
    copyText(document.getElementById('url-result-link').href).then(function() { toast('Link copied'); });
  });

  document.getElementById('url-qr-btn').addEventListener('click', function() {
    showQR(document.getElementById('url-result-link').href);
  });

  // --- Global helpers ---

  window.copyLink = function(url) { copyText(url).then(function() { toast('Link copied'); }); };
  window.openItem = function(url) { window.open(url, '_blank'); };
  window.showQR = showQR;
  window.hideQR = hideQR;
  window.toggleSelect = toggleSelect;
  window.selectAllVisible = selectAllVisible;
  window.bulkDelete = bulkDelete;
  window.bulkDownload = bulkDownload;

  function esc(s) { if (!s) return ''; return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;').replace(/'/g,'&#39;'); }
  function fmtSize(b) { if (b == null || b === 0) return ''; var u = ['B','KB','MB','GB']; var i = 0; while (b >= 1024 && i < u.length-1) { b/=1024; i++; } return (i===0 ? Math.round(b) : b.toFixed(1)) + ' ' + u[i]; }

  function iconCopy() { return '<svg width="15" height="15" viewBox="0 0 15 15" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><rect x="5" y="5" width="8" height="8" rx="1.5"/><path d="M10 5V3.5A1.5 1.5 0 008.5 2h-5A1.5 1.5 0 002 3.5v5A1.5 1.5 0 003.5 9H5"/></svg>'; }
  function iconDel() { return '<svg width="15" height="15" viewBox="0 0 15 15" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M3 4h9M5 4V3a1 1 0 011-1h3a1 1 0 011 1v1M6 7v4M9 7v4M4 4l1 8h5l1-8"/></svg>'; }
  function iconImg() { return '<svg viewBox="0 0 18 18" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="2" width="14" height="14" rx="2"/><circle cx="6" cy="6" r="1"/><polyline points="16 12 12 8 8 14"/></svg>'; }
  function iconText() { return '<svg viewBox="0 0 18 18" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M4 4h10M4 9h10M4 14h6"/></svg>'; }
  function iconLink() { return '<svg viewBox="0 0 18 18" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M7 11l-2 2a3.5 3.5 0 01-5-0.5l-1-1"/><path d="M11 7l2-2a3.5 3.5 0 005-0.5l1-1"/><path d="M11 4L9 6M7 14l-2 2"/></svg>'; }
  function getFileIcon(mime) {
    if (mime && mime.startsWith('image/')) return iconImg();
    if (mime && mime.startsWith('video/')) return '<svg viewBox="0 0 18 18" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="3" width="14" height="12" rx="2"/><polygon points="7.5 6.5 7.5 11.5 12 9"/></svg>';
    return '<svg viewBox="0 0 18 18" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M10 2H5a1 1 0 00-1 1v12a1 1 0 001 1h8a1 1 0 001-1V7l-3-5z"/><polyline points="10 2 10 7 15 7"/></svg>';
  }

  // --- Storage Info ---

  function loadStorage() {
    fetch('/api/space' + authHeader()).then(function(r) {
      if (!r.ok) throw new Error();
      return r.json();
    }).then(function(data) {
      var bar = document.getElementById('storage-bar');
      bar.classList.remove('hidden');
      var pctUsed = data.total > 0 ? data.used / data.total * 100 : 0;
      var pctText;
      if (pctUsed >= 99.995) {
        pctText = '100% used';
      } else if (pctUsed >= 10) {
        pctText = Math.round(pctUsed) + '% used';
      } else if (pctUsed >= 1) {
        pctText = pctUsed.toFixed(1) + '% used';
      } else if (pctUsed >= 0.01) {
        pctText = pctUsed.toFixed(2) + '% used';
      } else {
        pctText = '0.00% used';
      }
      document.getElementById('storage-text').textContent =
        fmtSize(data.used) + ' used of ' + fmtSize(data.total) + ' (' + pctText + ')';
      var fill = document.getElementById('storage-fill');
      fill.style.width = Math.min(100, pctUsed) + '%';
      fill.classList.remove('warn', 'critical');
      if (pctUsed >= 90) fill.classList.add('critical');
      else if (pctUsed >= 75) fill.classList.add('warn');
    }).catch(function() {});
  }

  // --- SSE Live Updates ---

  (function startSSE() {
    if (!window.EventSource) return;
    var es = new EventSource('/api/events' + authHeader());
    es.onmessage = function() { loadHistory(); };
    es.onerror = function() { es.close(); setTimeout(startSSE, 5000); };
  })();

  // --- Init ---
  loadHistory();
  loadStorage();
})();
