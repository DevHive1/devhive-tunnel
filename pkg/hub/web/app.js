/* devhive-tunnel dashboard — owner & GM console */
(() => {
"use strict";

/* ---------------- i18n ---------------- */
const I18N = {
  en: {
    console: "secure network console", owner: "Network Owner", gm: "GM Superuser",
    networkNameId: "Network name or ID", adminKey: "Admin key", gmKey: "GM superuser key",
    signIn: "Sign in", logout: "Logout", refresh: "Refresh", live: "live",
    ownerOnlyNote: "Only the network owner has control access",
    footnote: "devhive-tunnel · end-to-end encrypted mesh · dashboard secured with per-network owner keys",
    overview: "Overview", peers: "Peers", invites: "Invites", audit: "Audit", settings: "Settings",
    networks: "Networks", analytics: "Analytics", bans: "Global bans",
    online: "online", total: "total", network: "network", role: "role", 
    onlinePeers: "Online peers", totalPeers: "Total peers", networkId: "Network ID",
    yourIp: "Your virtual IP", created: "created", status: "status",
    nickname: "nickname", fingerprint: "fingerprint", vlanIp: "virtual IP",
    platform: "platform", lastSeen: "last seen", actions: "actions",
    kick: "Kick", ban: "Ban", unban: "Unban", promote: "Make admin", demote: "Remove admin",
    banned: "banned", adminBadge: "admin", onlineBadge: "online", offlineBadge: "offline",
    noPeers: "No peers yet — invite your team", noInvites: "No invites created",
    inviteNote: "note (who is it for?)", usesMax: "max uses (0 = unlimited)",
    expiresDays: "expires in days (0 = never)", inviteRole: "role", create: "Create",
    revoke: "Revoke", copy: "copy", copied: "Copied!",
    requireKey: "Require admin/invite key to join", allowAnon: "List network publicly",
    allowRelay: "Allow relay traffic", maxPeers: "Max peers (0 = unlimited)",
    maxUp: "Max relay Mbps (0 = unlimited)", subnet: "Virtual subnet",
    save: "Save", rotate: "Rotate admin key", rotateWarn: "Old admin key stops working immediately.",
    newAdminKey: "New admin key (save it now!)", settingsSaved: "Settings saved",
    netBlocked: "This network is suspended by the GM.",
    gmOverview: "Hub overview", networksManaged: "Networks", peersOnline: "Peers online",
    activeBans: "Global bans", blockedNets: "Suspended networks", uptime: "uptime",
    version: "version", hubFp: "Hub fingerprint", defaultSubnet: "default subnet",
    joinAttempts: "join attempts", authFails: "auth failures", routedFrames: "routed frames",
    blocked: "suspended", block: "Suspend", unblock: "Reactivate", delete: "Delete",
    blockReason: "reason", confirmBlock: "Suspend this whole network? All members are disconnected.",
    confirmDelete: "DELETE this network permanently? This cannot be undone.",
    confirmKick: "Kick this peer?", confirmBan: "Ban this peer?", confirmRotate: "Rotate admin key?",
    banFp: "Ban fingerprint", unban: "Unban", eventsPerHour: "Events per hour (24h)",
    topEvents: "Top events", networkUsage: "Network usage", noNetworks: "No networks yet",
    selectNetwork: "Select a network", back: "Back", details: "details",
    banUser: "Ban a user", banReason: "reason", cancel: "Cancel", ok: "OK",
    ownerLogin: "Owner sign-in", gmLogin: "GM sign-in", badLogin: "Wrong credentials",
    genBusy: "Working…", networkSuspended: "suspended", owner: "owner",
    changedOk: "Done", error: "Error", confirm: "Confirm", promoteOk: "Admin role updated",
    auditEmpty: "No audit entries", evJoined: "joined", evLeft: "disconnected",
    peersTitle: "Members of", invitesTitle: "Join invites for", auditTitle: "Security audit",
    settingsTitle: "Advanced settings", transfers: "transfers", security: "security",
    analyticsTitle: "Supervision analytics", totalNetworks: "total networks",
    kickAll: "Kick", unbanAll: "Lift ban", gmNetworksTitle: "All hive networks",
    gmBansTitle: "Global bans (all networks)", gmAuditTitle: "Hub & network audit",
    noBans: "No global bans", close: "Close", blockNet: "Suspend network",
    unblockNet: "Reactivate network", rotateAdmin: "New admin key", deleteNet: "Delete network",
    vlanNote: "Virtual LAN IP inside the encrypted network", emptyNet: "No members",
    bannedFp: "fingerprint", add: "Add", note: "note"
  },
  ar: {
    console: "لوحة تحكم الشبكة الآمنة", owner: "مالك الشبكة", gm: "المدير العام GM",
    networkNameId: "اسم الشبكة أو المعرف", adminKey: "مفتاح الإدارة", gmKey: "مفتاح المدير العام",
    signIn: "تسجيل الدخول", logout: "خروج", refresh: "تحديث", live: "مباشر",
    ownerOnlyNote: "مالك الشبكة فقط يملك صلاحية التحكم",
    footnote: "devhive-tunnel · شبكة مشفرة من طرف إلى طرف · لوحة محمية بمفاتيح المالك",
    overview: "نظرة عامة", peers: "الأعضاء", invites: "الدعوات", audit: "السجل", settings: "الإعدادات",
    networks: "الشبكات", analytics: "التحليلات", bans: "الحظر العام",
    online: "متصل", total: "الإجمالي", network: "الشبكة", role: "الدور",
    onlinePeers: "الأعضاء المتصلون", totalPeers: "إجمالي الأعضاء", networkId: "معرف الشبكة",
    yourIp: "عنوانك الافتراضي", created: "أُنشئت", status: "الحالة",
    nickname: "الاسم", fingerprint: "البصمة", vlanIp: "العنوان الافتراضي",
    platform: "الجهاز", lastSeen: "آخر ظهور", actions: "إجراءات",
    kick: "إخراج", ban: "حظر", unban: "رفع الحظر", promote: "ترقية لمشرف", demote: "إزالة الإشراف",
    banned: "محظور", adminBadge: "مشرف", onlineBadge: "متصل", offlineBadge: "غير متصل",
    noPeers: "لا يوجد أعضاء بعد — ادعُ فريقك", noInvites: "لا توجد دعوات",
    inviteNote: "ملاحظة (لمن الدعوة؟)", usesMax: "أقصى استخدامات (0 = بلا حد)",
    expiresDays: "تنتهي بعد أيام (0 = أبدًا)", inviteRole: "الدور", create: "إنشاء",
    revoke: "إلغاء", copy: "نسخ", copied: "تم النسخ!",
    requireKey: "طلب مفتاح إدارة/دعوة للانضمام", allowAnon: "إظهار الشبكة علنًا",
    allowRelay: "السماح بمرور البيانات عبر الخادم", maxPeers: "أقصى عدد أعضاء (0 = بلا حد)",
    maxUp: "أقصى سرعة مرور Mbps (0 = بلا حد)", subnet: "الشبكة الفرعية الافتراضية",
    save: "حفظ", rotate: "تغيير مفتاح الإدارة", rotateWarn: "المفتاح القديم يتوقف فورًا.",
    newAdminKey: "مفتاح الإدارة الجديد (احفظه الآن!)", settingsSaved: "تم حفظ الإعدادات",
    netBlocked: "هذه الشبكة موقوفة من المدير العام.",
    gmOverview: "نظرة عامة على الخادم", networksManaged: "الشبكات", peersOnline: "الأعضاء المتصلون",
    activeBans: "الحظر العام", blockedNets: "شبكات موقوفة", uptime: "مدة التشغيل",
    version: "الإصدار", hubFp: "بصمة الخادم", defaultSubnet: "الشبكة الفرعية الافتراضية",
    joinAttempts: "محاولات الانضمام", authFails: "فشل المصادقة", routedFrames: "إطارات مُمرّرة",
    blocked: "موقوفة", block: "إيقاف", unblock: "إعادة تشغيل", delete: "حذف",
    blockReason: "السبب", confirmBlock: "إيقاف الشبكة بالكامل؟ سيتم فصل كل الأعضاء.",
    confirmDelete: "حذف الشبكة نهائيًا؟ لا يمكن التراجع.",
    confirmKick: "إخراج هذا العضو؟", confirmBan: "حظر هذا العضو؟", confirmRotate: "تغيير مفتاح الإدارة؟",
    banFp: "حظر بصمة", unban: "رفع الحظر", eventsPerHour: "الأحداث في الساعة (24 ساعة)",
    topEvents: "أكثر الأحداث", networkUsage: "استخدام الشبكات", noNetworks: "لا توجد شبكات بعد",
    selectNetwork: "اختر شبكة", back: "رجوع", details: "التفاصيل",
    banUser: "حظر مستخدم", banReason: "السبب", cancel: "إلغاء", ok: "موافق",
    ownerLogin: "دخول المالك", gmLogin: "دخول المدير العام", badLogin: "بيانات غير صحيحة",
    genBusy: "جارٍ التنفيذ…", networkSuspended: "موقوفة", owner: "المالك",
    changedOk: "تم", error: "خطأ", confirm: "تأكيد", promoteOk: "تم تحديث دور المشرف",
    auditEmpty: "لا توجد سجلات", evJoined: "انضم", evLeft: "انقطع",
    peersTitle: "أعضاء", invitesTitle: "دعوات الانضمام لشبكة", auditTitle: "سجل الأمان",
    settingsTitle: "إعدادات متقدمة", transfers: "النقل", security: "الأمان",
    analyticsTitle: "تحليلات الإشراف", totalNetworks: "إجمالي الشبكات",
    kickAll: "إخراج", unbanAll: "رفع الحظر", gmNetworksTitle: "كل شبكات الخلية",
    gmBansTitle: "الحظر العام (كل الشبكات)", gmAuditTitle: "سجل الخادم والشبكات",
    noBans: "لا يوجد حظر عام", close: "إغلاق", blockNet: "إيقاف الشبكة",
    unblockNet: "إعادة تشغيل الشبكة", rotateAdmin: "مفتاح إدارة جديد", deleteNet: "حذف الشبكة",
    vlanNote: "عنوان الشبكة المحلية داخل الشبكة المشفرة", emptyNet: "لا يوجد أعضاء",
    bannedFp: "البصمة", add: "إضافة", note: "ملاحظة"
  }
};
let lang = localStorage.getItem("dh_lang") || "en";
const T = k => (I18N[lang] && I18N[lang][k]) || I18N.en[k] || k;

function applyI18n() {
  document.querySelectorAll("[data-i18n]").forEach(el => { el.textContent = T(el.dataset.i18n); });
  const rtl = lang === "ar";
  document.documentElement.lang = lang;
  document.documentElement.dir = rtl ? "rtl" : "ltr";
  document.body.dir = rtl ? "rtl" : "ltr";
  [el("langBtn"), el("langBtn2")].forEach(b => { if (b) b.textContent = rtl ? "English" : "عربي"; });
}
function setLang(l) {
  lang = l; localStorage.setItem("dh_lang", l); applyI18n(); renderNav(); route();
}

/* ---------------- state & api ---------------- */
const S = {
  token: sessionStorage.getItem("dh_token") || "",
  kind: sessionStorage.getItem("dh_kind") || "",     // owner | gm
  netId: sessionStorage.getItem("dh_net") || "",
  netName: sessionStorage.getItem("dh_netname") || "",
  tab: "overview", detail: "", currentNet: null, timer: null
};
const el = id => document.getElementById(id);
const esc = s => String(s == null ? "" : s).replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));

async function api(path, opts = {}) {
  opts.headers = Object.assign({ "Content-Type": "application/json" }, opts.headers || {});
  if (S.token) opts.headers.Authorization = "Bearer " + S.token;
  const r = await fetch(path, opts);
  let data = null;
  try { data = await r.json(); } catch (_) {}
  if (r.status === 401 && path !== "/api/admin/login" && path !== "/api/gm/login") {
    doLogout(); throw new Error("unauthorized");
  }
  if (!r.ok) throw new Error((data && (data.error || data.message)) || ("HTTP " + r.status));
  return data;
}

function toast(msg, cls = "") {
  const d = document.createElement("div");
  d.className = "toast " + cls; d.textContent = msg;
  el("toasts").appendChild(d);
  setTimeout(() => d.remove(), 4200);
}
const fmtTime = ts => { if (!ts) return "—"; const d = new Date(ts); return isNaN(d) ? ts : d.toLocaleString(lang === "ar" ? "ar" : "en-GB", { dateStyle: "short", timeStyle: "short" }); };
const uptime = s => { s = Number(s || 0); const d = Math.floor(s / 86400), h = Math.floor(s % 86400 / 3600), m = Math.floor(s % 3600 / 60); return (d ? d + "d " : "") + h + "h " + m + "m"; };

/* ---------------- login ---------------- */
let loginTab = "owner";
el("tabOwner").addEventListener("click", () => setLoginTab("owner"));
el("tabGM").addEventListener("click", () => setLoginTab("gm"));
function setLoginTab(t) {
  loginTab = t;
  el("tabOwner").classList.toggle("on", t === "owner");
  el("tabGM").classList.toggle("on", t === "gm");
  el("ownerForm").classList.toggle("hidden", t !== "owner");
  el("gmForm").classList.toggle("hidden", t === "owner");
  el("loginErr").textContent = "";
}
el("ownerGo").addEventListener("click", doOwnerLogin);
el("gmGo").addEventListener("click", doGMLogin);
el("ownerKey").addEventListener("keydown", e => { if (e.key === "Enter") doOwnerLogin(); });
el("gmKey").addEventListener("keydown", e => { if (e.key === "Enter") doGMLogin(); });

async function doOwnerLogin() {
  el("loginErr").textContent = "";
  try {
    const d = await api("/api/admin/login", { method: "POST", body: JSON.stringify({ network: el("ownerNet").value.trim(), admin_key: el("ownerKey").value }) });
    S.token = d.token; S.kind = "owner"; S.netId = d.network_id; S.netName = d.network_name;
    sessionStorage.setItem("dh_token", S.token); sessionStorage.setItem("dh_kind", S.kind);
    sessionStorage.setItem("dh_net", S.netId); sessionStorage.setItem("dh_netname", S.netName);
    enterApp();
  } catch (e) { el("loginErr").textContent = T("badLogin"); }
}
async function doGMLogin() {
  el("loginErr").textContent = "";
  try {
    const d = await api("/api/gm/login", { method: "POST", body: JSON.stringify({ gm_key: el("gmKey").value }) });
    S.token = d.token; S.kind = "gm"; S.netId = ""; S.netName = "";
    sessionStorage.setItem("dh_token", S.token); sessionStorage.setItem("dh_kind", S.kind);
    sessionStorage.removeItem("dh_net"); sessionStorage.removeItem("dh_netname");
    enterApp();
  } catch (e) { el("loginErr").textContent = T("badLogin"); }
}
function doLogout() {
  stopLive();
  if (S.token) fetch("/api/logout", { method: "POST", headers: { Authorization: "Bearer " + S.token } }).catch(() => {});
  sessionStorage.clear();
  S.token = ""; S.kind = ""; S.netId = "";
  clearInterval(S.timer);
  el("app").classList.add("hidden"); el("login").classList.remove("hidden");
  el("ownerKey").value = ""; el("gmKey").value = "";
}
el("logoutBtn").addEventListener("click", doLogout);
el("langBtn").addEventListener("click", () => setLang(lang === "ar" ? "en" : "ar"));
el("langBtn2").addEventListener("click", () => setLang(lang === "ar" ? "en" : "ar"));
el("menuBtn").addEventListener("click", () => el("side").classList.toggle("open"));
el("refreshBtn").addEventListener("click", () => route());

/* ---------------- shell ---------------- */
function enterApp() {
  el("login").classList.add("hidden");
  el("app").classList.remove("hidden");
  el("sideSub").textContent = S.kind === "gm" ? "GM console" : (S.netName || "network");
  el("roleBadge").textContent = S.kind === "gm" ? "GM" : "OWNER";
  el("whoChip").textContent = S.kind === "gm" ? "hub superuser" : (S.netId || "");
  renderNav(); route();
  startLive();
  clearInterval(S.timer);
  const every = S.kind === "gm" ? 30000 : 6000;
  S.timer = setInterval(() => { if (!document.hidden) route(true); }, every);
}

/* ---------------- gm live stream (SSE) ---------------- */
let liveES = null, liveBusy = false, liveLast = 0;
function stopLive() {
  if (liveES) { liveES.close(); liveES = null; }
}
function liveRefresh() {
  if (document.hidden || liveBusy) return;
  const ae = document.activeElement;
  if (ae && /^(INPUT|TEXTAREA|SELECT)$/.test(ae.tagName)) return;
  if (Date.now() - liveLast < 1500) return;
  liveBusy = true; liveLast = Date.now();
  Promise.resolve(route(true)).finally(() => { liveBusy = false; });
}
function startLive() {
  stopLive();
  if (S.kind !== "gm" || !S.token) return;
  try { liveES = new EventSource("/api/gm/live?token=" + encodeURIComponent(S.token)); }
  catch (e) { liveES = null; return; }
  liveES.onopen = () => { el("liveDot").style.background = "var(--ok)"; };
  liveES.onmessage = ev => {
    let snap;
    try { snap = JSON.parse(ev.data); } catch (e) { return; }
    S.live = snap;
    el("liveDot").style.background = "var(--ok)";
    liveRefresh();
  };
  liveES.onerror = () => { el("liveDot").style.background = "var(--bad)"; };
}

const NAV_OWNER = [
  ["overview", "◎"], ["peers", "👥"], ["invites", "✉"], ["audit", "🗎"], ["settings", "⚙"]
];
const NAV_GM = [
  ["overview", "◎"], ["networks", "🌐"], ["analytics", "📈"], ["bans", "⛔"], ["audit", "🗎"]
];
let currentNav = "overview";

function renderNav() {
  const nav = S.kind === "gm" ? NAV_GM : NAV_OWNER;
  el("nav").innerHTML = nav.map(([k, ico]) =>
    `<button class="nav-btn ${S.tab === k ? "on" : ""}" data-nav="${k}"><span class="ico">${ico}</span>${esc(T(k))}</button>`
  ).join("");
}
document.addEventListener("click", e => {
  const nav = e.target.closest("[data-nav]");
  if (nav) {
    S.tab = nav.dataset.nav; S.detail = ""; renderNav(); route();
    el("side").classList.remove("open");
  }
});
el("nav").addEventListener("click", () => {}); // delegation above

function setTitle(t) { el("pageTitle").textContent = t; }

const PAGES = {};
async function route(silent) {
  if (!S.token) return;
  const fn = PAGES[S.kind === "gm" ? "gm_" + S.tab : "owner_" + S.tab] || PAGES.fallback;
  try { await fn(!!silent); el("liveDot").style.background = "var(--ok)"; }
  catch (err) {
    if (String(err.message) !== "unauthorized") {
      el("liveDot").style.background = "var(--bad)";
      if (!silent) toast(T("error") + ": " + err.message, "bad");
    }
  }
}

/* ---------------- owner pages ---------------- */
PAGES.owner_overview = async () => {
  setTitle(T("overview"));
  const n = await api("/api/net");
  S.currentNet = n;
  const st = n.blocked ? `<div class="card" style="border-color:rgba(255,93,108,.4);color:var(--bad);margin-bottom:14px">⛔ ${esc(T("netBlocked"))} ${n.blocked && n.block_reason ? "(" + esc(n.block_reason) + ")" : ""}</div>` : "";
  el("view").innerHTML = st + `
  <div class="grid c4">
    ${stat(T("onlinePeers"), n.online, "ok")}
    ${stat(T("totalPeers"), n.total)}
    ${stat(T("networkId"), `<span class="mono">${esc(n.id)}</span>`, "", true)}
    ${stat(T("created"), fmtTime(n.created), "", true)}
  </div>
  <div class="section-title">${esc(T("network"))}<span class="line"></span></div>
  <div class="grid c2">
    <div class="card"><div class="k">${esc(T("settings"))}</div>
      <div class="muted" style="font-size:.83rem;line-height:1.9">
      ${esc(T("subnet"))}: <span class="mono">${esc(n.settings.subnet_cidr || "—")}</span><br>
      ${esc(T("requireKey"))}: ${n.settings.require_invite_key ? "✅" : "—"}<br>
      ${esc(T("allowAnon"))}: ${n.settings.allow_anonymous_discovery ? "✅" : "—"}<br>
      ${esc(T("maxPeers"))}: ${n.settings.max_peers || "∞"}
      </div>
    </div>
    <div class="card"><div class="k">${esc(T("peers"))} — ${esc(T("status"))}</div>
      ${onlineBar(n.peers)}
    </div>
  </div>`;
};
function stat(k, v, cls, small) {
  return `<div class="card stat"><div class="k">${esc(k)}</div><div class="v ${cls || ""}" style="${small ? "font-size:1rem" : ""}">${v}</div></div>`;
}
function onlineBar(peers) {
  const on = peers.filter(p => p.online).length, off = peers.length - on;
  const pct = peers.length ? Math.round(on * 100 / peers.length) : 0;
  return `<div style="margin-top:10px">
    <div style="height:10px;background:var(--bg2);border-radius:99px;overflow:hidden;display:flex">
      <div style="width:${pct}%;background:linear-gradient(90deg,var(--ok),#2aa869)"></div>
      <div style="flex:1;background:rgba(139,152,184,.18)"></div>
    </div>
    <div class="muted" style="font-size:.78rem;margin-top:8px">${on} ${esc(T("online"))} · ${off} ${esc(T("offlineBadge"))} · ${peers.length} ${esc(T("total"))}</div>
  </div>`;
}

PAGES.owner_peers = async () => {
  setTitle(T("peers"));
  const n = await api("/api/net");
  S.currentNet = n;
  const rows = n.peers.map(p => `
    <tr>
      <td><strong>${esc(p.nickname)}</strong> ${p.admin ? `<span class="pill admin">${esc(T("adminBadge"))}</span>` : ""}</td>
      <td><span class="pill ${p.online ? "on" : "off"}">${p.online ? esc(T("onlineBadge")) : esc(T("offlineBadge"))}</span></td>
      <td class="mono">${esc(p.vlan_ip || "—")}</td>
      <td class="mono copy" data-copy="${esc(p.fingerprint)}" title="${esc(p.fingerprint)}">${esc(p.fingerprint)}</td>
      <td>${esc(p.platform || "—")}</td>
      <td class="muted">${fmtTime(p.last_seen)}</td>
      <td><div class="actions">
        ${p.banned
          ? `<button class="btn small" data-act="unban" data-fp="${esc(p.fingerprint)}">${esc(T("unban"))}</button>`
          : `<button class="btn small" data-act="ban" data-fp="${esc(p.fingerprint)}">${esc(T("ban"))}</button>`}
        <button class="btn small" data-act="kick" data-fp="${esc(p.fingerprint)}">${esc(T("kick"))}</button>
        ${p.admin
          ? `<button class="btn small ghost" data-act="demote" data-fp="${esc(p.fingerprint)}">${esc(T("demote"))}</button>`
          : `<button class="btn small ghost" data-act="promote" data-fp="${esc(p.fingerprint)}">${esc(T("promote"))}</button>`}
      </div></td>
    </tr>`).join("");
  el("view").innerHTML = `
  <div class="card"><div class="table-wrap">
  <table><thead><tr>
    <th>${esc(T("nickname"))}</th><th>${esc(T("status"))}</th><th>${esc(T("vlanIp"))}</th>
    <th>${esc(T("fingerprint"))}</th><th>${esc(T("platform"))}</th><th>${esc(T("lastSeen"))}</th><th>${esc(T("actions"))}</th>
  </tr></thead><tbody>${rows || `<tr><td colspan="7"><div class="empty">${esc(T("noPeers"))}</div></td></tr>`}</tbody></table>
  </div></div>
  <div class="footnote">${esc(T("vlanNote"))}</div>`;
};

PAGES.owner_invites = async () => {
  setTitle(T("invites"));
  const n = await api("/api/net");
  const rows = n.invites.map(i => `
    <tr>
      <td class="mono">****${esc(i.code)}</td>
      <td>${esc(i.note || "—")}</td>
      <td>${i.uses}${i.uses_max ? " / " + i.uses_max : ""}</td>
      <td>${i.expires ? fmtTime(i.expires) : "∞"}</td>
      <td><span class="pill admin">${esc(i.role)}</span></td>
      <td><span class="pill ${i.active ? "on" : "off"}">${i.active ? "✅" : "—"}</span></td>
      <td><button class="btn small danger" data-act="revoke" data-code="****${esc(i.code)}">${esc(T("revoke"))}</button></td>
    </tr>`).join("");
  el("view").innerHTML = `
  <div class="grid c2">
    <div class="card">
      <div class="k">${esc(T("create"))}</div>
      <div class="field"><label>${esc(T("inviteNote"))}</label><input id="invNote"></div>
      <div class="field"><label>${esc(T("usesMax"))}</label><input id="invUses" type="number" min="0" value="1"></div>
      <div class="field"><label>${esc(T("expiresDays"))}</label><input id="invDays" type="number" min="0" value="7"></div>
      <div class="field"><label>${esc(T("inviteRole"))}</label>
        <select id="invRole"><option value="member">member</option><option value="admin">admin</option></select></div>
      <button class="btn primary" data-act="inviteCreate">${esc(T("create"))}</button>
      <div id="invOut" style="margin-top:10px"></div>
    </div>
    <div class="card">
      <div class="k">${esc(T("invites"))}</div>
      <div class="table-wrap"><table><thead><tr>
        <th>code</th><th>${esc(T("note"))}</th><th>${esc(T("usesMax"))}</th><th>${esc(T("expiresDays"))}</th><th>${esc(T("role"))}</th><th></th><th></th>
      </tr></thead><tbody>${rows || `<tr><td colspan="7"><div class="empty">${esc(T("noInvites"))}</div></td></tr>`}</tbody></table></div>
    </div>
  </div>`;
};

PAGES.owner_audit = async () => {
  setTitle(T("audit"));
  const d = await api("/api/audit");
  const rows = (d.audit || []).slice().reverse().map(l => `
    <div class="tl-item"><div class="t">${fmtTime(l.ts)}</div>
    <div><span class="ev">${esc(l.event)}</span> <span class="muted mono">${esc(l.actor)}</span>
    ${l.detail ? `<div class="muted">${esc(l.detail)}</div>` : ""}</div></div>`).join("");
  el("view").innerHTML = `<div class="card"><div class="timeline">${rows || `<div class="empty">${esc(T("auditEmpty"))}</div>`}</div></div>`;
};

PAGES.owner_settings = async () => {
  setTitle(T("settings"));
  const n = await api("/api/net");
  const s = n.settings;
  el("view").innerHTML = `
  <div class="grid c2">
    <div class="card">
      <div class="k">${esc(T("settingsTitle"))}</div>
      <div class="field"><label>${esc(T("subnet"))}</label><input id="setSubnet" value="${esc(s.subnet_cidr || "")}"></div>
      <div class="field"><label>${esc(T("maxPeers"))}</label><input id="setMaxPeers" type="number" min="0" value="${s.max_peers || 0}"></div>
      <div class="field"><label>${esc(T("maxUp"))}</label><input id="setMaxUp" type="number" min="0" value="${s.max_upload_mbps || 0}"></div>
      <label style="display:flex;gap:8px;align-items:center;margin:10px 0;font-size:.85rem">
        <input type="checkbox" id="setReqKey" style="width:auto" ${s.require_invite_key ? "checked" : ""}> ${esc(T("requireKey"))}</label>
      <label style="display:flex;gap:8px;align-items:center;margin:10px 0;font-size:.85rem">
        <input type="checkbox" id="setAnon" style="width:auto" ${s.allow_anonymous_discovery ? "checked" : ""}> ${esc(T("allowAnon"))}</label>
      <label style="display:flex;gap:8px;align-items:center;margin:10px 0;font-size:.85rem">
        <input type="checkbox" id="setRelay" style="width:auto" ${s.allow_relay ? "checked" : ""}> ${esc(T("allowRelay"))}</label>
      <button class="btn primary" data-act="saveSettings">${esc(T("save"))}</button>
    </div>
    <div class="card">
      <div class="k">${esc(T("security"))}</div>
      <p class="muted" style="font-size:.83rem;line-height:1.7">${esc(T("rotateWarn"))}</p>
      <button class="btn danger" data-act="rotateKey">${esc(T("rotate"))}</button>
      <div id="rotateOut" style="margin-top:12px"></div>
    </div>
  </div>`;
};

/* ---------------- gm pages ---------------- */
PAGES.gm_overview = async () => {
  setTitle(T("gmOverview"));
  const d = await api("/api/gm/overview");
  el("view").innerHTML = `
  <div class="grid c4">
    ${stat(T("networksManaged"), d.networks)}
    ${stat(T("peersOnline"), d.peers_online, "ok")}
    ${stat(T("totalPeers"), d.peers_total)}
    ${stat(T("activeBans"), d.global_bans, d.global_bans ? "bad" : "")}
    ${stat(T("blockedNets"), d.blocked_nets, d.blocked_nets ? "bad" : "")}
    ${stat(T("joinAttempts"), d.stats.JoinAttempts || 0)}
    ${stat(T("authFails"), d.stats.AuthFails || 0, (d.stats.AuthFails || 0) > 10 ? "bad" : "")}
    ${stat(T("routedFrames"), d.stats.RoutedFrames || 0)}
  </div>
  <div class="section-title">${esc(T("security"))}<span class="line"></span></div>
  <div class="grid c3">
    <div class="card"><div class="k">${esc(T("uptime"))}</div><div class="v">${uptime(d.uptime_sec)}</div></div>
    <div class="card"><div class="k">${esc(T("version"))}</div><div class="v" style="font-size:1.1rem">${esc(d.version)}</div></div>
    <div class="card"><div class="k">${esc(T("hubFp"))}</div><div class="v mono" style="font-size:1rem">${esc(d.hub_fingerprint)}</div></div>
    <div class="card"><div class="k">${esc(T("defaultSubnet"))}</div><div class="v mono" style="font-size:1rem">${esc(d.subnet)}</div></div>
    <div class="card"><div class="k">${esc(T("created"))}</div><div class="v" style="font-size:.92rem">${fmtTime(d.started)}</div></div>
  </div>`;
};

PAGES.gm_networks = async (silent) => {
  setTitle(T("networks"));
  if (S.detail) return gmNetworkDetail(silent);
  const d = await api("/api/gm/networks");
  S.networks = d.networks;
  const rows = d.networks.map(n => `
    <tr>
      <td><strong>${esc(n.name)}</strong><div class="muted mono">${esc(n.id)}</div></td>
      <td><span class="pill ${n.blocked ? "ban" : "on"}">${n.blocked ? esc(T("blocked")) : "✅"}</span></td>
      <td>${n.online} / ${n.total}</td>
      <td>${n.admins}</td>
      <td class="muted">${fmtTime(n.created)}</td>
      <td><div class="actions">
        <button class="btn small" data-act="gmDetail" data-net="${esc(n.id)}">${esc(T("details"))}</button>
        <button class="btn small danger" data-act="${n.blocked ? "gmUnblock" : "gmBlock"}" data-net="${esc(n.id)}">${n.blocked ? esc(T("unblockNet")) : esc(T("blockNet"))}</button>
      </div></td>
    </tr>`).join("");
  el("view").innerHTML = `
  <div class="head-actions"><div class="sp"></div>
    <button class="btn small ghost" data-act="gmBanForm">＋ ${esc(T("banUser"))}</button>
  </div>
  <div class="card"><div class="table-wrap"><table>
    <thead><tr><th>${esc(T("network"))}</th><th>${esc(T("status"))}</th><th>${esc(T("peers"))}</th><th>${esc(T("adminBadge"))}</th><th>${esc(T("created"))}</th><th>${esc(T("actions"))}</th></tr></thead>
    <tbody>${rows || `<tr><td colspan="6"><div class="empty">${esc(T("noNetworks"))}</div></td></tr>`}</tbody></table></div></div>
  <div id="gmBanPanel"></div>`;
};

async function gmNetworkDetail(silent) {
  setTitle(T("details"));
  const n = await api("/api/gm/network?network=" + encodeURIComponent(S.detail));
  const rows = n.peers.map(p => `
    <tr>
      <td><strong>${esc(p.nickname)}</strong> ${p.admin ? `<span class="pill admin">${esc(T("adminBadge"))}</span>` : ""} ${p.banned ? `<span class="pill ban">${esc(T("banned"))}</span>` : ""}</td>
      <td><span class="pill ${p.online ? "on" : "off"}">${p.online ? esc(T("onlineBadge")) : esc(T("offlineBadge"))}</span></td>
      <td class="mono copy" data-copy="${esc(p.fingerprint)}">${esc(p.fingerprint)}</td>
      <td class="mono">${esc(p.vlan_ip || "—")}</td>
      <td class="muted">${fmtTime(p.last_seen)}</td>
      <td><div class="actions">
        <button class="btn small" data-act="gmKick" data-fp="${esc(p.fingerprint)}" data-net="${esc(n.id)}">${esc(T("kickAll"))}</button>
        <button class="btn small danger" data-act="gmBanPeer" data-fp="${esc(p.fingerprint)}">${esc(T("ban"))}</button>
      </div></td>
    </tr>`).join("");
  el("view").innerHTML = `
  <button class="btn small ghost" data-act="gmBack" style="margin-bottom:12px">← ${esc(T("back"))}</button>
  <div class="grid c4">
    ${stat(T("networkId"), `<span class="mono">${esc(n.id)}</span>`, "", true)}
    ${stat(T("peersOnline"), n.online, "ok")}
    ${stat(T("totalPeers"), n.total)}
    ${stat(T("status"), n.blocked ? `<span style="color:var(--bad)">${esc(T("blocked"))}</span>` : `<span style="color:var(--ok)">✅</span>`)}
  </div>
  <div class="head-actions" style="margin-top:14px">
    <button class="btn small ${n.blocked ? "" : "danger"}" data-act="${n.blocked ? "gmUnblock" : "gmBlock"}" data-net="${esc(n.id)}">${n.blocked ? esc(T("unblockNet")) : esc(T("blockNet"))}</button>
    <button class="btn small" data-act="gmRotate" data-net="${esc(n.id)}">${esc(T("rotateAdmin"))}</button>
    <button class="btn small danger" data-act="gmDelete" data-net="${esc(n.id)}">${esc(T("deleteNet"))}</button>
  </div>
  <div class="card"><div class="table-wrap"><table>
    <thead><tr><th>${esc(T("nickname"))}</th><th>${esc(T("status"))}</th><th>${esc(T("fingerprint"))}</th><th>${esc(T("vlanIp"))}</th><th>${esc(T("lastSeen"))}</th><th>${esc(T("actions"))}</th></tr></thead>
    <tbody>${rows || `<tr><td colspan="6"><div class="empty">${esc(T("emptyNet"))}</div></td></tr>`}</tbody></table></div></div>
  <div class="section-title">${esc(T("audit"))}<span class="line"></span></div>
  <div class="card"><div class="timeline">${(n.audit || []).slice().reverse().map(l => `<div class="tl-item"><div class="t">${fmtTime(l.ts)}</div><div><span class="ev">${esc(l.event)}</span> <span class="muted mono">${esc(l.actor)}</span>${l.detail ? `<div class="muted">${esc(l.detail)}</div>` : ""}</div></div>`).join("") || `<div class="empty">${esc(T("auditEmpty"))}</div>`}</div></div>`;
}

PAGES.gm_analytics = async () => {
  setTitle(T("analytics"));
  const d = await api("/api/gm/analytics");
  const hours = Object.keys(d.events_per_hour || {}).sort();
  const max = Math.max(1, ...hours.map(h => d.events_per_hour[h]));
  const bars = hours.map(h => `<div class="b" style="height:${Math.max(3, 100 * d.events_per_hour[h] / max)}%" title="${h}: ${d.events_per_hour[h]}"><span>${d.events_per_hour[h] || ""}</span></div>`).join("");
  const xaxis = hours.map(h => `<span>${h}</span>`).join("");
  el("view").innerHTML = `
  <div class="grid c4">
    ${stat(T("joinAttempts"), d.security.join_attempts || 0)}
    ${stat(T("authFails"), d.security.auth_fails || 0, d.security.auth_fails > 10 ? "bad" : "")}
    ${stat(T("activeBans"), d.security.global_bans, d.security.global_bans ? "bad" : "")}
    ${stat(T("blockedNets"), d.security.blocked_nets, d.security.blocked_nets ? "bad" : "")}
  </div>
  <div class="section-title">${esc(T("eventsPerHour"))}<span class="line"></span></div>
  <div class="card">${hours.length ? `<div class="bars">${bars}</div><div class="bars-x">${xaxis}</div>` : `<div class="empty">—</div>`}</div>
  <div class="grid c2">
    <div class="card"><div class="k">${esc(T("topEvents"))}</div>
      ${(d.top_events || []).map(e => `<div style="display:flex;justify-content:space-between;padding:6px 0;border-bottom:1px dashed var(--line);font-size:.83rem"><span>${esc(e.event)}</span><span class="mono">${e.count}</span></div>`).join("") || "—"}
    </div>
    <div class="card"><div class="k">${esc(T("networkUsage"))}</div>
      ${(d.network_usage || []).map(n => `<div style="display:flex;justify-content:space-between;padding:6px 0;border-bottom:1px dashed var(--line);font-size:.83rem"><span>${esc(n.name)} ${n.blocked ? "⛔" : ""}</span><span class="mono">${n.online}/${n.total}</span></div>`).join("") || "—"}
    </div>
  </div>`;
};

PAGES.gm_bans = async () => {
  setTitle(T("bans"));
  const d = await api("/api/gm/audit");
  const nets = await api("/api/gm/networks");
  const rows = (d.audit || []).filter(l => l.event === "GLOBAL_BAN" || l.event === "GLOBAL_UNBAN").slice().reverse().map(l => `
    <div class="tl-item"><div class="t">${fmtTime(l.ts)}</div><div><span class="ev">${esc(l.event)}</span> <span class="mono muted">${esc(l.actor)}</span><div class="muted mono">${esc(l.detail)}</div></div></div>`).join("");
  el("view").innerHTML = `
  <div class="card" style="margin-bottom:14px">
    <div class="k">${esc(T("banUser"))}</div>
    <div class="grid c2">
      <div class="field"><label>${esc(T("banFp"))}</label><input id="gmBanFp" placeholder="a1b2c3d4e5f6"></div>
      <div class="field"><label>${esc(T("banReason"))}</label><input id="gmBanReason" placeholder="${esc(T("note"))}"></div>
    </div>
    <button class="btn danger" data-act="gmBanByFp">${esc(T("ban"))}</button>
    <div class="footnote">${esc(T("gmBansTitle"))}</div>
  </div>
  <div class="card"><div class="timeline">${rows || `<div class="empty">${esc(T("noBans"))}</div>`}</div></div>`;
};

PAGES.gm_audit = async () => {
  setTitle(T("audit"));
  const d = await api("/api/gm/audit");
  const rows = (d.audit || []).slice().reverse().map(l => `
    <div class="tl-item"><div class="t">${fmtTime(l.ts)}</div><div><span class="ev">${esc(l.event)}</span> <span class="mono muted">${esc(l.actor)}</span>${l.detail ? `<div class="muted">${esc(l.detail)}</div>` : ""}</div></div>`).join("");
  el("view").innerHTML = `<div class="card"><div class="timeline">${rows || `<div class="empty">${esc(T("auditEmpty"))}</div>`}</div></div>`;
};

PAGES.fallback = async () => { el("view").innerHTML = ""; };

/* ---------------- actions (event delegation) ---------------- */
document.addEventListener("click", async e => {
  const copyEl = e.target.closest("[data-copy]");
  if (copyEl) {
    try { await navigator.clipboard.writeText(copyEl.dataset.copy); toast(T("copied"), "ok"); } catch (_) {}
    return;
  }
  const b = e.target.closest("[data-act]");
  if (!b) return;
  const act = b.dataset.act, fp = b.dataset.fp || "", net = b.dataset.net || "";
  const q = s => document.querySelector(s) ? document.querySelector(s).value : "";
  b.disabled = true;
  try {
    switch (act) {
      case "kick":
        if (!confirm(T("confirmKick"))) break;
        await api("/api/peers/kick", { method: "POST", body: JSON.stringify({ fp }) });
        toast(T("changedOk"), "ok"); route(); break;
      case "ban":
        if (!confirm(T("confirmBan"))) break;
        await api("/api/peers/ban", { method: "POST", body: JSON.stringify({ fp, reason: "" }) });
        toast(T("changedOk"), "ok"); route(); break;
      case "unban":
        await api("/api/peers/unban", { method: "POST", body: JSON.stringify({ fp }) });
        toast(T("changedOk"), "ok"); route(); break;
      case "promote":
        await api("/api/peers/promote", { method: "POST", body: JSON.stringify({ fp, admin: true }) });
        toast(T("promoteOk"), "ok"); route(); break;
      case "demote":
        await api("/api/peers/promote", { method: "POST", body: JSON.stringify({ fp, admin: false }) });
        toast(T("promoteOk"), "ok"); route(); break;
      case "inviteCreate": {
        const out = await api("/api/invites", { method: "POST", body: JSON.stringify({
          note: q("#invNote"), uses_max: Number(q("#invUses") || 0), expires_days: Number(q("#invDays") || 0), role: q("#invRole")
        }) });
        el("invOut").innerHTML = `<div class="card" style="border-color:rgba(53,208,127,.4)">
          <div class="k">${esc(T("copy"))}</div><div class="mono copy" data-copy="${esc(out.code)}" style="font-size:1rem">${esc(out.code)}</div>
          <div class="muted" style="font-size:.75rem;margin-top:6px">${esc(T("inviteNote"))}</div></div>`;
        route(); break;
      }
      case "revoke":
        await api("/api/invites/revoke", { method: "POST", body: JSON.stringify({ code: b.dataset.code }) });
        toast(T("changedOk"), "ok"); route(); break;
      case "saveSettings":
        await api("/api/net/settings", { method: "POST", body: JSON.stringify({
          subnet_cidr: q("#setSubnet"), max_peers: Number(q("#setMaxPeers") || 0),
          max_upload_mbps: Number(q("#setMaxUp") || 0),
          require_invite_key: document.querySelector("#setReqKey").checked,
          allow_anonymous_discovery: document.querySelector("#setAnon").checked,
          allow_relay: document.querySelector("#setRelay").checked
        }) });
        toast(T("settingsSaved"), "ok"); break;
      case "rotateKey":
        if (!confirm(T("confirmRotate"))) break;
        { const out = await api("/api/net/rotate-admin", { method: "POST" });
          el("rotateOut").innerHTML = `<div class="card"><div class="k">${esc(T("newAdminKey"))}</div>
            <div class="mono copy" data-copy="${esc(out.admin_key)}">${esc(out.admin_key)}</div></div>`; }
        break;
      case "gmDetail": S.detail = net; await route(); break;
      case "gmBack": S.detail = ""; await route(); break;
      case "gmBlock":
        if (!confirm(T("confirmBlock"))) break;
        await api("/api/gm/block-network", { method: "POST", body: JSON.stringify({ network: net, reason: prompt(T("blockReason")) || "" }) });
        toast(T("changedOk"), "ok"); route(); break;
      case "gmUnblock":
        await api("/api/gm/unblock-network", { method: "POST", body: JSON.stringify({ network: net }) });
        toast(T("changedOk"), "ok"); route(); break;
      case "gmRotate":
        if (!confirm(T("confirmRotate"))) break;
        { const out = await api("/api/gm/rotate-admin", { method: "POST", body: JSON.stringify({ network: net }) });
          alert(T("newAdminKey") + "\n\n" + out.admin_key); }
        break;
      case "gmDelete":
        if (!confirm(T("confirmDelete"))) break;
        await api("/api/gm/delete-network", { method: "POST", body: JSON.stringify({ network: net }) });
        S.detail = ""; toast(T("changedOk"), "ok"); route(); break;
      case "gmKick":
        await api("/api/gm/kick", { method: "POST", body: JSON.stringify({ fp, network: net }) });
        toast(T("changedOk"), "ok"); route(); break;
      case "gmBanPeer": {
        const reason = prompt(T("banReason")) || "";
        await api("/api/gm/ban", { method: "POST", body: JSON.stringify({ fp, reason }) });
        toast(T("changedOk"), "ok"); route(); break;
      }
      case "gmBanForm": {
        await api("/api/gm/ban", { method: "POST", body: JSON.stringify({ fp: prompt(T("banFp")) || "", reason: prompt(T("banReason")) || "" }) });
        toast(T("changedOk"), "ok"); route(); break;
      }
      case "gmBanByFp":
        await api("/api/gm/ban", { method: "POST", body: JSON.stringify({ fp: q("#gmBanFp"), reason: q("#gmBanReason") }) });
        toast(T("changedOk"), "ok"); route(); break;
    }
  } catch (err) {
    if (String(err.message) !== "unauthorized") toast(T("error") + ": " + err.message, "bad");
  } finally { b.disabled = false; }
});

/* ---------------- boot ---------------- */
applyI18n();
setLoginTab("owner");
if ("serviceWorker" in navigator) navigator.serviceWorker.register("/sw.js").catch(() => {});
document.addEventListener("visibilitychange", () => {
  if (!document.hidden && S.token) route(true);
});
if (S.token && S.kind) enterApp();
})();
