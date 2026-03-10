    const STORAGE_KEYS = {
      sessionUser: "agentopt_session_user",
      sessionOrg: "agentopt_session_org",
      activeTab: "agentopt_dashboard_tab",
      onboardingDone: "agentopt_onboarding_done"
    };
    const TAB_IDS = ["overview", "cli"];
    const WIZARD_STEPS = 4;

    const state = {
      busy: false,
      activeTab: "overview",
      selectedProjectID: "",
      recommendationIndex: new Map(),
      session: null,
      wizardStep: 0
    };

    const $ = (id) => document.getElementById(id);

    function escapeHTML(value) {
      return String(value == null ? "" : value).replace(/[&<>"']/g, (char) => ({
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        "\"": "&quot;",
        "'": "&#39;"
      }[char]));
    }

    function escapeAttr(value) {
      return escapeHTML(value);
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
        // Storage is optional for the dashboard; ignore browser restrictions.
      }
    }

    function readJSONStorage(key, fallback = null) {
      const value = readStorage(key);
      if (!value) {
        return fallback;
      }
      try {
        return JSON.parse(value);
      } catch (error) {
        return fallback;
      }
    }

    function setStatus(text, isError = false) {
      const el = $("status");
      el.textContent = text;
      el.dataset.tone = isError ? "error" : "info";
    }

    /* ── Wizard ── */

    function showWizard() {
      $("onboardingWizard").classList.remove("is-hidden");
      $("mainDashboard").classList.remove("is-visible");
      setWizardStep(0);
    }

    function hideWizard() {
      $("onboardingWizard").classList.add("is-hidden");
      $("mainDashboard").classList.add("is-visible");
      writeStorage(STORAGE_KEYS.onboardingDone, "1");
    }

    function setWizardStep(step) {
      state.wizardStep = Math.max(0, Math.min(step, WIZARD_STEPS - 1));
      const dots = $("wizardProgress").children;
      const steps = document.querySelectorAll("[data-wizard-step]");

      for (let i = 0; i < dots.length; i++) {
        dots[i].classList.toggle("is-done", i < state.wizardStep);
        dots[i].classList.toggle("is-active", i === state.wizardStep);
      }

      steps.forEach((el) => {
        el.classList.toggle("is-active", Number(el.dataset.wizardStep) === state.wizardStep);
      });
    }

    function updateWizardCommands() {
      const origin = window.location.origin || "http://127.0.0.1:8082";
      const wizLogin = $("wizLoginCmd");
      if (wizLogin) {
        wizLogin.textContent = `./agentopt login --server ${origin}`;
      }
    }

    /* ── Tabs ── */

    function setActiveTab(nextTab) {
      const tab = TAB_IDS.includes(nextTab) ? nextTab : "overview";
      state.activeTab = tab;
      writeStorage(STORAGE_KEYS.activeTab, tab);

      document.querySelectorAll("[data-tab]").forEach((button) => {
        const active = button.dataset.tab === tab;
        button.classList.toggle("is-active", active);
        button.setAttribute("aria-pressed", active ? "true" : "false");
      });

      document.querySelectorAll("[data-tab-panel]").forEach((panel) => {
        const active = panel.dataset.tabPanel === tab;
        panel.classList.toggle("is-active", active);
        panel.hidden = !active;
      });
    }

    /* ── Session ── */

    function cacheSession(session) {
      state.session = session;
      writeStorage(STORAGE_KEYS.sessionUser, JSON.stringify((session && session.user) || {}));
      writeStorage(STORAGE_KEYS.sessionOrg, JSON.stringify((session && session.organization) || {}));
    }

    function clearSession() {
      [
        STORAGE_KEYS.sessionUser,
        STORAGE_KEYS.sessionOrg,
        STORAGE_KEYS.activeTab
      ].forEach((key) => writeStorage(key, ""));
      state.session = null;
      state.selectedProjectID = "";
    }

    function renderSessionContext() {
      const session = state.session || {
        user: readJSONStorage(STORAGE_KEYS.sessionUser, {}) || {},
        organization: readJSONStorage(STORAGE_KEYS.sessionOrg, {}) || {}
      };
      const user = session.user || {};
      const org = session.organization || {};

      $("sessionHeading").textContent = org.name
        ? `${org.name} workspace dashboard`
        : "Review AI usage and approve recommended changes for your workspace.";
      $("sessionSummary").textContent = user.name
        ? `Signed in as ${user.name}. Review your AI usage history, see analysis results, and approve or decline recommended changes.`
        : "Sign in on the landing page to review your AI usage and approve recommended changes.";

      $("topBarUser").textContent = user.name || user.email || "-";
      $("topBarOrg").textContent = org.name || org.id || "-";
    }

    function syncBusyUI() {
      const busy = state.busy;
      const loadBtn = $("loadBtn");
      const issueTokenBtn = $("issueTokenBtn");

      loadBtn.disabled = busy;
      loadBtn.textContent = busy ? "Refreshing..." : "Refresh";
      issueTokenBtn.disabled = busy;
      issueTokenBtn.textContent = busy ? "Creating token..." : "Create CLI token";

      document.querySelectorAll("button[data-action]").forEach((button) => {
        button.disabled = busy;
      });
    }

    async function requestJSON(url, options = {}, fallbackMessage = "Request failed.") {
      const headers = Object.assign({}, options.headers || {});
      const response = await fetch(url, Object.assign({ credentials: "same-origin" }, options, { headers }));
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

      if (!response.ok) {
        const failure = new Error((envelope && (envelope.msg || envelope.message)) || fallbackMessage);
        failure.status = response.status;
        throw failure;
      }
      if (!envelope || typeof envelope !== "object") {
        const unexpected = new Error("The server returned an unexpected response.");
        unexpected.status = response.status;
        throw unexpected;
      }
      if (envelope.code !== 0) {
        const applicationError = new Error(envelope.msg || envelope.message || fallbackMessage);
        applicationError.status = response.status;
        throw applicationError;
      }

      return envelope.data || {};
    }

    function restorePreferences() {
      state.activeTab = readStorage(STORAGE_KEYS.activeTab, "overview");
      state.session = {
        user: readJSONStorage(STORAGE_KEYS.sessionUser, {}) || {},
        organization: readJSONStorage(STORAGE_KEYS.sessionOrg, {}) || {}
      };
      renderSessionContext();
    }

    function isUnauthorized(error) {
      return Boolean(error && typeof error === "object" && error.status === 401);
    }

    function redirectToLanding(message) {
      clearSession();
      if (message) {
        try {
          window.sessionStorage.setItem("agentopt_redirect_notice", message);
        } catch (error) {
          // Ignore sessionStorage failures and continue the redirect.
        }
      }
      window.location.replace("/");
    }

    async function ensureSession() {
      try {
        const session = await requestJSON("/api/v1/auth/me", {}, "Failed to restore the signed-in session.");
        cacheSession(session);
        renderSessionContext();
        return true;
      } catch (error) {
        if (isUnauthorized(error)) {
          redirectToLanding("Your session expired. Sign in again.");
          return false;
        }
        throw error;
      }
    }

    async function copyText(value) {
      const text = String(value || "").trim();
      if (!text) {
        return false;
      }

      if (navigator.clipboard && window.isSecureContext) {
        try {
          await navigator.clipboard.writeText(text);
          return true;
        } catch (error) {
          // Fall back to a temporary textarea below.
        }
      }

      const textarea = document.createElement("textarea");
      textarea.value = text;
      textarea.setAttribute("readonly", "readonly");
      textarea.style.position = "absolute";
      textarea.style.left = "-9999px";
      document.body.appendChild(textarea);
      textarea.select();

      let copied = false;
      try {
        copied = document.execCommand("copy");
      } catch (error) {
        copied = false;
      }

      document.body.removeChild(textarea);
      return copied;
    }

    async function copyCommand(targetID, label) {
      const target = $(targetID);
      if (!target) {
        setStatus("The requested command block is missing.", true);
        return;
      }

      const copied = await copyText(target.textContent);
      if (!copied) {
        setStatus(`Failed to copy the ${label || "command"}.`, true);
        return;
      }

      setStatus(`Copied the ${label || "command"}.`);
    }

    function toArray(value) {
      return Array.isArray(value) ? value : [];
    }

    function formatCount(value) {
      return new Intl.NumberFormat("en-US").format(Number(value || 0));
    }

    function formatDateTime(value) {
      if (!value) {
        return "Not yet";
      }

      const date = new Date(value);
      if (Number.isNaN(date.getTime())) {
        return String(value);
      }

      return new Intl.DateTimeFormat("en-US", {
        dateStyle: "medium",
        timeStyle: "short"
      }).format(date);
    }

    function titleize(value) {
      return String(value || "")
        .replace(/[_-]+/g, " ")
        .replace(/\s+/g, " ")
        .trim()
        .replace(/\b\w/g, (char) => char.toUpperCase());
    }

    function baseName(path) {
      const parts = String(path || "").split(/[\\/]/);
      return parts[parts.length - 1] || "workspace settings";
    }

    function emptyState(title, description) {
      return `
        <div class="item empty">
          <div class="item-title">${escapeHTML(title)}</div>
          <div class="meta">${escapeHTML(description)}</div>
        </div>
      `;
    }

    function pill(label, tone) {
      const safeTone = tone ? ` pill-${tone}` : "";
      return `<span class="pill${safeTone}">${escapeHTML(label)}</span>`;
    }

    function riskTone(risk) {
      const raw = String(risk || "").toLowerCase();
      if (raw.includes("high")) {
        return "danger";
      }
      if (raw.includes("medium")) {
        return "warn";
      }
      return "good";
    }

    function tokenTone(status) {
      const raw = String(status || "").toLowerCase();
      if (raw === "active") {
        return "good";
      }
      if (raw === "expired") {
        return "warn";
      }
      if (raw === "revoked") {
        return "danger";
      }
      return "sky";
    }

    function truncateText(value, maxLength = 96) {
      const text = String(value || "").trim();
      if (text.length <= maxLength) {
        return text;
      }
      return `${text.slice(0, maxLength - 1)}...`;
    }

    function normalizeInlineText(value) {
      return String(value || "")
        .replace(/\r\n/g, "\n")
        .replace(/\s+/g, " ")
        .trim();
    }

    function extractUserRequest(raw) {
      let text = String(raw || "").trim();
      if (!text) {
        return "";
      }

      const requestMarkers = [
        "## My request for Codex:",
        "## My request for Codex",
        "My request for Codex:"
      ];
      for (const marker of requestMarkers) {
        if (text.includes(marker)) {
          return normalizeInlineText(text.slice(text.lastIndexOf(marker) + marker.length));
        }
      }

      text = text
        .replace(/<environment_context>[\s\S]*?<\/environment_context>/gi, " ")
        .replace(/# AGENTS\.md instructions[\s\S]*?<\/INSTRUCTIONS>/gi, " ");

      const lines = text.replace(/\r\n/g, "\n").split("\n");
      const cleaned = [];
      let skipInstructions = false;
      let skipOpenTabs = false;

      for (const rawLine of lines) {
        const line = rawLine.trim();
        if (!line) {
          continue;
        }
        if (/^# AGENTS\.md instructions/i.test(line)) {
          skipInstructions = true;
          continue;
        }
        if (skipInstructions) {
          if (/^<\/INSTRUCTIONS>$/i.test(line)) {
            skipInstructions = false;
          }
          continue;
        }
        if (/^# Context from my IDE setup:?$/i.test(line)) {
          continue;
        }
        if (/^## Open tabs:?$/i.test(line)) {
          skipOpenTabs = true;
          continue;
        }
        if (/^## My request for Codex:?$/i.test(line)) {
          skipOpenTabs = false;
          continue;
        }
        if (skipOpenTabs) {
          if (/^##\s+/.test(line)) {
            skipOpenTabs = false;
          } else {
            continue;
          }
        }
        if (/^<image>$/i.test(line) || /^<\/image>$/i.test(line)) {
          continue;
        }
        cleaned.push(line);
      }

      return normalizeInlineText(cleaned.join(" "));
    }

    function sessionPrimaryRequest(item) {
      const queries = toArray(item.raw_queries);
      for (const query of queries) {
        const request = extractUserRequest(query);
        if (request) {
          return request;
        }
      }
      return "";
    }

    function stepSummary(step) {
      if (step && step.summary) {
        return String(step.summary);
      }
      if (step && step.operation && String(step.operation).includes("append")) {
        return `Add guidance to ${baseName(step.file_path)}`;
      }
      if (step && step.target_file) {
        return `Update ${baseName(step.target_file)}`;
      }
      if (step && step.file_path) {
        return `Update ${baseName(step.file_path)}`;
      }
      return "Update workspace configuration";
    }

    function recommendationPlanSummary(item) {
      const steps = toArray(item.change_plan);
      if (!steps.length) {
        return "One settings update is ready to review.";
      }
      if (steps.length === 1) {
        return stepSummary(steps[0]);
      }
      return `${steps.length} coordinated updates will be applied together.`;
    }

    function patchPreviewSummary(items) {
      const steps = toArray(items);
      if (!steps.length) {
        return "One reviewed update is queued for this workspace.";
      }
      if (steps.length === 1) {
        return stepSummary(steps[0]);
      }
      return `${steps.length} reviewed updates will land together.`;
    }

    function recommendationTitle(recommendationID) {
      const recommendation = state.recommendationIndex.get(recommendationID);
      return recommendation ? recommendation.title : "Configuration update";
    }

    function recommendationSummary(recommendationID) {
      const recommendation = state.recommendationIndex.get(recommendationID);
      return recommendation ? recommendation.summary : "A reviewed configuration update for this workspace.";
    }

    function tokenSummary(item) {
      if (item.last_used_at) {
        return `Last used ${formatDateTime(item.last_used_at)}.`;
      }
      if (item.status === "revoked" && item.revoked_at) {
        return `Revoked ${formatDateTime(item.revoked_at)}.`;
      }
      if (item.expires_at) {
        return `Expires ${formatDateTime(item.expires_at)}.`;
      }
      return "Ready to authenticate a local CLI install.";
    }

    function workloadNarrative(overview) {
      const action = String(overview.action_summary || "").trim();
      const outcome = String(overview.outcome_summary || "").trim();
      const input = Number(overview.avg_input_tokens_per_query || 0);
      const output = Number(overview.avg_output_tokens_per_query || 0);
      const tokenRead = input > 0 || output > 0
        ? (input >= output
          ? " Prompt-side token usage is currently the larger share of each captured query."
          : " Response-side token usage is currently the larger share of each captured query.")
        : "";
      const combined = `${action} ${outcome}${tokenRead}`.trim();
      return combined || "AgentOpt is collecting enough setup and session context to produce steadier recommendations.";
    }

    function sessionTone(item) {
      const queries = toArray(item.raw_queries).length;
      const tokens = Number(item.token_in || 0) + Number(item.token_out || 0);
      if (queries >= 3 && tokens >= 6000) {
        return "warn";
      }
      if (queries > 0) {
        return "good";
      }
      return "sky";
    }

    function sessionLabel(item) {
      const tone = sessionTone(item);
      if (tone === "good") {
        return "Captured";
      }
      if (tone === "warn") {
        return "Heavy prompt setup";
      }
      return "Small sample";
    }

    function sessionSummaryLines(item) {
      const queries = toArray(item.raw_queries);
      const inputTokens = Number(item.token_in || 0);
      const outputTokens = Number(item.token_out || 0);
      const totalTokens = inputTokens + outputTokens;
      const primaryRequest = sessionPrimaryRequest(item);
      const lines = [`${formatCount(queries.length)} raw quer${queries.length === 1 ? "y" : "ies"} captured from the CLI.`];

      if (inputTokens > 0 || outputTokens > 0) {
        lines.push(`${formatCount(inputTokens)} input and ${formatCount(outputTokens)} output tokens were uploaded for this session.`);
      } else if (totalTokens > 0) {
        lines.push(`${formatCount(totalTokens)} total tokens were uploaded for this session.`);
      }
      if (primaryRequest) {
        const additionalRequests = Math.max(queries.length - 1, 0);
        lines.push(additionalRequests > 0
          ? `${formatCount(additionalRequests)} more user request${additionalRequests === 1 ? "" : "s"} were captured in this session.`
          : "This session captured a single user request.");
      } else if (queries[0]) {
        lines.push(`User request: ${truncateText(normalizeInlineText(queries[0]))}`);
      }
      if (queries.length >= 2) {
        lines.push("Repeated phrasing here can be turned into a shared instruction block.");
      } else {
        lines.push("More raw queries will make the next instruction suggestion more specific.");
      }

      return lines;
    }

    /* ── Render ── */

    function renderOverview(overview) {
      const activeRecs = Number(overview.active_recommendations || 0);
      $("activeRecommendations").textContent = formatCount(activeRecs);
      $("totalSessions").textContent = formatCount(overview.total_sessions);
      $("avgInputTokensPerQuery").textContent = formatCount(Math.round(Number(overview.avg_input_tokens_per_query || 0)));
      $("avgOutputTokensPerQuery").textContent = formatCount(Math.round(Number(overview.avg_output_tokens_per_query || 0)));

      const totalSessions = Number(overview.total_sessions || 0);

      $("activeRecommendationsMeta").textContent = activeRecs === 0
        ? "No suggestions yet. Upload more sessions to generate recommendations."
        : `${formatCount(activeRecs)} configuration suggestion(s) from the analysis engine.`;
      $("totalSessionsMeta").textContent = totalSessions === 0
        ? "Upload sessions from the CLI to start tracking AI usage."
        : `${formatCount(totalSessions)} AI usage session(s) collected from the CLI so far.`;
      $("avgTokensMeta").textContent = `${formatCount(overview.total_input_tokens || 0)} input / ${formatCount(overview.total_output_tokens || 0)} output tokens uploaded so far.`;
      $("overviewNarrative").textContent = workloadNarrative(overview);
    }

    function renderActionItems(recommendations, reviewQueue) {
      const html = [];

      reviewQueue.forEach((item) => {
        html.push(`
          <div class="item">
            <div class="item-top">
              <div class="item-title">
                ${escapeHTML(recommendationTitle(item.recommendation_id))}
                <small>${escapeHTML(recommendationSummary(item.recommendation_id))}</small>
              </div>
              ${pill("Needs approval", "warn")}
            </div>
            <div class="step-list">
              <div class="step-line">${escapeHTML(patchPreviewSummary(item.patch_preview))}</div>
            </div>
            <div class="action-row">
              <button class="primary-button" type="button" data-action="review-plan" data-apply-id="${escapeAttr(item.apply_id)}" data-decision="approve">Approve</button>
              <button class="secondary-button" type="button" data-action="review-plan" data-apply-id="${escapeAttr(item.apply_id)}" data-decision="reject">Decline</button>
            </div>
          </div>
        `);
      });

      const reviewedIDs = new Set(reviewQueue.map((item) => item.recommendation_id));
      const newRecs = recommendations.filter((item) => !reviewedIDs.has(item.id));

      newRecs.slice(0, 5).forEach((item) => {
        html.push(`
          <div class="item">
            <div class="item-top">
              <div class="item-title">
                ${escapeHTML(item.title)}
                <small>${escapeHTML(truncateText(item.summary, 150))}</small>
              </div>
              ${pill(item.risk || "Low risk", riskTone(item.risk))}
            </div>
            <div class="step-list">
              <div class="step-line">${escapeHTML(truncateText(item.reason, 150))}</div>
              <div class="step-line">${escapeHTML(recommendationPlanSummary(item))}</div>
            </div>
            <div class="action-row">
              <button class="primary-button" type="button" data-action="approve-recommendation" data-recommendation-id="${escapeAttr(item.id)}">Approve</button>
              <button class="secondary-button" type="button" data-action="decline-recommendation" data-recommendation-id="${escapeAttr(item.id)}">Decline</button>
            </div>
          </div>
        `);
      });

      $("actionItemList").innerHTML = html.length
        ? html.join("")
        : emptyState(
            "No suggestions yet",
            "Upload sessions from the CLI and AgentOpt will start proposing configuration improvements."
          );
    }

    function renderSessionSummaries(items) {
      if (!items.length) {
        $("sessionSummaryList").innerHTML = emptyState(
          "No sessions uploaded yet",
          "Run `agentopt session --recent 5` from the CLI to upload your recent AI usage sessions."
        );
        return;
      }

      $("sessionSummaryList").innerHTML = items.slice(0, 5).map((item) => `
        <div class="item">
          <div class="item-top">
            <div class="item-title">
              ${escapeHTML(truncateText(sessionPrimaryRequest(item) || (toArray(item.raw_queries).length > 0 ? "User request" : "Recent work"), 84))}
              <small>Recorded ${escapeHTML(formatDateTime(item.timestamp))}</small>
            </div>
            ${pill(sessionLabel(item), sessionTone(item))}
          </div>
          <div class="step-list">${sessionSummaryLines(item).slice(0, 3).map((line) => `<div class="step-line">${escapeHTML(line)}</div>`).join("")}</div>
        </div>
      `).join("");
    }

    function renderCLITokens(items) {
      if (!items.length) {
        $("cliTokenList").innerHTML = emptyState(
          "No CLI tokens issued yet",
          "Create a CLI token when you want a new local machine to authenticate."
        );
        return;
      }

      $("cliTokenList").innerHTML = items.map((item) => {
        const canRevoke = String(item.status || "").toLowerCase() === "active";
        const identity = item.label || item.token_prefix || item.token_id;
        return `
          <div class="item">
            <div class="item-top">
              <div class="item-title">
                ${escapeHTML(identity)}
                <small>${escapeHTML(item.token_prefix || item.token_id)}</small>
              </div>
              ${pill(titleize(item.status || "active"), tokenTone(item.status))}
            </div>
            <div class="step-list">
              <div class="step-line">${escapeHTML(`Issued ${formatDateTime(item.created_at)}.`)}</div>
              <div class="step-line">${escapeHTML(tokenSummary(item))}</div>
            </div>
            ${canRevoke ? `
              <div class="action-row">
                <button class="secondary-button" type="button" data-action="revoke-cli-token" data-token-id="${escapeAttr(item.token_id)}">Revoke token</button>
              </div>
            ` : ""}
          </div>
        `;
      }).join("");
    }

    /* ── Actions ── */

    async function withBusy(task) {
      if (state.busy) {
        return false;
      }

      state.busy = true;
      syncBusyUI();

      try {
        await task();
        return true;
      } finally {
        state.busy = false;
        syncBusyUI();
      }
    }

    function requireUser() {
      const userID = state.session && state.session.user ? state.session.user.id : "";
      if (!userID) {
        redirectToLanding("Sign in again to continue.");
      }
      return userID;
    }

    async function approveRecommendation(recommendationID) {
      const requestedBy = requireUser();
      if (!requestedBy) {
        return;
      }

      try {
        await withBusy(async () => {
          const data = await requestJSON("/api/v1/recommendations/apply", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              recommendation_id: recommendationID,
              requested_by: requestedBy,
              scope: "user"
            })
          }, "Failed to create the change plan.");

          if (data.policy_mode !== "auto_approved" && data.apply_id) {
            await requestJSON("/api/v1/change-plans/review", {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({
                apply_id: data.apply_id,
                decision: "approve",
                reviewed_by: requestedBy,
                review_note: "dashboard approve"
              })
            }, "Failed to approve the plan.");
          }

          await load({ skipBusy: true });
          setStatus("Approved. The local CLI can pick up this change on the next sync.");
        });
      } catch (error) {
        if (isUnauthorized(error)) {
          redirectToLanding("Your session expired. Sign in again.");
          return;
        }
        setStatus(error instanceof Error ? error.message : "Failed to approve.", true);
      }
    }

    async function declineRecommendation(recommendationID) {
      const requestedBy = requireUser();
      if (!requestedBy) {
        return;
      }

      try {
        await withBusy(async () => {
          const data = await requestJSON("/api/v1/recommendations/apply", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              recommendation_id: recommendationID,
              requested_by: requestedBy,
              scope: "user"
            })
          }, "Failed to create the change plan.");

          if (data.apply_id) {
            await requestJSON("/api/v1/change-plans/review", {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({
                apply_id: data.apply_id,
                decision: "reject",
                reviewed_by: requestedBy,
                review_note: "dashboard decline"
              })
            }, "Failed to decline the plan.");
          }

          await load({ skipBusy: true });
          setStatus("Declined. The suggestion has been dismissed.");
        });
      } catch (error) {
        if (isUnauthorized(error)) {
          redirectToLanding("Your session expired. Sign in again.");
          return;
        }
        setStatus(error instanceof Error ? error.message : "Failed to decline.", true);
      }
    }

    async function reviewPlan(applyID, decision) {
      const reviewer = requireUser();
      if (!reviewer) {
        return;
      }

      try {
        await withBusy(async () => {
          await requestJSON("/api/v1/change-plans/review", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              apply_id: applyID,
              decision,
              reviewed_by: reviewer,
              review_note: `dashboard ${decision}`
            })
          }, "Failed to update the review decision.");

          await load({ skipBusy: true });
          setStatus(decision === "approve"
            ? "Approved. The local CLI can pick up this change on the next sync."
            : "Declined. The plan will stay out of the rollout queue.");
        });
      } catch (error) {
        if (isUnauthorized(error)) {
          redirectToLanding("Your session expired. Sign in again.");
          return;
        }
        setStatus(error instanceof Error ? error.message : "Failed to update the review decision.", true);
      }
    }

    async function issueCLIToken() {
      try {
        await withBusy(async () => {
          const data = await requestJSON("/api/v1/auth/cli-tokens", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              label: "CLI login token"
            })
          }, "Failed to issue a CLI token.");

          $("issuedCliToken").textContent = data.token || "Token was issued.";
          $("cliTokenMeta").textContent = data.expires_at
            ? `CLI token issued for ${data.label || "CLI login"} and expires ${formatDateTime(data.expires_at)}. Paste it into \`agentopt login\` on the machine you want to connect.`
            : "CLI token issued. Paste it into `agentopt login` on the machine you want to connect.";

          const wizOutput = $("wizTokenOutput");
          if (wizOutput) {
            wizOutput.textContent = data.token || "Token was issued.";
          }

          const tokens = await requestJSON("/api/v1/auth/cli-tokens", {}, "Failed to refresh issued CLI tokens.");
          renderCLITokens(toArray(tokens.items));
          setStatus("CLI token issued. Paste it into `agentopt login` on the device you want to connect.");
        });
      } catch (error) {
        if (isUnauthorized(error)) {
          redirectToLanding("Your session expired. Sign in again.");
          return;
        }
        setStatus(error instanceof Error ? error.message : "Failed to issue a CLI token.", true);
      }
    }

    async function revokeCLIToken(tokenID) {
      try {
        await withBusy(async () => {
          await requestJSON("/api/v1/auth/cli-tokens/revoke", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ token_id: tokenID })
          }, "Failed to revoke the CLI token.");

          const tokens = await requestJSON("/api/v1/auth/cli-tokens", {}, "Failed to refresh issued CLI tokens.");
          renderCLITokens(toArray(tokens.items));
          setStatus("CLI token revoked. That token can no longer authenticate a local CLI install.");
        });
      } catch (error) {
        if (isUnauthorized(error)) {
          redirectToLanding("Your session expired. Sign in again.");
          return;
        }
        setStatus(error instanceof Error ? error.message : "Failed to revoke the CLI token.", true);
      }
    }

    async function signOut() {
      try {
        await requestJSON("/api/v1/auth/logout", {
          method: "POST"
        }, "Failed to sign out.");
      } catch (error) {
        if (!isUnauthorized(error)) {
          setStatus(error instanceof Error ? error.message : "Failed to sign out.", true);
          return;
        }
      }

      clearSession();
      window.location.replace("/");
    }

    /* ── Load ── */

    async function load(options = {}) {
      const manageBusy = !options.skipBusy;
      if (manageBusy && state.busy) {
        return;
      }

      const orgID = state.session && state.session.organization ? state.session.organization.id : "";
      if (!orgID) {
        redirectToLanding("Sign in again to open the dashboard.");
        return;
      }

      if (manageBusy) {
        state.busy = true;
        syncBusyUI();
      }

      setStatus("Loading workspace signals...");

      try {
        const [overview, projectsData, cliTokensData] = await Promise.all([
          requestJSON(`/api/v1/dashboard/overview?org_id=${encodeURIComponent(orgID)}`, {}, "Failed to load the dashboard overview."),
          requestJSON(`/api/v1/projects?org_id=${encodeURIComponent(orgID)}`, {}, "Failed to load the shared workspace."),
          requestJSON("/api/v1/auth/cli-tokens", {}, "Failed to load issued CLI tokens.")
        ]);

        const projects = toArray(projectsData.items);
        const projectID = projects[0] ? projects[0].id : "";
        state.selectedProjectID = projectID;
        renderSessionContext();

        renderOverview(overview);
        renderCLITokens(toArray(cliTokensData.items));

        state.recommendationIndex = new Map();

        let recommendations = [];
        let reviewQueue = [];
        let sessions = [];

        if (projectID) {
          const [recommendationsData, reviewData, sessionData] = await Promise.all([
            requestJSON(`/api/v1/recommendations?project_id=${encodeURIComponent(projectID)}`, {}, "Failed to load workspace suggestions."),
            requestJSON(`/api/v1/change-plans?project_id=${encodeURIComponent(projectID)}&status=awaiting_review`, {}, "Failed to load the approval queue."),
            requestJSON(`/api/v1/session-summaries?project_id=${encodeURIComponent(projectID)}&limit=5`, {}, "Failed to load recent sessions.")
          ]);

          recommendations = toArray(recommendationsData.items);
          reviewQueue = toArray(reviewData.items);
          sessions = toArray(sessionData.items);

          recommendations.forEach((item) => {
            state.recommendationIndex.set(item.id, item);
          });
        }

        renderActionItems(recommendations, reviewQueue);
        renderSessionSummaries(sessions);

        const shouldShowWizard = !projectID && readStorage(STORAGE_KEYS.onboardingDone) !== "1";
        if (shouldShowWizard) {
          showWizard();
        } else {
          hideWizard();
        }

        if (projectID) {
          const workspaceName = projects.find((item) => item.id === projectID)?.name || "Shared workspace";
          setStatus(`Showing ${workspaceName}. Review your AI usage and approve recommended changes.`);
        } else {
          setStatus("No workspace connected yet. Run the CLI to start uploading sessions.");
        }
      } catch (error) {
        if (isUnauthorized(error)) {
          redirectToLanding("Your session expired. Sign in again.");
          return;
        }
        setStatus(error instanceof Error ? error.message : "Failed to load the dashboard.", true);
      } finally {
        if (manageBusy) {
          state.busy = false;
          syncBusyUI();
        }
      }
    }

    /* ── Event delegation ── */

    function handleActionClick(event) {
      if (!(event.target instanceof Element)) {
        return;
      }

      const button = event.target.closest("button[data-action]");
      if (!button || button.disabled) {
        return;
      }

      switch (button.dataset.action) {
        case "refresh-dashboard":
          load();
          break;
        case "issue-cli-token":
        case "wizard-issue-token":
          issueCLIToken();
          break;
        case "revoke-cli-token":
          revokeCLIToken(button.dataset.tokenId || "");
          break;
        case "switch-tab":
          setActiveTab(button.dataset.tab || "overview");
          break;
        case "copy-command":
          copyCommand(button.dataset.copyTarget || "", button.dataset.copyLabel || "command");
          break;
        case "approve-recommendation":
          approveRecommendation(button.dataset.recommendationId || "");
          break;
        case "decline-recommendation":
          declineRecommendation(button.dataset.recommendationId || "");
          break;
        case "review-plan":
          reviewPlan(button.dataset.applyId || "", button.dataset.decision || "");
          break;
        case "sign-out":
          signOut();
          break;
        case "wizard-next":
          setWizardStep(state.wizardStep + 1);
          break;
        case "wizard-back":
          setWizardStep(state.wizardStep - 1);
          break;
        case "wizard-done":
        case "skip-wizard":
          hideWizard();
          break;
        default:
          break;
      }
    }

    document.addEventListener("click", handleActionClick);

    async function boot() {
      restorePreferences();
      updateWizardCommands();
      setActiveTab(state.activeTab);
      syncBusyUI();

      $("onboardingWizard").classList.add("is-hidden");
      $("mainDashboard").classList.add("is-visible");

      if (!(await ensureSession())) {
        return;
      }

      load();
    }

    boot();
