"use strict";

const http = require("http");
const { HOST, PORT, startBackend, stopBackend } = require("./server.js");
const autostart = require("./autostart.js");

const PEER_COLUMNS = [
    ["ipv4", "IP"],
    ["hostname", "Устройство"],
    ["cost", "Тип"],
    ["latency", "Задержка"],
    ["loss", "Потери"],
    ["rx", "Приём"],
    ["tx", "Передача"],
    ["nat", "NAT"],
];

let state = { networks: [], connected: [] };
let peers = {};
let editingIndex = -1;
let shuttingDown = false;

function request(method, urlPath, body) {
    return new Promise((resolve, reject) => {
        const data = body ? JSON.stringify(body) : null;
        const req = http.request({
            host: HOST,
            port: PORT,
            path: urlPath,
            method,
            headers: data
                ? { "Content-Type": "application/json", "Content-Length": Buffer.byteLength(data) }
                : {},
        }, (res) => {
            let buf = "";
            res.on("data", (c) => { buf += c; });
            res.on("end", () => {
                try {
                    resolve(JSON.parse(buf || "{}"));
                } catch (e) {
                    reject(new Error("некорректный ответ сервиса"));
                }
            });
        });
        req.on("error", reject);
        if (data) req.write(data);
        req.end();
    });
}

function esc(s) {
    return String(s == null ? "" : s)
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;");
}

function setStatus(text, isError) {
    const el = document.getElementById("status");
    el.textContent = text;
    el.classList.toggle("error", !!isError);
}

async function refreshState() {
    const r = await request("GET", "/api/state");
    if (!r.ok) throw new Error(r.error || "не удалось получить состояние");
    state.networks = r.networks || [];
    state.connected = r.connected || [];
}

async function refreshPeers() {
    try {
        const r = await request("GET", "/api/peers");
        if (r.ok) {
            peers = r.networks || {};
            renderPeers();
            updateStatus();
        }
    } catch (e) { /* сервис может быть занят */ }
}

function updateStatus() {
    const total = state.networks.length;
    const on = state.connected.length;
    const failed = state.networks.filter((n) => peers[n.name] && peers[n.name].error).length;
    if (total === 0) {
        setStatus("Сетей нет");
    } else if (failed > 0) {
        setStatus("Ошибка подключения", true);
    } else if (on === 0) {
        setStatus("Не подключено");
    } else {
        setStatus("Подключено: " + on + " из " + total);
    }
}

function render() {
    const wrap = document.getElementById("networks");
    document.getElementById("empty").classList.toggle("hidden", state.networks.length > 0);
    wrap.innerHTML = "";

    state.networks.forEach((net, i) => {
        const connected = state.connected.includes(net.name);
        const card = document.createElement("div");
        card.className = "card";
        card.dataset.name = net.name;
        card.innerHTML =
            '<div class="card-head">' +
            '<div class="dot ' + (connected ? "on" : "off") + '"></div>' +
            "<div>" +
            '<div class="net-name">' + esc(net.name) + "</div>" +
            '<div class="net-sub">' + esc(net.hostname || "имя устройства по умолчанию") + "</div>" +
            "</div>" +
            '<div class="card-actions">' +
            '<button class="connect">' + (connected ? "Отключить" : "Подключить") + "</button>" +
            '<button class="ghost edit">Изменить</button>' +
            '<button class="danger del">Удалить</button>' +
            "</div>" +
            "</div>" +
            '<table><thead><tr>' +
            PEER_COLUMNS.map((c) => "<th>" + c[1] + "</th>").join("") +
            "</tr></thead><tbody></tbody></table>";

        card.querySelector(".connect").addEventListener("click", () => {
            if (connected) disconnectNetwork(net.name);
            else connectNetwork(net.name);
        });
        card.querySelector(".edit").addEventListener("click", () => openNetDialog(i));
        card.querySelector(".del").addEventListener("click", () => deleteNetwork(i));
        wrap.appendChild(card);
    });

    renderPeers();
    updateStatus();
}

function renderPeers() {
    state.networks.forEach((net) => {
        const card = document.querySelector('.card[data-name="' + cssEscape(net.name) + '"]');
        if (!card) return;
        const tbody = card.querySelector("tbody");
        const data = peers[net.name];
        const list = data && Array.isArray(data.peers) ? data.peers : [];

        if (data && data.error) {
            tbody.innerHTML = '<tr><td colspan="' + PEER_COLUMNS.length + '" class="peers-error">' +
                esc(data.error) + "</td></tr>";
            return;
        }
        if (data && data.starting) {
            tbody.innerHTML = '<tr><td colspan="' + PEER_COLUMNS.length + '" class="peers-empty">' +
                "Подключение (первый раз скачивается EasyTier)..." + "</td></tr>";
            return;
        }
        if (list.length === 0) {
            tbody.innerHTML = '<tr><td colspan="' + PEER_COLUMNS.length + '" class="peers-empty">' +
                (data && data.connected ? "Ожидание устройств..." : "Не подключено") + "</td></tr>";
            return;
        }

        tbody.innerHTML = list.map((p) => {
            const cells = PEER_COLUMNS.map((c) => {
                const v = p[c[0]];
                const cls = c[0] === "ipv4" ? ' class="mono"' : "";
                return "<td" + cls + ">" + esc(v) + "</td>";
            }).join("");
            return "<tr>" + cells + "</tr>";
        }).join("");
    });
}

function cssEscape(s) {
    return String(s).replace(/["\\]/g, "\\$&");
}

async function saveNetworks() {
    const r = await request("POST", "/api/networks", { networks: state.networks });
    if (!r.ok) throw new Error(r.error || "не удалось сохранить");
}

async function connectNetwork(name) {
    setStatus("Подключение...");
    try {
        const r = await request("POST", "/api/connect", { name });
        if (!r.ok) throw new Error(r.error || "ошибка подключения");
    } catch (e) {
        setStatus(e.message, true);
        return;
    }
    await refreshState();
    render();
    await refreshPeers();
    setTimeout(refreshPeers, 800);
}

async function disconnectNetwork(name) {
    try {
        await request("POST", "/api/disconnect", { name });
    } catch (e) { /* ignore */ }
    await refreshState();
    render();
    refreshPeers();
}

async function deleteNetwork(index) {
    const net = state.networks[index];
    if (!net) return;
    if (!confirm('Удалить сеть «' + net.name + '»?')) return;
    state.networks.splice(index, 1);
    await saveNetworks();
    await refreshState();
    render();
}

function openNetDialog(index) {
    editingIndex = typeof index === "number" ? index : -1;
    const editing = editingIndex >= 0;
    document.getElementById("dialogTitle").textContent = editing ? "Изменить сеть" : "Новая сеть";
    const net = editing ? state.networks[editingIndex] : { name: "", secret: "", hostname: "" };
    document.getElementById("f-name").value = net.name || "";
    document.getElementById("f-secret").value = net.secret || "";
    document.getElementById("f-hostname").value = net.hostname || "";
    document.getElementById("netDialog").showModal();
}

async function saveNetDialog() {
    const name = document.getElementById("f-name").value.trim();
    if (!name) {
        alert("Введите имя сети");
        return;
    }
    const net = {
        name,
        secret: document.getElementById("f-secret").value,
        hostname: document.getElementById("f-hostname").value.trim(),
    };

    const duplicate = state.networks.some((n, i) => n.name === name && i !== editingIndex);
    if (duplicate) {
        alert("Сеть с таким именем уже есть");
        return;
    }

    if (editingIndex >= 0) state.networks[editingIndex] = net;
    else state.networks.push(net);

    document.getElementById("netDialog").close();
    try {
        await saveNetworks();
    } catch (e) {
        alert(e.message);
    }
    await refreshState();
    render();
    refreshPeers();
}

function openOptsDialog() {
    document.getElementById("opt-autostart").checked = autostart.isEnabled();
    document.getElementById("optsDialog").showModal();
}

function saveOptsDialog() {
    const want = document.getElementById("opt-autostart").checked;
    try {
        if (want) autostart.enable();
        else autostart.disable();
    } catch (e) {
        alert("Не удалось изменить автозапуск: " + e.message);
    }
    document.getElementById("optsDialog").close();
}

async function boot() {
    setStatus("Запуск сервиса...");
    try {
        await startBackend();
    } catch (e) {
        setStatus("Ошибка: " + e.message, true);
        return;
    }
    await refreshState();
    render();
    await refreshPeers();

    setInterval(refreshPeers, 2500);
    setInterval(async () => {
        try {
            const before = state.connected.join(",");
            await refreshState();
            if (state.connected.join(",") !== before) render();
        } catch (e) { /* ignore */ }
    }, 3000);
}

function wireUI() {
    try { nw.Window.get().menu = new nw.Menu(); } catch (e) { /* меню необязательно */ }
    document.getElementById("addBtn").addEventListener("click", () => openNetDialog(-1));
    document.getElementById("optsBtn").addEventListener("click", openOptsDialog);
    document.getElementById("dialogSave").addEventListener("click", (e) => { e.preventDefault(); saveNetDialog(); });
    document.getElementById("optsSave").addEventListener("click", (e) => { e.preventDefault(); saveOptsDialog(); });
}

function wireClose() {
    try {
        const win = nw.Window.get();
        win.on("close", async function () {
            if (shuttingDown) return;
            shuttingDown = true;
            win.hide();
            try { await stopBackend(); } catch (e) { /* ignore */ }
            win.close(true);
        });
    } catch (e) { /* ignore */ }
}

window.addEventListener("DOMContentLoaded", () => {
    wireUI();
    wireClose();
    boot();
});
