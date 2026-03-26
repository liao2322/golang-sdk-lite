const state = {
  currentView: "files",
  currentPath: "/",
  session: null,
  deviceCode: null,
  linkMode: window.localStorage.getItem("halal_link_mode") || "redirect",
};

// DOM Elements
const views = {
  auth: document.getElementById("auth-view"),
  main: document.getElementById("main-view")
};

const elements = {
  content: document.getElementById("content-view"),
  toastContainer: document.getElementById("toast-container"),
  breadcrumbs: document.getElementById("breadcrumbs"),
  viewTitle: document.getElementById("view-title"),
  player: {
    modal: document.getElementById("player-modal"),
    body: document.getElementById("player-body"),
    title: document.getElementById("player-title"),
    closeBtn: document.getElementById("close-player")
  },
  devicePanel: document.getElementById("device-panel"),
  linkModeSelect: document.getElementById("link-mode-select")
};

// Initialization
function init() {
  bindEvents();
  registerServiceWorker();
  boot();
}

function bindEvents() {
  document.getElementById("start-login").addEventListener("click", startDeviceLogin);
  document.getElementById("logout-btn").addEventListener("click", logout);
  document.getElementById("refresh-btn").addEventListener("click", () => loadView(state.currentView));
  document.getElementById("new-folder-btn").addEventListener("click", createFolder);

  elements.player.closeBtn.addEventListener("click", closePlayer);

  elements.linkModeSelect.value = state.linkMode;
  elements.linkModeSelect.addEventListener("change", () => {
    state.linkMode = elements.linkModeSelect.value;
    window.localStorage.setItem("halal_link_mode", state.linkMode);
    toast(`Switched to ${state.linkMode === "proxy" ? "Proxy" : "Redirect"} mode`, "success", "ph-plugs-connected");
    loadView(state.currentView);
  });

  document.querySelectorAll(".nav-item[data-view]").forEach((button) => {
    button.addEventListener("click", () => {
      document.querySelectorAll(".nav-item[data-view]").forEach((item) => item.classList.remove("active"));
      button.classList.add("active");
      loadView(button.dataset.view);
    });
  });
}

async function boot() {
  try {
    const session = await api("/api/auth/session");
    if (session.default_mode && !window.localStorage.getItem("halal_link_mode")) {
      state.linkMode = session.default_mode;
      elements.linkModeSelect.value = state.linkMode;
    }
    if (session.authenticated) {
      state.session = session;
      showView('main');
      await loadView("files");
      return;
    }
  } catch (e) {
    console.error("Session check failed", e);
  }
  showView('auth');
}

function showView(viewName) {
  if (viewName === 'auth') {
    views.main.classList.add("hidden");
    views.auth.classList.remove("hidden");
  } else {
    views.auth.classList.add("hidden");
    views.main.classList.remove("hidden");
  }
}

// Auth Logic
async function startDeviceLogin() {
  const btn = document.getElementById("start-login");
  btn.disabled = true;
  btn.innerHTML = `<i class="ph ph-spinner spinner"></i> Starting...`;

  try {
    const data = await api("/api/auth/device-code/start", { method: "POST" });
    state.deviceCode = data.device_code;

    elements.devicePanel.classList.remove("hidden");
    document.getElementById("user-code").textContent = data.user_code;

    const link = document.getElementById("verification-link");
    link.href = data.verification_uri;
    // Don't change link text, keep it as "Open Verification"

    document.getElementById("device-status").textContent = "Waiting for authorization...";
    pollDeviceCode(data.interval || 5);
  } catch (e) {
    btn.disabled = false;
    btn.innerHTML = `<i class="ph ph-device-mobile"></i> Start Device Login`;
  }
}

async function pollDeviceCode(interval) {
  if (!state.deviceCode) return;
  try {
    const result = await api(`/api/auth/device-code/status?device_code=${encodeURIComponent(state.deviceCode)}`);
    document.getElementById("device-status").textContent = result.status || "Waiting...";

    if (result.status === "AUTHORIZATION_SUCCESS") {
      state.deviceCode = null;
      state.session = await api("/api/auth/session");
      showView('main');
      toast("Successfully logged in", "success", "ph-check-circle");
      await loadView("files");
      return;
    }
  } catch (e) {
    console.error("Polling error", e);
  }
  window.setTimeout(() => pollDeviceCode(interval), interval * 1000);
}

async function logout() {
  try {
    await api("/api/auth/logout", { method: "POST" });
    state.session = null;
    state.deviceCode = null;
    showView('auth');
    toast("Logged out", "info", "ph-info");

    // Reset login button
    const btn = document.getElementById("start-login");
    btn.disabled = false;
    btn.innerHTML = `<i class="ph ph-device-mobile"></i> Start Device Login`;
    elements.devicePanel.classList.add("hidden");
  } catch (e) {
    console.error("Logout error", e);
  }
}

// View Routing & Rendering
async function loadView(view) {
  state.currentView = view;
  elements.content.innerHTML = `<div class="empty-state"><div class="spinner"></div><p>Loading...</p></div>`;

  try {
    switch (view) {
      case "files":
        elements.viewTitle.textContent = "Files";
        elements.breadcrumbs.textContent = state.currentPath;
        await renderFiles();
        break;
      case "recent":
        elements.viewTitle.textContent = "Recent";
        elements.breadcrumbs.textContent = "Last 7 days";
        await renderRecent();
        break;
      case "trash":
        elements.viewTitle.textContent = "Trash";
        elements.breadcrumbs.textContent = "Deleted items";
        await renderTrash();
        break;
      case "offline":
        elements.viewTitle.textContent = "Offline Tasks";
        elements.breadcrumbs.textContent = "Active downloads";
        await renderOffline();
        break;
      case "account":
        elements.viewTitle.textContent = "Account";
        elements.breadcrumbs.textContent = "Session info";
        await renderAccount();
        break;
    }
  } catch (e) {
    elements.content.innerHTML = emptyState(`Failed to load view: ${e.message}`, "ph-warning-circle");
  }
}

async function renderFiles() {
  const data = await api(`/api/files?path=${encodeURIComponent(state.currentPath)}`);
  elements.content.innerHTML = data.items?.length
    ? `<div class="file-list">${data.items.map((item) => fileCard(item, false)).join("")}</div>`
    : emptyState("Folder is empty.", "ph-folder-dashed");
  wireFileActions(data.items || [], false);
}

async function renderRecent() {
  const startTs = Math.floor(Date.now() / 1000) - 7 * 24 * 60 * 60;
  const data = await api(`/api/files/recent?path=%2F&start_ts=${startTs}`);
  elements.content.innerHTML = data.items?.length
    ? `<div class="file-list">${data.items.map((item) => fileCard(item, false)).join("")}</div>`
    : emptyState("No recent files found.", "ph-clock-dashed");
  wireFileActions(data.items || [], false);
}

async function renderTrash() {
  const data = await api("/api/files/trash");
  elements.content.innerHTML = data.items?.length
    ? `<div class="file-list">${data.items.map((item) => fileCard(item, true)).join("")}</div>`
    : emptyState("Trash is empty.", "ph-trash");
  wireFileActions(data.items || [], true);
}

async function renderOffline() {
  const data = await api("/api/offline/tasks");
  const form = `
    <div class="offline-card">
      <div style="display:flex; align-items:center; gap:0.5rem">
        <i class="ph-fill ph-cloud-arrow-down" style="color:var(--primary); font-size:1.5rem"></i>
        <h3>New Offline Task</h3>
      </div>
      <div class="task-input-wrapper">
        <input id="offline-url" class="glass-input" placeholder="Enter magnet link or URL..." />
        <button id="offline-create" class="btn btn-primary"><i class="ph ph-plus"></i></button>
      </div>
    </div>
  `;
  const list = data.tasks?.length
    ? data.tasks.map((task) => `
        <div class="offline-card">
          <h3 title="${escapeAttr(task.name || task.url || task.identity)}">${escapeHTML(task.name || task.url || task.identity)}</h3>
          <div style="display:flex; justify-content:space-between; align-items:center">
            <p class="muted"><i class="ph ph-info"></i> ${task.status} · ${task.progress || 0}%</p>
            <button class="btn btn-icon text-danger" data-offline-delete="${task.identity}" title="Delete Task"><i class="ph ph-trash"></i></button>
          </div>
        </div>
      `).join("")
    : emptyState("No active offline tasks.", "ph-cloud-slash");

  elements.content.innerHTML = `<div class="card-grid">${form}${list !== '<div class="empty-state"><i class="ph ph-cloud-slash"></i><p>No active offline tasks.</p></div>' ? list : ''}</div>${list === '<div class="empty-state"><i class="ph ph-cloud-slash"></i><p>No active offline tasks.</p></div>' ? list : ''}`;

  document.getElementById("offline-create").addEventListener("click", createOfflineTask);
  document.querySelectorAll("[data-offline-delete]").forEach((button) => {
    button.addEventListener("click", async () => {
      try {
        await api(`/api/offline/tasks/${button.dataset.offlineDelete}`, { method: "DELETE" });
        toast("Task deleted", "info");
        await renderOffline();
      } catch (e) { }
    });
  });
}

async function renderAccount() {
  const [me, quota] = await Promise.all([api("/api/user/me"), api("/api/user/quota")]);
  elements.content.innerHTML = `
    <div class="card-grid">
      <div class="stat-card" style="grid-column: 1 / -1; display:flex; flex-direction:row; align-items:center; gap:2rem;">
        <div style="background:var(--primary-glow); padding:1rem; border-radius:var(--radius-full)">
           <i class="ph-fill ph-user" style="font-size:3rem; color:white"></i>
        </div>
        <div>
          <h3>${escapeHTML(me.name || "Unnamed User")}</h3>
          <p class="muted">ID: ${escapeHTML(me.identity || "-")}</p>
        </div>
      </div>
      
      <div class="stat-card">
        <div style="display:flex; align-items:center; gap:0.5rem">
          <i class="ph-fill ph-hard-drive" style="color:var(--primary); font-size:1.5rem"></i>
          <h3>Storage</h3>
        </div>
        <div class="progress-bar-container" style="background:rgba(0,0,0,0.3); height:8px; border-radius:4px; margin:0.5rem 0; overflow:hidden">
           <div style="background:var(--primary); width: ${Math.min(100, (quota.disk_statistics_quota?.bytes_used / quota.disk_statistics_quota?.bytes_quota) * 100 || 0)}%; height:100%"></div>
        </div>
        <p class="muted">${formatBytes(quota.disk_statistics_quota?.bytes_used)} / ${formatBytes(quota.disk_statistics_quota?.bytes_quota)}</p>
      </div>
      
      <div class="stat-card">
        <div style="display:flex; align-items:center; gap:0.5rem">
          <i class="ph-fill ph-wifi-high" style="color:#38bdf8; font-size:1.5rem"></i>
          <h3>Traffic limit</h3>
        </div>
        <p class="muted">Today: ${formatBytes(quota.traffic_statistics_quota?.bytes_downloaded_today)}</p>
      </div>
      
      <div class="stat-card">
        <div style="display:flex; align-items:center; gap:0.5rem">
          <i class="ph-fill ph-cloud-arrow-down" style="color:#fbbf24; font-size:1.5rem"></i>
          <h3>Offline allowance</h3>
        </div>
        <p class="muted">Submitted today: ${quota.offline_task_statistics_quota?.tasks_commited_today || 0}</p>
      </div>
    </div>
  `;
}

// Components
function fileCard(file, inTrash) {
  const isDir = file.dir;
  const iconClass = isDir ? "ph-fill ph-folder file-icon folder" : "ph-fill ph-file file-icon";

  return `
    <article class="file-card">
      <div class="file-row">
        <div class="file-meta">
          <i class="${iconClass}"></i>
          <div class="file-name-container">
            <button class="link-btn" data-open="${escapeAttr(file.identity)}" data-path="${escapeAttr(file.path || "")}" data-dir="${isDir}">
              ${escapeHTML(file.name || file.path || file.identity)}
            </button>
            <span class="muted" title="${escapeAttr(file.path || "")}">${escapeHTML(file.path || "-")}</span>
          </div>
        </div>
        <div class="muted">${isDir ? "Folder" : "File"}</div>
        <div class="muted">${formatBytes(file.size)}</div>
        <div class="muted">${formatTs(file.update_ts)}</div>
        <div class="row-actions">
          ${isDir ? "" : `<button class="btn btn-icon" data-play="${escapeAttr(file.identity)}" title="Play/Preview"><i class="ph ph-play-circle"></i></button>`}
          ${isDir ? "" : `<a class="btn btn-icon" href="${buildMediaURL("/api/files/download/", file.identity)}" target="_blank" rel="noreferrer" title="Download"><i class="ph ph-download-simple"></i></a>`}
          ${inTrash
      ? `<button class="btn btn-icon" data-recover="${escapeAttr(file.identity)}" title="Recover"><i class="ph ph-arrow-u-up-left"></i></button>
               <button class="btn btn-icon text-danger" data-delete="${escapeAttr(file.identity)}" title="Permanent Delete"><i class="ph ph-trash"></i></button>`
      : `<button class="btn btn-icon" data-rename="${escapeAttr(file.identity)}" data-name="${escapeAttr(file.name || "")}" title="Rename"><i class="ph ph-pencil-simple"></i></button>
               <button class="btn btn-icon text-danger" data-trash="${escapeAttr(file.identity)}" title="Move to Trash"><i class="ph ph-trash"></i></button>`}
        </div>
      </div>
    </article>
  `;
}

function wireFileActions(items, inTrash) {
  const query = (selector, event, handler) => {
    document.querySelectorAll(selector).forEach(el => el.addEventListener(event, handler));
  };

  query("[data-open]", "click", async (e) => {
    const btn = e.currentTarget;
    if (btn.dataset.dir === "true") {
      state.currentPath = btn.dataset.path || "/";
      await loadView("files");
    } else {
      const file = items.find(i => i.identity === btn.dataset.open);
      if (file) openPlayer(file);
    }
  });

  query("[data-play]", "click", (e) => {
    const file = items.find(i => i.identity === e.currentTarget.dataset.play);
    if (file) openPlayer(file);
  });

  query("[data-rename]", "click", async (e) => {
    const btn = e.currentTarget;
    const nextName = window.prompt("New name:", btn.dataset.name || "");
    if (!nextName) return;
    try {
      await api("/api/files/rename", {
        method: "POST",
        body: JSON.stringify({ identity: btn.dataset.rename, name: nextName }),
      });
      toast("Renamed successfully", "success");
      await loadView(state.currentView);
    } catch (err) { }
  });

  query("[data-trash]", "click", async (e) => {
    try {
      const btn = e.currentTarget;
      await api("/api/files/trash-action", {
        method: "POST",
        body: JSON.stringify({ ids: [btn.dataset.trash] }),
      });
      toast("Moved to trash", "info");
      await loadView(state.currentView);
    } catch (err) { }
  });

  query("[data-recover]", "click", async (e) => {
    try {
      await api("/api/files/recover", {
        method: "POST",
        body: JSON.stringify({ ids: [e.currentTarget.dataset.recover] }),
      });
      toast("File recovered", "success");
      await renderTrash();
    } catch (err) { }
  });

  query("[data-delete]", "click", async (e) => {
    try {
      if (!confirm("Are you sure you want to permanently delete this?")) return;
      await api("/api/files/delete", {
        method: "POST",
        body: JSON.stringify({ ids: [e.currentTarget.dataset.delete] }),
      });
      toast("Permanently deleted", "info");
      await renderTrash();
    } catch (err) { }
  });
}

function openPlayer(file) {
  const mime = (file.mime_type || "").toLowerCase();
  const name = file.name || file.path || "Preview";
  const src = buildMediaURL("/api/files/play/", file.identity);

  let inner = `<div class="empty-state"><i class="ph ph-file-video"></i><p>This file type cannot be previewed. Please download it.</p></div>`;

  if (mime.startsWith("video/") || /\.(mp4|webm|m3u8)$/i.test(name)) {
    inner = `<video controls playsinline src="${src}" crossorigin="anonymous" referrerpolicy="no-referrer"></video>`;
  } else if (mime.startsWith("audio/") || /\.(mp3|wav|ogg|m4a)$/i.test(name)) {
    inner = `<audio controls src="${src}" crossorigin="anonymous" referrerpolicy="no-referrer"></audio>`;
  } else if (mime.startsWith("image/") || /\.(png|jpg|jpeg|gif|webp)$/i.test(name)) {
    inner = `<img alt="${escapeAttr(name)}" src="${src}" referrerpolicy="no-referrer" />`;
  }

  elements.player.title.textContent = name;
  elements.player.body.innerHTML = inner;
  elements.player.modal.showModal();
}

function closePlayer() {
  // Clear HTML to stop playback
  elements.player.body.innerHTML = '';
  elements.player.modal.close();
}

async function createFolder() {
  if (state.currentView !== "files") {
    toast("Can only create folders in Files view", "warning", "ph-warning");
    return;
  }
  const name = window.prompt("Folder name:");
  if (!name) return;
  try {
    await api("/api/files/create", {
      method: "POST",
      body: JSON.stringify({ parent: state.currentPath, name, dir: true }),
    });
    toast("Folder created", "success");
    await renderFiles();
  } catch (e) { }
}

async function createOfflineTask() {
  const input = document.getElementById("offline-url");
  const url = input.value.trim();
  if (!url) {
    toast("Please enter a valid URL", "warning");
    return;
  }
  try {
    const btn = document.getElementById("offline-create");
    btn.innerHTML = `<i class="ph ph-spinner spinner"></i>`;
    btn.disabled = true;

    await api("/api/offline/tasks", {
      method: "POST",
      body: JSON.stringify({ url, save_path: "/" }),
    });

    input.value = "";
    toast("Task created successfully", "success");
    await renderOffline();
  } catch (e) {
    const btn = document.getElementById("offline-create");
    btn.innerHTML = `<i class="ph ph-plus"></i>`;
    btn.disabled = false;
  }
}

// Utils & Helpers
async function api(url, options = {}) {
  const response = await fetch(url, {
    credentials: "same-origin",
    headers: {
      "Content-Type": "application/json",
      ...(options.headers || {}),
    },
    ...options,
  });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    const message = data.error || "Request failed";
    toast(message, "error", "ph-warning-octagon");
    throw new Error(message);
  }
  return data;
}

function toast(message, type = "info", icon = "ph-info") {
  const el = document.createElement("div");
  el.className = `toast toast-${type}`;
  el.innerHTML = `<i class="ph ${icon}" style="font-size:1.25rem"></i> <span>${escapeHTML(message)}</span>`;
  elements.toastContainer.appendChild(el);

  setTimeout(() => {
    el.classList.add("hiding");
    el.addEventListener("animationend", () => el.remove());
  }, 3000);
}

function emptyState(text, icon = "ph-folder-dashed") {
  return `<div class="empty-state">
    <i class="ph ${icon}"></i>
    <p>${escapeHTML(text)}</p>
  </div>`;
}

function formatBytes(value) {
  const size = Number(value || 0);
  if (!size) return "-";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let current = size;
  let index = 0;
  while (current >= 1024 && index < units.length - 1) {
    current /= 1024;
    index += 1;
  }
  return `${current.toFixed(current >= 10 || index === 0 ? 0 : 1)} ${units[index]}`;
}

function formatTs(value) {
  const num = Number(value || 0);
  if (!num) return "-";
  return new Date(num * 1000).toLocaleString("zh-CN");
}

function escapeHTML(value) {
  return String(value ?? "").replace(/[&<>"']/g, (char) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
  }[char]));
}

function escapeAttr(value) {
  return escapeHTML(value);
}

function buildMediaURL(prefix, identity) {
  return `${prefix}${encodeURIComponent(identity)}?mode=${encodeURIComponent(state.linkMode)}`;
}

function registerServiceWorker() {
  if (!("serviceWorker" in navigator)) {
    return;
  }
  window.addEventListener("load", () => {
    navigator.serviceWorker.register("/sw.js").catch(() => {
      // Keep app usable even if service worker registration fails.
    });
  });
}

// Start app
init();
