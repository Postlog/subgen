// app.js — the subgen admin SPA (Vue 3, global build, no build step). The backend
// serves pure JSON under /admin/api/*; this drives it with fetch.
const { createApp } = Vue;

const app = createApp({
  data() {
    return {
      tab: "users",
      busy: false,      // a mutation is in flight (disables actions)
      actingId: 0,      // user id whose row action is in flight (row spinner)
      toasts: [],
      _tid: 0,
      _uidc: 0,         // client-side uid counter (stable keys for drag-n-drop)
      users: [],
      // Users table is server-paged/filtered: the backend returns one page; these
      // drive the query (q = name substring, userInboundFilter = OR over inbound ids).
      userSearch: "",
      userInboundFilter: [],
      userPage: 1,
      userPerPage: 50,
      userTotal: 0,
      inboundFilterOpen: false, // the inbound-filter popover
      _userTimer: 0,            // debounce handle for the search box
      nodes: [],
      schema: null, // config UI schema (rule/group/policy/provider catalogs), from the backend
      uForm: { open: false, id: 0, name: "", description: "", inbounds: [] },
      nodeForm: { open: false, id: 0, name: "", vpnHost: "", panelBaseURL: "", panelBasePath: "", token: "", inbounds: [] },
      provForm: { open: false, idx: -1 }, // which provider the edit modal is editing

      // cfg: structured mihomo config. groups/rules carry a client-side _uid; a
      // policy ref is encoded as a string `pref` (direct|reject|…|smart|force:<id>|
      // group:<groupUid>) so it binds straight to a <select>.
      cfg: { groups: [], rules: [], providers: [], baseYAML: "", profileTitle: "", filename: "", profileUpdateInterval: 1 },

      // Config scope: which config the editor is bound to. 'base' = the shared base
      // config; 'user' = a per-user custom config (userId/name set). customs lists the
      // users that already have a custom config (for the scope selector).
      cfgScope: { kind: "base", userId: 0, name: "" },
      customs: [],
      customPick: { open: false, userId: 0 }, // user picker for "+ custom config"
    };
  },
  computed: {
    // Every inbound across the fleet, as picker options. label = "<node>-<inbound>"
    // (the mihomo proxy name); id is node_inbounds.id.
    inboundOptions() {
      const out = [];
      for (const n of this.nodes) {
        for (const i of (n.inbounds || [])) {
          out.push({ id: i.id, label: n.name + "-" + i.name, port: i.port });
        }
      }
      return out;
    },
    // Total pages for the users table (at least 1, so the pager always shows "1 из 1").
    userPageCount() { return Math.max(1, Math.ceil(this.userTotal / this.userPerPage)); },
    // Windowed page list for the pager. Always a CONSTANT 7 slots when there are >7
    // pages (first, last, a 3-wide window, and "…" fillers), so the layout width — and
    // thus the ← → arrows — never shifts as you page.
    userPages() {
      const last = this.userPageCount, cur = this.userPage;
      if (last <= 7) return Array.from({ length: last }, (_, i) => i + 1);
      if (cur <= 4) return [1, 2, 3, 4, 5, "...", last];
      if (cur >= last - 3) return [1, "...", last - 4, last - 3, last - 2, last - 1, last];
      return [1, "...", cur - 1, cur, cur + 1, "...", last];
    },
    // Users that don't yet have a custom config — the candidates for "+ custom config".
    usersWithoutCustom() {
      const taken = new Set(this.customs.map((c) => c.userId));
      return this.users.filter((u) => !taken.has(u.id));
    },
    // The provider the edit modal edits (same object ref → edits
    // flow straight back into cfg.providers).
    editProv() { return this.cfg.providers[this.provForm.idx] || null; },
    // Inline validation hints for the routing config.
    cfgWarnings() {
      const w = [];
      const matches = this.cfg.rules.filter((r) => r.type === "MATCH");
      if (!matches.length) w.push("Нет правила MATCH — добавьте catch-all в конце.");
      else if (this.cfg.rules[this.cfg.rules.length - 1].type !== "MATCH") w.push("Правило MATCH должно быть последним.");
      const pUids = new Set(this.cfg.providers.map((p) => p._uid));
      for (const r of this.cfg.rules) {
        if (r.type === "RULE-SET" && (r.providerUid == null || !pUids.has(r.providerUid))) w.push("RULE-SET: не выбран провайдер или он удалён.");
      }
      const uids = new Set(this.cfg.groups.map((g) => g._uid));
      const dangling = (pref) => pref.startsWith("group:") && !uids.has(+pref.slice(6));
      if (this.cfg.rules.some((r) => dangling(r.pref)) || this.cfg.groups.some((g) => g.members.some((m) => dangling(m.pref))))
        w.push("Есть ссылка на удалённую группу.");
      return w;
    },
  },
  methods: {
    uid() { return ++this._uidc; },

    go(tab) { this.tab = tab; this.load(tab); },
    async load(tab) {
      try {
        if (tab === "users") {
          await this.loadNodes(); // inbound options for the filter
          await this.loadUsers();
        } else if (tab === "nodes") {
          await this.loadNodes();
        } else if (tab === "config") {
          await this.loadNodes(); // inbound options for the policy pickers
          if (!this.schema) this.schema = await this.getJSON("/admin/api/config/mihomo/schema");
          await this.loadCustoms();
          // Keep the active scope if its custom config still exists; else fall to base.
          if (this.cfgScope.kind === "user" && !this.customs.some((c) => c.userId === this.cfgScope.userId)) {
            this.cfgScope = { kind: "base", userId: 0, name: "" };
          }
          await this.loadScopeConfig();
        }
      } catch (e) { this.toast(false, "Загрузка: " + e); }
    },
    async loadNodes() { this.nodes = (await this.getJSON("/admin/api/nodes")).nodes || []; },

    // ---- users: server-paged list ----------------------------------------------
    // loadUsers fetches the current page with the active search/inbound filter. If a
    // page ends up past the end (e.g. after deletes), it steps back and refetches.
    async loadUsers() {
      const p = new URLSearchParams();
      const q = this.userSearch.trim();
      if (q) p.set("q", q);
      for (const id of this.userInboundFilter) p.append("inbound", id);
      p.set("page", this.userPage);
      p.set("perPage", this.userPerPage);
      const d = await this.getJSON("/admin/api/users?" + p.toString());
      this.users = d.users || [];
      this.userTotal = d.total || 0;
      if (!this.users.length && this.userTotal && this.userPage > 1) {
        this.userPage = this.userPageCount;
        await this.loadUsers();
      }
    },
    // onUserSearch debounces the search box, then resets to page 1 and reloads.
    onUserSearch() {
      clearTimeout(this._userTimer);
      this._userTimer = setTimeout(() => { this.userPage = 1; this.loadUsers(); }, 250);
    },
    // toggleInboundFilter flips one inbound in the OR-filter and reloads from page 1.
    toggleInboundFilter(id) {
      const i = this.userInboundFilter.indexOf(id);
      if (i === -1) this.userInboundFilter.push(id); else this.userInboundFilter.splice(i, 1);
      this.userPage = 1;
      this.loadUsers();
    },
    clearInboundFilter() {
      if (!this.userInboundFilter.length) return;
      this.userInboundFilter = [];
      this.userPage = 1;
      this.loadUsers();
    },
    gotoUserPage(p) {
      p = Math.min(Math.max(1, p), this.userPageCount);
      if (p === this.userPage) return;
      this.userPage = p;
      this.loadUsers();
    },

    // ---- config: scope (base vs per-user custom) -------------------------------
    // loadCustoms pulls both the custom-config owners and the full id+name user list
    // (the picker's source — kept off the paged /admin/api/users).
    async loadCustoms() {
      const d = await this.getJSON("/admin/api/config/mihomo/customs");
      this.customs = d.customs || [];
      this.users = d.users || [];
    },
    // loadScopeConfig fetches the config for the active scope into the editor.
    async loadScopeConfig() {
      const url = this.cfgScope.kind === "user"
        ? "/admin/api/config/mihomo?user=" + this.cfgScope.userId
        : "/admin/api/config/mihomo";
      this.loadConfig(await this.getJSON(url));
    },
    // onScopeChange handles the scope <select>: base, a custom config, or "+ new".
    onScopeChange(val) {
      if (val === "+new") { this.customPick = { open: true, userId: 0 }; return; }
      if (val === "base") { this.switchScope({ kind: "base", userId: 0, name: "" }); return; }
      const userId = +val.slice(5); // "user:<id>"
      const c = this.customs.find((x) => x.userId === userId);
      this.switchScope({ kind: "user", userId, name: c ? c.name : "" });
    },
    async switchScope(scope) {
      this.cfgScope = scope;
      await this.loadScopeConfig();
    },
    // createCustom clones the base into a new custom config for the picked user, then
    // switches the editor to it.
    async createCustom() {
      const userId = this.customPick.userId;
      if (!userId) return;
      const d = await this.post("/admin/api/config/mihomo/custom/create", { userId });
      if (!d.ok) return;
      this.customPick.open = false;
      await this.loadCustoms();
      const c = this.customs.find((x) => x.userId === userId);
      await this.switchScope({ kind: "user", userId, name: c ? c.name : "" });
    },
    // deleteCustom drops the active user's custom config and returns to the base.
    async deleteCustom() {
      if (this.cfgScope.kind !== "user") return;
      if (!confirm("Удалить кастомный конфиг пользователя " + this.cfgScope.name + "?")) return;
      const d = await this.post("/admin/api/config/mihomo/custom/delete", { userId: this.cfgScope.userId });
      if (!d.ok) return;
      await this.loadCustoms();
      await this.switchScope({ kind: "base", userId: 0, name: "" });
    },

    // ---- config: load / encode -------------------------------------------------
    loadConfig(c) {
      const groups = (c.groups || []).map((g) => ({
        _uid: this.uid(), name: g.name, type: g.type, url: g.url || "",
        interval: g.interval || 0, tolerance: g.tolerance || 0, lazy: !!g.lazy, members: [],
      }));
      const gUid = groups.map((g) => g._uid); // returned index -> uid
      (c.groups || []).forEach((g, gi) => {
        groups[gi].members = (g.members || []).map((m) => ({ _uid: this.uid(), pref: this.refToPref(m, gUid) }));
      });
      this.cfg.groups = groups;
      this.cfg.providers = (c.providers || []).map((p) => ({ ...p, _uid: this.uid() }));
      const pUid = this.cfg.providers.map((p) => p._uid); // providerIdx -> uid
      this.cfg.rules = (c.rules || []).map((r) => ({
        _uid: this.uid(), type: r.type, value: r.value || "",
        providerUid: (r.providerIdx === undefined || r.providerIdx === null) ? null : pUid[r.providerIdx],
        noResolve: !!r.noResolve, pref: this.refToPref(r.target, gUid),
        conditions: this.loadConditions(r.conditions, pUid),
      }));
      this.cfg.baseYAML = c.baseYAML || "";
      this.cfg.profileTitle = c.profileTitle || "";
      this.cfg.filename = c.filename || "";
      this.cfg.profileUpdateInterval = c.profileUpdateInterval ?? 1;
    },
    // loadConditions maps API sub-conditions (a logical rule's children) into the editor
    // model, recursively. A RULE-SET sub-condition's providerIdx is resolved to the
    // provider's stable uid (pUid: providerIdx -> uid), like a top-level RULE-SET rule.
    loadConditions(conds, pUid) {
      return (conds || []).map((c) => ({
        _uid: cuid(), type: c.type, value: c.value || "",
        providerUid: (c.providerIdx === undefined || c.providerIdx === null) ? null : pUid[c.providerIdx],
        conditions: this.loadConditions(c.conditions, pUid),
      }));
    },
    // dumpConditions serialises a logical rule's children back to the API shape,
    // recursively: a logical child carries nested conditions, a RULE-SET child a
    // providerIdx, every other child a value.
    dumpConditions(conds, provUidToIdx) {
      return (conds || []).map((c) => {
        if (this.isLogical(c.type)) return { type: c.type, conditions: this.dumpConditions(c.conditions, provUidToIdx) };
        if (this.isRuleSet(c.type)) { const i = provUidToIdx[c.providerUid]; return { type: c.type, providerIdx: i === undefined ? null : i }; }
        return { type: c.type, value: c.value || "" };
      });
    },
    // refToPref encodes an API PolicyRef ({kind,inboundId,groupIdx}) into the select
    // string; a group ref is stored by the referenced group's stable uid.
    refToPref(ref, gUid) {
      if (!ref) return "direct";
      if (ref.kind === "inbound") return "inbound:" + ref.inboundId;
      if (ref.kind === "group") return "group:" + gUid[ref.groupIdx];
      return ref.kind;
    },
    // prefToRef turns a select string into the JSON PolicyRef the backend expects
    // (a group ref becomes the group's current array index).
    prefToRef(pref, uidToIdx) {
      if (pref.startsWith("inbound:")) return { kind: "inbound", inboundId: Number(pref.slice(8)) };
      if (pref.startsWith("group:")) { const i = uidToIdx[+pref.slice(6)]; return { kind: "group", groupIdx: i === undefined ? null : i }; }
      return { kind: pref };
    },

    // ---- config: editing -------------------------------------------------------
    addGroup() { this.cfg.groups.push({ _uid: this.uid(), name: "", type: "select", url: "", interval: 0, tolerance: 0, lazy: false, members: [{ _uid: this.uid(), pref: "direct" }] }); },
    delGroup(i) { this.cfg.groups.splice(i, 1); },
    addMember(g) { g.members.push({ _uid: this.uid(), pref: "direct" }); },
    delMember(g, i) { g.members.splice(i, 1); },
    addRule() { this.cfg.rules.push({ _uid: this.uid(), type: "DOMAIN-SUFFIX", value: "", providerUid: null, noResolve: false, pref: "direct", conditions: [] }); },
    delRule(i) { this.cfg.rules.splice(i, 1); },
    // addCondition appends a fresh leaf sub-condition to a logical rule (or condition).
    addCondition(node) { node.conditions.push(newCondition()); },
    addProvider() { this.cfg.providers.push({ _uid: this.uid(), name: "", behavior: "domain", format: "mrs", url: "", interval: 86400, mirror: false, mirrorInterval: 86400 }); this.openProvider(this.cfg.providers.length - 1); },
    openProvider(i) { this.provForm = { open: true, idx: i }; },
    // checkProvider probes the provider URL via the backend (reachable / file present /
    // right format) and toasts the outcome. Saves nothing; a per-row _checking flag
    // drives the inline spinner.
    async checkProvider(p) {
      if (!p.url) { this.toast(false, "Сначала укажите URL у провайдера"); return; }
      p._checking = true;
      try {
        const r = await fetch("/admin/api/config/mihomo/provider/check", {
          method: "POST",
          headers: { "Content-Type": "application/json", Accept: "application/json" },
          body: JSON.stringify({ url: p.url, format: p.format }),
        });
        if (r.status === 401 || r.status === 403) { location.assign("/admin/login"); return; }
        const d = await r.json().catch(() => ({}));
        this.toast(r.ok, r.ok ? (d.message || "OK") : (d.errMessage || "Ошибка проверки"));
      } catch (e) { this.toast(false, "Сеть: " + e); }
      finally { p._checking = false; }
    },
    reorder(arr, oldIndex, newIndex) { const [m] = arr.splice(oldIndex, 1); arr.splice(newIndex, 0, m); },

    // schema lookups — the config UI is driven entirely by the backend schema.
    ruleInfo(t) { return (this.schema?.rules?.types || []).find((r) => r.type === t); },
    groupInfo(t) { return (this.schema?.proxyGroup?.types || []).find((g) => g.type === t); },
    isMatch(t) { return !!this.ruleInfo(t)?.isMatch; },
    isRuleSet(t) { return !!this.ruleInfo(t)?.takesProvider; },
    isLogical(t) { return !!this.ruleInfo(t)?.isLogical; },
    supportsNoResolve(t) { return !!this.ruleInfo(t)?.supportsNoResolve; },
    groupHealthCheck(t) { return !!this.groupInfo(t)?.usesHealthCheck; },
    groupTolerance(t) { return !!this.groupInfo(t)?.usesTolerance; },
    // which reference categories a rule target / group member may point at.
    ruleDestinations(t) { return this.ruleInfo(t)?.destinations || []; },
    groupItems(t) { return this.groupInfo(t)?.items || []; },

    async saveConfig() {
      const uidToIdx = {};
      this.cfg.groups.forEach((g, i) => { uidToIdx[g._uid] = i; });
      const provUidToIdx = {};
      this.cfg.providers.forEach((p, i) => { provUidToIdx[p._uid] = i; });

      const groups = this.cfg.groups.map((g) => {
        const grp = {
          name: g.name, type: g.type, url: g.url || "",
          members: g.members.map((m) => this.prefToRef(m.pref, uidToIdx)),
        };
        // interval/lazy only for health-check types; tolerance only for url-test —
        // omit otherwise so the backend stores NULL (not an inapplicable zero).
        if (this.groupHealthCheck(g.type)) {
          grp.interval = g.interval || 0;
          grp.lazy = !!g.lazy;
        }
        if (this.groupTolerance(g.type)) {
          grp.tolerance = g.tolerance || 0;
        }
        return grp;
      });

      const rules = this.cfg.rules.map((r) => {
        const rule = {
          type: r.type,
          target: this.prefToRef(r.pref, uidToIdx),
        };
        // Logical rule (AND/OR/NOT): the payload is the sub-condition tree, no
        // value/provider/no-resolve.
        if (this.isLogical(r.type)) {
          rule.conditions = this.dumpConditions(r.conditions, provUidToIdx);
          return rule;
        }
        // value only for value-taking types (omitted for MATCH and RULE-SET).
        if (!this.isMatch(r.type) && !this.isRuleSet(r.type)) {
          rule.value = r.value || "";
        }
        // no-resolve only sent for supporting types when actually on (omitted = off).
        if (!this.isMatch(r.type) && this.supportsNoResolve(r.type) && r.noResolve) {
          rule.noResolve = true;
        }
        if (this.isRuleSet(r.type)) {
          const idx = provUidToIdx[r.providerUid];
          rule.providerIdx = idx === undefined ? null : idx;
        }
        return rule;
      });

      const providers = this.cfg.providers.map((p) => ({
        name: p.name, behavior: p.behavior || "", format: p.format || "",
        url: p.url || "", interval: p.interval || 0,
        mirror: !!p.mirror, mirrorInterval: p.mirror ? (p.mirrorInterval || 0) : 0,
      }));

      const payload = {
        baseYAML: this.cfg.baseYAML, groups, rules, providers,
        profileTitle: this.cfg.profileTitle || "",
        filename: this.cfg.filename || "",
        profileUpdateInterval: Number(this.cfg.profileUpdateInterval) || 0,
      };
      if (this.cfgScope.kind === "user") payload.userId = this.cfgScope.userId;
      await this.post("/admin/api/config/mihomo/save", payload);
    },

    // ---- http / ui -------------------------------------------------------------
    async getJSON(url) {
      const r = await fetch(url, { headers: { Accept: "application/json" } });
      if (r.status === 401 || r.status === 403) { location.assign("/admin/login"); throw new Error("auth"); }
      if (!r.ok) throw new Error("HTTP " + r.status);
      return r.json();
    },
    // post sends a JSON mutation and normalises the API's idiomatic contract — 2xx with
    // {message}, 4xx with {errMessage}, or 204 (no body) — into a {ok, msg, data} shape
    // the callers use. A 401/403 bounces to the login page.
    async post(url, body) {
      this.busy = true;
      try {
        const r = await fetch(url, {
          method: "POST",
          headers: { "Content-Type": "application/json", Accept: "application/json" },
          body: JSON.stringify(body),
        });
        if (r.status === 401 || r.status === 403) { location.assign("/admin/login"); return { ok: false }; }
        const d = r.status === 204 ? {} : await r.json().catch(() => ({}));
        const ok = r.ok;
        const msg = ok ? (d.message || "Готово") : (d.errMessage || "Ошибка");
        this.toast(ok, msg);
        return { ok, msg, data: d };
      } catch (e) { this.toast(false, "Сеть: " + e); return { ok: false }; }
      finally { this.busy = false; }
    },
    // logout clears the session (POST), then navigates to the login page itself.
    async logout() {
      try { await fetch("/admin/api/logout", { method: "POST", headers: { Accept: "application/json" } }); } catch (_) { /* navigate regardless */ }
      location.assign("/admin/login");
    },
    toast(ok, msg) {
      const id = ++this._tid;
      this.toasts.push({ id, ok, msg });
      setTimeout(() => { this.toasts = this.toasts.filter((t) => t.id !== id); }, ok ? 2800 : 6000);
    },
    copy(text) {
      navigator.clipboard.writeText(text).then(
        () => this.toast(true, "Скопировано"),
        () => this.toast(false, "Не удалось скопировать"),
      );
    },
    hsize(b) {
      b = b || 0;
      if (b < 1024) return b + " B";
      const u = ["KB", "MB", "GB", "TB", "PB"];
      let i = -1;
      do { b /= 1024; i++; } while (b >= 1024 && i < u.length - 1);
      return b.toFixed(1) + " " + u[i];
    },

    // ---- users --------------------------------------------------------------
    openCreateUser() { this.uForm = { open: true, id: 0, name: "", description: "", inbounds: [] }; },
    openEdit(u) {
      this.uForm = { open: true, id: u.id, name: u.name, description: u.description || "", inbounds: (u.inbounds || []).map((i) => i.id) };
    },
    async submitUser() {
      const f = this.uForm;
      const url = f.id ? "/admin/api/users/edit" : "/admin/api/users/create";
      const params = { inboundIDs: f.inbounds, description: f.description };
      if (f.id) params.id = f.id; else params.name = f.name;
      const d = await this.post(url, params);
      if (d.ok) { this.uForm.open = false; this.loadUsers(); }
    },
    async deleteUser(u) {
      if (!confirm("Удалить " + u.name + "?")) return;
      this.actingId = u.id;
      try { const d = await this.post("/admin/api/users/delete", { id: u.id }); if (d.ok) await this.loadUsers(); }
      finally { this.actingId = 0; }
    },
    async recreateUser(u) {
      this.actingId = u.id;
      try { const d = await this.post("/admin/api/users/recreate", { id: u.id }); if (d.ok) await this.loadUsers(); }
      finally { this.actingId = 0; }
    },

    // ---- nodes --------------------------------------------------------------
    openCreateNode() { this.nodeForm = { open: true, id: 0, name: "", vpnHost: "", panelBaseURL: "", panelBasePath: "", token: "", inbounds: [{ id: 0, name: "", port: "" }] }; },
    openNode(n) {
      this.nodeForm = {
        open: true, id: n.id, name: n.name, vpnHost: n.vpnHost, panelBaseURL: n.panelBaseURL,
        panelBasePath: n.panelBasePath, token: "",
        inbounds: (n.inbounds || []).map((i) => ({ id: i.id, name: i.name, port: i.port })),
      };
    },
    addInbound() { this.nodeForm.inbounds.push({ id: 0, name: "", port: "" }); },
    async saveNode() {
      const f = this.nodeForm;
      const inbounds = f.inbounds
        .filter((inb) => inb.port)
        .map((inb) => ({ id: inb.id || 0, name: inb.name, port: Number(inb.port) }));
      const d = await this.post("/admin/api/nodes/save", {
        id: f.id || 0, name: f.name, vpnHost: f.vpnHost, panelBaseURL: f.panelBaseURL,
        panelBasePath: f.panelBasePath, token: f.token, inbounds,
      });
      if (d.ok) { this.nodeForm.open = false; this.load("nodes"); }
    },
    async deleteNode(n) {
      if (!confirm("Удалить узел " + n.name + "?")) return;
      const d = await this.post("/admin/api/nodes/delete", { id: n.id });
      if (d.ok) this.load("nodes");
    },

    closeModals() { this.uForm.open = false; this.nodeForm.open = false; this.inboundFilterOpen = false; },
  },
  mounted() {
    this.load("users");
    window.addEventListener("keydown", (e) => { if (e.key === "Escape") this.closeModals(); });
  },
});

// v-sortable: drag-to-reorder a list via SortableJS. Pass {handle, item, end} where
// end(oldIndex, newIndex) reorders the backing array. Stable :key per item keeps Vue
// and Sortable in sync (Sortable moves the DOM, the array splice mirrors it).
app.directive("sortable", {
  mounted(el, binding) {
    const o = binding.value || {};
    el._sortable = Sortable.create(el, {
      handle: o.handle || ".drag", draggable: o.item || ".s-item", animation: 150,
      onEnd(e) { if (e.oldIndex !== e.newIndex && o.end) o.end(e.oldIndex, e.newIndex); },
    });
  },
  unmounted(el) { if (el._sortable) el._sortable.destroy(); },
});

// policy-picker: a <select> choosing a routing target / group member. It renders ONLY
// the reference categories the schema allows for this context (`allowed`): actions
// (built-in policies), inbounds (every fleet inbound), and other groups — nothing about
// the taxonomy is hardcoded here. v-model is the encoded `pref` string
// (<kind> / inbound:<id> / group:<uid>).
app.component("policy-picker", {
  props: { modelValue: String, allowed: Array, actions: Array, inbounds: Array, groups: Array },
  emits: ["update:modelValue"],
  methods: { has(cat) { return (this.allowed || []).includes(cat); } },
  template: `
    <select class="form-select form-select-sm" :value="modelValue" @change="$emit('update:modelValue', $event.target.value)">
      <optgroup label="Действия" v-if="has('actions')">
        <option v-for="a in actions" :key="a.kind" :value="a.kind">{{ a.label }}</option>
      </optgroup>
      <optgroup label="Инбаунды" v-if="has('inbounds')">
        <option v-for="f in inbounds" :key="f.id" :value="'inbound:'+f.id">{{ f.label }}</option>
      </optgroup>
      <optgroup label="Группы" v-if="has('groups') && groups.length">
        <option v-for="(g,i) in groups" :key="g._uid" :value="'group:'+g._uid">{{ g.name || ('группа '+(i+1)) }}</option>
      </optgroup>
    </select>`,
});

// cuid is the stable-key generator for condition-tree nodes (separate counter from the
// root's uid(); only needs to be unique among siblings for Vue's :key). newCondition is
// the fresh leaf factory used by "add condition" at any depth.
let _cuidc = 0;
const cuid = () => "c" + (++_cuidc);
function newCondition() { return { _uid: cuid(), type: "DOMAIN-SUFFIX", value: "", providerUid: null, conditions: [] }; }

// condition-node: one matcher inside a logical rule (AND/OR/NOT), rendered recursively.
// A leaf shows a type select + a value input (or a provider select for RULE-SET); a
// logical node shows its children (each another condition-node) with add/remove and any
// depth of nesting. Sub-conditions have no target and no no-resolve — they are matchers,
// not full rules. Reordering is omitted on purpose: AND/OR/NOT are commutative, so the
// order of conditions has no effect on matching. The type list comes from the schema
// (MATCH excluded — it cannot be a condition); nothing about the taxonomy is hardcoded.
app.component("condition-node", {
  name: "condition-node",
  props: { node: Object, schema: Object, providers: Array },
  emits: ["remove"],
  computed: {
    types() { return (this.schema?.rules?.types || []).filter((t) => !t.isMatch); },
  },
  methods: {
    info(t) { return (this.schema?.rules?.types || []).find((r) => r.type === t); },
    isLogical(t) { return !!this.info(t)?.isLogical; },
    isRuleSet(t) { return !!this.info(t)?.takesProvider; },
    addChild() { this.node.conditions.push(newCondition()); },
    delChild(i) { this.node.conditions.splice(i, 1); },
  },
  template: `
    <div class="cond-node">
      <div class="cond-row">
        <select class="form-select form-select-sm" style="max-width:200px" v-model="node.type">
          <option v-for="t in types" :key="t.type" :value="t.type">{{ t.type }}</option>
        </select>
        <select v-if="isRuleSet(node.type)" class="form-select form-select-sm grow" v-model="node.providerUid">
          <option :value="null" disabled>— провайдер —</option>
          <option v-for="(p,pi) in providers" :key="p._uid" :value="p._uid">{{ p.name || ('провайдер '+(pi+1)) }}</option>
        </select>
        <input v-else-if="!isLogical(node.type)" class="form-control form-control-sm grow" v-model="node.value" placeholder="значение">
        <span v-else class="grow text-dim small">вложенные условия</span>
        <button class="btn btn-sm btn-danger-soft act" @click="$emit('remove')" title="удалить условие">✕</button>
      </div>
      <div v-if="isLogical(node.type)" class="cond-children">
        <condition-node v-for="(c,ci) in node.conditions" :key="c._uid" :node="c" :schema="schema" :providers="providers" @remove="delChild(ci)"></condition-node>
        <button class="btn btn-sm btn-outline-secondary mt-1" @click="addChild()">Добавить условие</button>
      </div>
    </div>`,
});

// duration-input: a human-friendly TTL field over a seconds integer. Renders a number
// + a unit (sec/min/hour/day); v-model is always seconds. On load it decomposes the
// seconds into the largest exact unit; a _self guard skips the echo so typing in one
// unit isn't snapped to another.
app.component("duration-input", {
  props: { modelValue: { type: Number, default: 0 } },
  emits: ["update:modelValue"],
  data() { return { num: 0, unit: 1, _self: null }; },
  watch: { modelValue: { immediate: true, handler(v) { if (v !== this._self) this.sync(v || 0); } } },
  methods: {
    sync(secs) {
      let u = 1;
      for (const k of [86400, 3600, 60]) { if (secs !== 0 && secs % k === 0) { u = k; break; } }
      this.unit = u;
      this.num = secs / u;
    },
    emit() {
      const v = Math.max(0, Math.round(this.num || 0)) * this.unit;
      this._self = v;
      this.$emit("update:modelValue", v);
    },
  },
  template: `
    <div class="dur-input">
      <input class="form-control" type="number" min="0" v-model.number="num" @input="emit">
      <select class="form-select" v-model.number="unit" @change="emit">
        <option :value="1">сек</option>
        <option :value="60">мин</option>
        <option :value="3600">час</option>
        <option :value="86400">дн</option>
      </select>
    </div>`,
});

// Reusable modal: backdrop + card with a header/body/footer slot, fade+lift
// transition, click-outside / ✕ to close (the parent owns the `open` flag).
app.component("modal", {
  props: { open: Boolean, title: String, lg: Boolean },
  emits: ["close"],
  template: `
    <transition name="modal">
      <div v-if="open" class="modal-backdrop-custom" @click.self="$emit('close')">
        <div class="modal-card" :class="{lg}" role="dialog" aria-modal="true">
          <div class="modal-head">
            <h5>{{ title }}</h5>
            <button class="icon-btn" @click="$emit('close')" aria-label="Закрыть">✕</button>
          </div>
          <div class="modal-body"><slot></slot></div>
          <div class="modal-foot"><slot name="footer"></slot></div>
        </div>
      </div>
    </transition>`,
});

// loadMonaco lazily boots the Monaco editor from the CDN via its AMD loader (already
// on the page) and resolves the global `monaco`. Memoised so every yaml-editor instance
// shares one load. Workers are shimmed through a data: URL that re-points Monaco's base
// at the CDN (the usual cross-origin self-hosting dance).
const MONACO_CDN = "https://cdn.jsdelivr.net/npm/monaco-editor@0.52.2/min";
let _monacoPromise = null;
function loadMonaco() {
  if (_monacoPromise) return _monacoPromise;
  _monacoPromise = new Promise((resolve, reject) => {
    window.MonacoEnvironment = {
      getWorkerUrl() {
        return "data:text/javascript;charset=utf-8," + encodeURIComponent(
          "self.MonacoEnvironment={baseUrl:'" + MONACO_CDN + "/'};" +
          "importScripts('" + MONACO_CDN + "/vs/base/worker/workerMain.js');",
        );
      },
    };
    window.require.config({ paths: { vs: MONACO_CDN + "/vs" } });
    window.require(["vs/editor/editor.main"], () => resolve(window.monaco), reject);
  });
  return _monacoPromise;
}

// defineSubgenTheme registers (once) a dark Monaco theme tuned to the subgen palette.
function defineSubgenTheme(monaco) {
  if (monaco.editor.__subgenTheme) return;
  monaco.editor.__subgenTheme = true;
  monaco.editor.defineTheme("subgen-dark", {
    base: "vs-dark", inherit: true,
    rules: [
      { token: "comment", foreground: "5b6473", fontStyle: "italic" },
      { token: "type", foreground: "7fcdf0" },     // mapping keys
      { token: "string", foreground: "9ece6a" },
      { token: "string.yaml", foreground: "9ece6a" },
      { token: "number", foreground: "f7b955" },
      { token: "keyword", foreground: "bb9af7" },   // true/false/null
      { token: "tag", foreground: "f7768e" },       // anchors / tags
      { token: "operators", foreground: "737f8d" },
    ],
    colors: {
      "editor.background": "#23252b",
      "editor.foreground": "#d9d9d9",
      "editorLineNumber.foreground": "#5a5c63",
      "editorLineNumber.activeForeground": "#d9d9d9",
      "editor.lineHighlightBackground": "#ffffff08",
      "editor.lineHighlightBorder": "#00000000",
      "editorCursor.foreground": "#3c89e8",
      "editor.selectionBackground": "#1668dc55",
      "editor.inactiveSelectionBackground": "#1668dc30",
      "editorIndentGuide.background1": "#34363d",
      "editorIndentGuide.activeBackground1": "#45474f",
      "editorBracketMatch.background": "#1668dc33",
      "editorBracketMatch.border": "#1668dc80",
      "scrollbarSlider.background": "#ffffff20",
      "scrollbarSlider.hoverBackground": "#ffffff30",
      "scrollbarSlider.activeBackground": "#ffffff40",
      "editorError.foreground": "#dc4446",
    },
  });
}

// yaml-editor: the base-YAML field, backed by Monaco. Built-in YAML colorisation,
// current-line highlight and 2-space indentation; live validation runs js-yaml on a
// debounce and feeds error markers (squiggle + hover) plus a status line. v-model is
// the text. Monaco loads from the CDN on mount and is disposed on unmount.
app.component("yaml-editor", {
  props: { modelValue: { type: String, default: "" } },
  emits: ["update:modelValue"],
  data() { return { err: null, ready: false, _t: 0 }; },
  async mounted() {
    const monaco = await loadMonaco();
    if (this._gone) return; // unmounted while the CDN was loading
    this._monaco = monaco;
    defineSubgenTheme(monaco);
    this._ed = monaco.editor.create(this.$refs.host, {
      value: this.modelValue || "",
      language: "yaml",
      theme: "subgen-dark",
      automaticLayout: true,
      minimap: { enabled: false },
      tabSize: 2, insertSpaces: true, detectIndentation: false,
      fontSize: 13, lineHeight: 20,
      fontFamily: "ui-monospace,SFMono-Regular,Consolas,'Liberation Mono',Menlo,monospace",
      renderLineHighlight: "all", scrollBeyondLastLine: false, smoothScrolling: true,
      lineNumbersMinChars: 3, glyphMargin: false, folding: true, wordWrap: "off",
      padding: { top: 8, bottom: 8 }, fixedOverflowWidgets: true,
      scrollbar: { verticalScrollbarSize: 12, horizontalScrollbarSize: 12, useShadows: false },
    });
    this.ready = true;
    this._ed.onDidChangeModelContent(() => {
      const v = this._ed.getValue();
      this.$emit("update:modelValue", v);
      this.schedule(v);
    });
    this.validate(this.modelValue || "");
  },
  beforeUnmount() {
    this._gone = true;
    clearTimeout(this._t);
    if (this._ed) this._ed.dispose();
  },
  watch: {
    // keep Monaco in sync when the model is replaced from outside (e.g. config reload),
    // without clobbering the caret during normal typing (guarded by value equality).
    modelValue(v) { if (this._ed && v !== this._ed.getValue()) this._ed.setValue(v || ""); },
  },
  methods: {
    schedule(v) { clearTimeout(this._t); this._t = setTimeout(() => this.validate(v), 250); },
    validate(v) {
      const monaco = this._monaco, model = this._ed && this._ed.getModel();
      if (!monaco || !model) return;
      const txt = v || "";
      if (!txt.trim()) { this.err = null; monaco.editor.setModelMarkers(model, "yaml", []); return; }
      try {
        jsyaml.load(txt);
        this.err = null;
        monaco.editor.setModelMarkers(model, "yaml", []);
      } catch (ex) {
        const m = ex.mark || {};
        const line = (m.line || 0) + 1, col = (m.column || 0) + 1;
        this.err = { line, col, msg: ex.reason || ex.message };
        monaco.editor.setModelMarkers(model, "yaml", [{
          severity: monaco.MarkerSeverity.Error,
          message: ex.reason || ex.message,
          startLineNumber: line, startColumn: col, endLineNumber: line, endColumn: col + 1,
        }]);
      }
    },
    gotoErr() {
      if (!this.err || !this._ed) return;
      this._ed.revealLineInCenter(this.err.line);
      this._ed.setPosition({ lineNumber: this.err.line, column: this.err.col });
      this._ed.focus();
    },
  },
  template: `
    <div class="ye">
      <div class="ye-mon" ref="host"><span v-if="!ready" class="ye-loading">загрузка редактора…</span></div>
      <div class="ye-status" :class="err ? 'is-err' : ''" @click="gotoErr">
        <template v-if="err"><span class="ye-dot"></span>строка {{ err.line }}:{{ err.col }} — {{ err.msg }}</template>
      </div>
    </div>`,
});

app.mount("#app");
