"use strict";

const fs = require("fs");
const path = require("path");
const os = require("os");
const { execFileSync } = require("child_process");

const WIN_RUN_KEY = "HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run";
const WIN_VALUE = "pnet";

function desktopFile() {
    return path.join(os.homedir(), ".config", "autostart", "pnet.desktop");
}

function quote(s) {
    return '"' + s + '"';
}

// Команда запуска приложения. В упакованном виде достаточно самого exe,
// в режиме разработки добавляем путь к приложению.
function launchCommand() {
    // В AppImage сборке путь к смонтированному приложению временный — для
    // автозапуска нужен сам файл .AppImage.
    if (process.platform !== "win32" && process.env.APPIMAGE) {
        return [process.env.APPIMAGE];
    }
    const exe = process.execPath;
    const parts = [exe];
    try {
        if (typeof nw !== "undefined" && nw.App && nw.App.startPath) {
            const start = path.resolve(nw.App.startPath);
            const exeDir = path.resolve(path.dirname(exe));
            if (start !== exeDir) parts.push(start);
        }
    } catch (e) { /* ignore */ }
    return parts;
}

function isEnabled() {
    if (process.platform === "win32") {
        try {
            const out = execFileSync("reg", ["query", WIN_RUN_KEY, "/v", WIN_VALUE], { encoding: "utf8" });
            return out.includes(WIN_VALUE);
        } catch (e) {
            return false;
        }
    }
    return fs.existsSync(desktopFile());
}

function enable() {
    const cmd = launchCommand().map(quote).join(" ");
    if (process.platform === "win32") {
        execFileSync("reg", [
            "add", WIN_RUN_KEY, "/v", WIN_VALUE, "/t", "REG_SZ", "/d", cmd, "/f",
        ]);
        return;
    }
    const file = desktopFile();
    fs.mkdirSync(path.dirname(file), { recursive: true });
    const content =
        "[Desktop Entry]\n" +
        "Type=Application\n" +
        "Name=pnet\n" +
        "Comment=pnet VPN\n" +
        "Exec=" + cmd + "\n" +
        "Terminal=false\n" +
        "X-GNOME-Autostart-enabled=true\n";
    fs.writeFileSync(file, content, { mode: 0o644 });
}

function disable() {
    if (process.platform === "win32") {
        try {
            execFileSync("reg", ["delete", WIN_RUN_KEY, "/v", WIN_VALUE, "/f"]);
        } catch (e) { /* ignore */ }
        return;
    }
    try {
        fs.unlinkSync(desktopFile());
    } catch (e) { /* ignore */ }
}

module.exports = { isEnabled, enable, disable };
