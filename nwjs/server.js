"use strict";

const { spawn } = require("child_process");
const path = require("path");
const fs = require("fs");
const http = require("http");

const HOST = "127.0.0.1";
const PORT = 19090;

let child = null;

function appDir() {
    try {
        if (typeof nw !== "undefined" && nw.App && nw.App.startPath) {
            return nw.App.startPath;
        }
    } catch (e) { /* ignore */ }
    return path.resolve(__dirname);
}

function backendPath() {
    if (process.env.PNET_BACKEND) return process.env.PNET_BACKEND;
    const name = process.platform === "win32" ? "pnet-core.exe" : "pnet-core";
    // __dirname всегда рядом с бэкендом (в AppImage это usr/bin, где лежат
    // pnet, pnet-core и server.js). appDir() в AppImage указывает на корень
    // образа, поэтому используем его только как запасной вариант.
    const here = path.resolve(__dirname);
    const dir = appDir();
    const candidates = [
        path.join(here, name),
        path.join(dir, name),
        path.join(here, "..", name),
        path.join(dir, "..", name),
        path.join(dir, "..", "build", "dist-linux", name),
        path.join(dir, "..", "build", "dist-win", name),
    ];
    for (const c of candidates) {
        try {
            if (fs.existsSync(c)) return c;
        } catch (e) { /* ignore */ }
    }
    return candidates[0];
}

function ping() {
    return new Promise((resolve) => {
        const req = http.get({ host: HOST, port: PORT, path: "/api/state", timeout: 2000 }, (res) => {
            res.resume();
            resolve(res.statusCode === 200);
        });
        req.on("error", () => resolve(false));
        req.on("timeout", () => { req.destroy(); resolve(false); });
    });
}

function startBackend() {
    return new Promise((resolve, reject) => {
        ping().then((up) => {
            if (up) return resolve();

            const bin = backendPath();
            if (!fs.existsSync(bin)) {
                return reject(new Error("бэкенд не найден: " + bin));
            }

            child = spawn(bin, ["server"], {
                windowsHide: true,
                stdio: "ignore",
                detached: false,
            });
            child.on("error", reject);

            const startedAt = Date.now();
            const iv = setInterval(async () => {
                if (await ping()) {
                    clearInterval(iv);
                    resolve();
                } else if (Date.now() - startedAt > 120000) {
                    clearInterval(iv);
                    reject(new Error("не удалось дождаться запуска сервиса"));
                }
            }, 500);
        });
    });
}

function stopBackend() {
    return new Promise((resolve) => {
        const req = http.request({ host: HOST, port: PORT, path: "/api/shutdown", method: "POST" }, (res) => {
            res.resume();
            resolve();
        });
        req.on("error", () => resolve());
        req.end();
        setTimeout(() => {
            if (child) {
                try { child.kill(); } catch (e) { /* ignore */ }
            }
            resolve();
        }, 1500);
    });
}

module.exports = { HOST, PORT, appDir, backendPath, startBackend, stopBackend };
