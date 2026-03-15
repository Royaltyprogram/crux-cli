const ADMIN_STORAGE_KEYS = {
  sessionUser: "crux_session_user",
  sessionOrg: "crux_session_org",
};

const adminState = {
  busy: false,
  importMetrics: null,
  session: null,
  users: [],
};

const $ = (id) => document.getElementById(id);

function escapeHTML(value) {
  return String(value == null ? "" : value).replace(
    /[&<>"']/g,
    (char) =>
      ({
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        '"': "&quot;",
        "'": "&#39;",
      })[char],
  );
}

function readStorage(key, fallback = "") {
  try {
    const value = window.localStorage.getItem(key);
    return value == null ? fallback : value;
  } catch (error) {
    return fallback;
  }
}

function writeStorage(key, value) {
  try {
    if (value == null || value === "") {
      window.localStorage.removeItem(key);
      return;
    }
    window.localStorage.setItem(key, value);
  } catch (error) {
  }
}

function writeSession(session) {
  adminState.session = session;
  writeStorage(
    ADMIN_STORAGE_KEYS.sessionUser,
    JSON.stringify((session && session.user) || {}),
  );
  writeStorage(
    ADMIN_STORAGE_KEYS.sessionOrg,
    JSON.stringify((session && session.organization) || {}),
  );
}

function clearSession() {
  writeStorage(ADMIN_STORAGE_KEYS.sessionUser, "");
  writeStorage(ADMIN_STORAGE_KEYS.sessionOrg, "");
  adminState.session = null;
}

function setStatus(text, isError = false) {
  const status = $("status");
  status.textContent = text;
  status.dataset.tone = isError ? "error" : "info";
}

function setLandingNotice(message) {
  try {
    if (message) {
      window.sessionStorage.setItem("crux_redirect_notice", message);
    } else {
      window.sessionStorage.removeItem("crux_redirect_notice");
    }
  } catch (error) {
  }
}

function redirectToLanding(message) {
  setLandingNotice(message);
  clearSession();
  window.location.replace("/");
}

function redirectToDashboard(message) {
  setLandingNotice(message);
  window.location.replace("/dashboard");
}

async function requestJSON(url, options = {}, fallbackMessage = "Request failed.") {
  const response = await fetch(url, Object.assign({ credentials: "same-origin" }, options));
  const rawBody = await response.text();

  let envelope = null;
  if (rawBody) {
    try {
      envelope = JSON.parse(rawBody);
    } catch (error) {
      const unreadable = new Error(response.ok ? "The server returned an unreadable response." : fallbackMessage);
      unreadable.status = response.status;
      throw unreadable;
    }
  }

  if (!response.ok || !envelope || envelope.code !== 0) {
    const failure = new Error((envelope && (envelope.msg || envelope.message)) || fallbackMessage);
    failure.status = response.status;
    throw failure;
  }

  return envelope.data || {};
}

function isUnauthorized(error) {
  return !!(error && (error.status === 401 || error.status === 403));
}

function syncHeader() {
  const session = adminState.session || {};
  const user = session.user || {};
  const organization = session.organization || {};
  $("topBarUser").textContent = user.name || user.email || "-";
  $("topBarOrg").textContent = organization.name || organization.id || "-";
}

function formatDateTime(value) {
  if (!value) {
    return "Not yet";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return String(value);
  }
  return date.toLocaleString();
}

function formatCount(value) {
  return new Intl.NumberFormat("en-US").format(Number(value || 0));
}

function formatPercent(value) {
  return `${Math.round(Number(value || 0) * 100)}%`;
}

function formatLatency(value) {
  const ms = Math.round(Number(value || 0));
  if (!ms || ms < 0) {
    return "0s";
  }
  if (ms < 1000) {
    return `${ms}ms`;
  }
  if (ms < 60000) {
    const seconds = ms / 1000;
    return `${seconds >= 10 ? Math.round(seconds) : seconds.toFixed(1)}s`;
  }
  const minutes = Math.floor(ms / 60000);
  const seconds = Math.round((ms % 60000) / 1000);
  return seconds > 0 ? `${minutes}m ${seconds}s` : `${minutes}m`;
}

function pill(label, tone) {
  return `<span class="pill ${toneClass(tone)}">${escapeHTML(label)}</span>`;
}

function toneClass(tone) {
  switch (tone) {
    case "good":
      return "pill-good";
    case "warn":
      return "pill-warn";
    case "danger":
      return "pill-danger";
    default:
      return "pill-sky";
  }
}

function statusTone(status) {
  switch (String(status || "").toLowerCase()) {
    case "active":
      return "good";
    case "disabled":
      return "warn";
    case "deleted":
      return "danger";
    default:
      return "sky";
  }
}

function roleTone(role) {
  return String(role || "").toLowerCase() === "admin" ? "sky" : "good";
}

function sourceTone(source) {
  switch (String(source || "").toLowerCase()) {
    case "google":
      return "good";
    case "demo":
      return "warn";
    case "bootstrap":
      return "sky";
    case "managed":
      return "good";
    default:
      return "sky";
  }
}

function importJobTone(status) {
  switch (String(status || "").toLowerCase()) {
    case "succeeded":
      return "good";
    case "failed":
      return "danger";
    case "partially_failed":
    case "queued":
      return "warn";
    case "receiving_chunks":
    case "running":
      return "sky";
    default:
      return "sky";
  }
}

function emptyState(title, body) {
  return `
    <div class="item">
      <div class="item-title">${escapeHTML(title)}</div>
      <div class="admin-empty-note">${escapeHTML(body)}</div>
    </div>
  `;
}

function importFailureSummary(item) {
  const failures = Array.isArray(item && item.failures) ? item.failures : [];
  const latest = failures.length ? failures[failures.length - 1] : null;
  if (!latest) {
    return String((item && item.last_error) || "").trim();
  }
  const parts = [];
  if (latest.session_id) {
    parts.push(latest.session_id);
  }
  if (latest.error) {
    parts.push(latest.error);
  }
  if (Number(latest.http_status || 0) > 0) {
    parts.push(`HTTP ${Number(latest.http_status)}`);
  }
  if (Number(latest.api_error_code || 0) > 0) {
    parts.push(`API ${Number(latest.api_error_code)}`);
  }
  return parts.join(" - ");
}

function renderAdminImportMetrics(data) {
  const metrics = (data && data.metrics) || null;
  adminState.importMetrics = data || null;

  const created = Number((metrics && metrics.created_jobs) || 0);
  const active =
    Number((metrics && metrics.receiving_jobs) || 0) +
    Number((metrics && metrics.queued_jobs) || 0) +
    Number((metrics && metrics.running_jobs) || 0);
  const degraded =
    Number((metrics && metrics.failed_jobs) || 0) +
    Number((metrics && metrics.partially_failed_jobs) || 0);

  $("adminImportCreatedJobs").textContent = formatCount(created);
  $("adminImportActiveJobs").textContent = formatCount(active);
  $("adminImportFailureRate").textContent = formatPercent((metrics && metrics.failure_rate) || 0);
  $("adminImportAvgDuration").textContent = formatLatency((metrics && metrics.avg_duration_ms) || 0);

  $("importMetricsSummary").textContent = created > 0
    ? `Tracking ${formatCount(created)} async import job(s); ${formatCount(active)} active and ${formatCount(degraded)} degraded recently.`
    : "No async import jobs have been recorded yet for this organization.";

  const activeJobs = Array.isArray(data && data.active_jobs) ? data.active_jobs : [];
  $("adminActiveImportList").innerHTML = activeJobs.length
    ? activeJobs.map((item) => `
        <div class="item">
          <div class="item-top">
            <div class="item-title">
              ${escapeHTML(item.job_id || "Import job")}
              <small>${escapeHTML(`Project ${item.project_id || "-"}`)}</small>
            </div>
            ${pill(item.status || "running", importJobTone(item.status))}
          </div>
          <div class="step-list">
            <div class="step-line">${escapeHTML(`${formatCount(Math.max(Number(item.processed_sessions || 0), Number(item.received_sessions || 0)))} visible progress updates`)} </div>
            <div class="step-line">${escapeHTML(`Processed ${formatCount(item.processed_sessions || 0)} · Uploaded ${formatCount(Number(item.uploaded_sessions || 0) + Number(item.updated_sessions || 0))}`)}</div>
            <div class="step-line">${escapeHTML(`Last activity ${formatDateTime(item.completed_at || item.started_at || item.created_at)}`)}</div>
          </div>
        </div>
      `).join("")
    : emptyState(
        "No active import jobs",
        "Receiving, queued, or running jobs will appear here.",
      );

  const recentFailures = Array.isArray(data && data.recent_failures) ? data.recent_failures : [];
  $("adminFailureImportList").innerHTML = recentFailures.length
    ? recentFailures.map((item) => `
        <div class="item">
          <div class="item-top">
            <div class="item-title">
              ${escapeHTML(item.job_id || "Import job")}
              <small>${escapeHTML(`Project ${item.project_id || "-"} · Failed ${formatCount(item.failed_sessions || 0)}`)}</small>
            </div>
            ${pill(item.status || "failed", importJobTone(item.status))}
          </div>
          <div class="step-list">
            <div class="step-line">${escapeHTML(`Processed ${formatCount(item.processed_sessions || 0)} · Uploaded ${formatCount(Number(item.uploaded_sessions || 0) + Number(item.updated_sessions || 0))}`)}</div>
            <div class="step-line">${escapeHTML(importFailureSummary(item) || "Latest failure summary unavailable.")}</div>
            <div class="step-line">${escapeHTML(`Completed ${formatDateTime(item.completed_at || item.created_at)}`)}</div>
          </div>
        </div>
      `).join("")
    : emptyState(
        "No recent failed imports",
        "Partial or failed jobs will show their latest failure summaries here.",
      );
}

function renderUsers(items) {
  adminState.users = items.slice();
  $("listSummary").textContent = `${items.length} user account(s) in the current organization.`;

  if (!items.length) {
    $("userList").innerHTML = emptyState(
      "No users matched the current filters",
      "Broaden the filters or ask the user to sign in with Google first.",
    );
    return;
  }

  const currentUserID = String((adminState.session && adminState.session.user && adminState.session.user.id) || "");
  $("userList").innerHTML = items.map((item) => {
    const isDeleted = String(item.status || "").toLowerCase() === "deleted";
    const isDisabled = String(item.status || "").toLowerCase() === "disabled";
    const isCurrent = String(item.id || "") === currentUserID;
    const actions = [];

    if (!isDeleted) {
      if (!isDisabled) {
        actions.push(`<button class="secondary-button" type="button" data-action="deactivate-user" data-user-id="${escapeHTML(item.id)}">Deactivate</button>`);
      }
      actions.push(`<button class="secondary-button" type="button" data-action="delete-user" data-user-id="${escapeHTML(item.id)}">Delete</button>`);
    }

    return `
      <div class="item">
        <div class="item-top">
          <div class="item-title">
            ${escapeHTML(item.name || item.email || item.id)}
            <small>${escapeHTML(item.email || item.id)}${isCurrent ? " · current session" : ""}</small>
          </div>
          <div class="item-pill-row">
            ${pill(item.role || "member", roleTone(item.role))}
            ${pill(item.status || "active", statusTone(item.status))}
            ${pill(item.source || "managed", sourceTone(item.source))}
          </div>
        </div>
        <div class="step-list">
          <div class="step-line">${escapeHTML(`User ID ${item.id}`)}</div>
          <div class="step-line">${escapeHTML(`Created ${formatDateTime(item.created_at)}`)}</div>
          <div class="step-line">${escapeHTML(`Last login ${formatDateTime(item.last_login_at)}`)}</div>
          <div class="step-line">${escapeHTML(`Source ${item.source || "unknown"}`)}</div>
        </div>
        ${actions.length ? `<div class="action-row">${actions.join("")}</div>` : ""}
      </div>
    `;
  }).join("");
}

function currentFilters() {
  return {
    search: $("searchInput").value.trim(),
    role: $("roleFilter").value,
    status: $("statusFilter").value,
    includeDeleted: $("includeDeleted").checked,
  };
}

function buildFilterURL() {
  const params = new URLSearchParams();
  const filters = currentFilters();
  if (filters.search) {
    params.set("search", filters.search);
  }
  if (filters.role) {
    params.set("role", filters.role);
  }
  if (filters.status) {
    params.set("status", filters.status);
  }
  if (filters.includeDeleted) {
    params.set("include_deleted", "true");
  }
  const encoded = params.toString();
  return encoded ? `/api/v1/admin/users?${encoded}` : "/api/v1/admin/users";
}

async function withBusy(task) {
  if (adminState.busy) {
    return false;
  }
  adminState.busy = true;
  try {
    await task();
    return true;
  } finally {
    adminState.busy = false;
  }
}

async function loadAdminData(message) {
  try {
    await withBusy(async () => {
      setStatus(message || "Loading admin data...");
      const [userData, importMetrics] = await Promise.all([
        requestJSON(buildFilterURL(), {}, "Failed to load users."),
        requestJSON("/api/v1/admin/import-job-metrics?limit=6", {}, "Failed to load import metrics."),
      ]);
      renderUsers(Array.isArray(userData.items) ? userData.items : []);
      renderAdminImportMetrics(importMetrics);
      setStatus(`Loaded ${adminState.users.length} user account(s) and import telemetry.`);
    });
  } catch (error) {
    if (isUnauthorized(error)) {
      redirectToLanding("Your admin session expired. Sign in again.");
      return;
    }
    setStatus(error instanceof Error ? error.message : "Failed to load users.", true);
  }
}

async function bootstrapSession() {
  try {
    const session = await requestJSON("/api/v1/auth/me", {}, "Failed to read the current session.");
    writeSession(session);
    syncHeader();

    if (String((session.user && session.user.role) || "").toLowerCase() !== "admin") {
      redirectToDashboard("Admin access is required for that page.");
      return false;
    }
    return true;
  } catch (error) {
    redirectToLanding("Sign in again to open the admin page.");
    return false;
  }
}

async function deactivateUser(userID) {
  if (!window.confirm("Deactivate this user? Existing sessions will be revoked.")) {
    return;
  }

  try {
    await withBusy(async () => {
      setStatus("Deactivating user...");
      await requestJSON(
        "/api/v1/admin/users/deactivate",
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ user_id: userID }),
        },
        "Failed to deactivate the user.",
      );
      setStatus("User deactivated.");
      await loadAdminData();
    });
  } catch (error) {
    if (isUnauthorized(error)) {
      redirectToLanding("Your admin session expired. Sign in again.");
      return;
    }
    setStatus(error instanceof Error ? error.message : "Failed to deactivate the user.", true);
  }
}

async function deleteUser(userID) {
  if (!window.confirm("Soft-delete this user? The account will disappear from the default list.")) {
    return;
  }

  try {
    await withBusy(async () => {
      setStatus("Deleting user...");
      await requestJSON(
        "/api/v1/admin/users/delete",
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ user_id: userID }),
        },
        "Failed to delete the user.",
      );
      setStatus("User deleted.");
      await loadAdminData();
    });
  } catch (error) {
    if (isUnauthorized(error)) {
      redirectToLanding("Your admin session expired. Sign in again.");
      return;
    }
    setStatus(error instanceof Error ? error.message : "Failed to delete the user.", true);
  }
}

async function signOut() {
  try {
    await requestJSON(
      "/api/v1/auth/logout",
      {
        method: "POST",
      },
      "Failed to sign out.",
    );
  } catch (error) {
    if (!isUnauthorized(error)) {
      setStatus(error instanceof Error ? error.message : "Failed to sign out.", true);
      return;
    }
  }

  clearSession();
  window.location.replace("/");
}

function bindActions() {
  $("filterForm").addEventListener("submit", (event) => {
    event.preventDefault();
    loadAdminData();
  });
  $("refreshBtn").addEventListener("click", () => loadAdminData("Refreshing admin data..."));
  $("signOutBtn").addEventListener("click", signOut);
  $("userList").addEventListener("click", (event) => {
    const target = event.target;
    if (!(target instanceof HTMLElement)) {
      return;
    }
    const userID = target.dataset.userId;
    if (!userID) {
      return;
    }
    switch (target.dataset.action) {
      case "deactivate-user":
        deactivateUser(userID);
        break;
      case "delete-user":
        deleteUser(userID);
        break;
      default:
        break;
    }
  });
}

async function init() {
  bindActions();
  const ready = await bootstrapSession();
  if (!ready) {
    return;
  }
  await loadAdminData();
}

init();
