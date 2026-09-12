// Electron main process.
//
// It is plain CommonJS on purpose: the renderer is where the application lives
// and where TypeScript earns its keep. Adding a second build pipeline to
// transpile this file would be more machinery than the file is long.
//
// Its whole job is to be the bridge. The renderer cannot open a named pipe or a
// unix socket, so every request goes through IPC to here, and here speaks
// ordinary HTTP over the channel via Node's `socketPath` — which accepts both a
// Windows named pipe and a unix socket, so there is one implementation.

const { app, BrowserWindow, ipcMain, shell, dialog } = require("electron");
const { execFile, spawn } = require("node:child_process");
const http = require("node:http");
const path = require("node:path");

const DEV_URL = process.env.MNEMOSYNE_DEV_URL || "http://localhost:5173";
const CHANNEL = "mnemosyne";

/** How long to wait for a daemon we just spawned to answer. */
const START_TIMEOUT_MS = 15_000;
const START_POLL_MS = 250;

/** @type {{endpoint: string, token: string, root: string, portable: boolean} | null} */
let channelInfo = null;

/** @type {import("node:http").ClientRequest | null} */
let eventStream = null;

/** Resolved once: asking the binary repeatedly for a path that cannot change is waste. */
let binaryPath = null;

// --- the backend binary ---

/**
 * Finds the mnemosyne binary. The packaged app ships it beside the resources;
 * a development checkout has it in the workspace's bin directory; otherwise it
 * is expected on PATH, which is what `mnemosyne install` arranges.
 */
function findBinary() {
  if (binaryPath) return binaryPath;

  const name = process.platform === "win32" ? "mnemosyne.exe" : "mnemosyne";
  const candidates = [
    process.env.MNEMOSYNE_BIN,
    path.join(process.resourcesPath || "", name),
    // apps/desktop -> apps -> repo root -> bin
    path.join(app.getAppPath(), "..", "..", "bin", name),
    name,
  ].filter(Boolean);

  // The bare name is last and always present, so the loop cannot come up empty.
  binaryPath = candidates[candidates.length - 1];
  for (const candidate of candidates) {
    try {
      require("node:fs").accessSync(candidate);
      binaryPath = candidate;
      break;
    } catch {
      // Not there; try the next one. A bare name on PATH is not accessSync-able
      // by definition, which is why it stays the fallback rather than a match.
    }
  }
  return binaryPath;
}

/**
 * Asks the binary where the daemon listens. Doing this rather than
 * reimplementing root resolution, portable detection, and per-platform endpoint
 * naming in JavaScript means the app and the backend cannot disagree about it.
 * @returns {Promise<{endpoint: string, baseUrl: string, tokenPath: string, token: string, root: string, portable: boolean, running: boolean}>}
 */
function readChannel() {
  return new Promise((resolve, reject) => {
    execFile(findBinary(), ["channel", "--json"], { timeout: 10_000 }, (err, stdout) => {
      if (err) {
        reject(new Error(`could not run "${findBinary()} channel": ${err.message}`));
        return;
      }
      try {
        resolve(JSON.parse(stdout));
      } catch (parseError) {
        reject(new Error(`unreadable channel description: ${parseError.message}`));
      }
    });
  });
}

/**
 * Makes sure a daemon is answering, starting one if not.
 *
 * An installed Mnemosyne already starts its daemon at logon, so this normally
 * finds one. Spawning is for a development checkout and for the first run
 * before the logon entry has fired once.
 */
async function ensureDaemon() {
  let info = await readChannel();
  if (info.running) return info;

  const child = spawn(findBinary(), ["daemon"], { detached: true, stdio: "ignore" });
  child.unref();

  const deadline = Date.now() + START_TIMEOUT_MS;
  while (Date.now() < deadline) {
    await new Promise((r) => setTimeout(r, START_POLL_MS));
    info = await readChannel();
    if (info.running) return info;
  }
  throw new Error(`the daemon did not start within ${START_TIMEOUT_MS / 1000}s (endpoint ${info.endpoint})`);
}

// --- HTTP over the channel ---

/**
 * One request to the daemon.
 * @param {string} method
 * @param {string} urlPath
 * @param {unknown} [body]
 * @returns {Promise<{status: number, body: unknown}>}
 */
function request(method, urlPath, body) {
  if (!channelInfo) return Promise.reject(new Error("not connected to a daemon"));

  const payload = body === undefined ? null : Buffer.from(JSON.stringify(body));
  const headers = { Authorization: `Bearer ${channelInfo.token}` };
  if (payload) {
    headers["Content-Type"] = "application/json";
    headers["Content-Length"] = payload.length;
  }

  return new Promise((resolve, reject) => {
    const req = http.request(
      { socketPath: channelInfo.endpoint, path: urlPath, method, headers },
      (res) => {
        const chunks = [];
        res.on("data", (chunk) => chunks.push(chunk));
        res.on("end", () => {
          const text = Buffer.concat(chunks).toString("utf8");
          let parsed = null;
          if (text) {
            try {
              parsed = JSON.parse(text);
            } catch {
              // 204s and any non-JSON error page land here. The status is what
              // the caller acts on, so an unparseable body is not fatal.
              parsed = { error: text };
            }
          }
          resolve({ status: res.statusCode || 0, body: parsed });
        });
      },
    );
    req.on("error", reject);
    if (payload) req.write(payload);
    req.end();
  });
}

/**
 * Holds the SSE stream open and forwards each event to the window, so a memory
 * written by an agent appears without the renderer polling.
 * @param {BrowserWindow} win
 */
function streamEvents(win) {
  if (!channelInfo) return;

  const req = http.request(
    {
      socketPath: channelInfo.endpoint,
      path: "/v1/events",
      method: "GET",
      headers: { Authorization: `Bearer ${channelInfo.token}`, Accept: "text/event-stream" },
    },
    (res) => {
      let buffer = "";
      res.setEncoding("utf8");
      res.on("data", (chunk) => {
        buffer += chunk;
        // SSE messages are separated by a blank line. Anything after the last
        // one is a partial message and stays in the buffer.
        const messages = buffer.split("\n\n");
        buffer = messages.pop() || "";
        for (const message of messages) {
          for (const line of message.split("\n")) {
            if (!line.startsWith("data:")) continue;
            try {
              const event = JSON.parse(line.slice(5).trim());
              if (!win.isDestroyed()) win.webContents.send(`${CHANNEL}:event`, event);
            } catch {
              // A heartbeat comment or a truncated line. Nothing to report.
            }
          }
        }
      });
      res.on("end", () => reconnect(win));
    },
  );
  req.on("error", () => reconnect(win));
  req.end();
  eventStream = req;
}

/**
 * Reopens the stream after the daemon restarts. The renderer refetches on
 * reconnect, so a gap costs a stale view for a second, not a wrong one.
 * @param {BrowserWindow} win
 */
function reconnect(win) {
  eventStream = null;
  if (win.isDestroyed()) return;
  setTimeout(() => {
    if (!win.isDestroyed()) streamEvents(win);
  }, 2000);
}

// --- window ---

function createWindow() {
  const win = new BrowserWindow({
    width: 1280,
    height: 820,
    minWidth: 720,
    minHeight: 480,
    // Dark first, and set here as well as in CSS so the frame does not flash
    // white before the renderer paints.
    backgroundColor: "#1f1f1f",
    autoHideMenuBar: true,
    webPreferences: {
      preload: path.join(__dirname, "preload.cjs"),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: false,
    },
  });

  // Anything that wants a new window is an external link. Open it in the
  // browser rather than giving a web page a chrome-less Electron window.
  win.webContents.setWindowOpenHandler(({ url }) => {
    shell.openExternal(url);
    return { action: "deny" };
  });

  // A renderer error in development is otherwise invisible from the terminal,
  // which makes a blank window impossible to diagnose.
  if (!app.isPackaged) {
    win.webContents.on("console-message", (_e, level, message, line, source) => {
      if (level >= 2) console.error(`[renderer] ${source}:${line} ${message}`);
    });
  }

  loadRenderer(win);
  return win;
}

/**
 * Loads the UI: the dev server when one is running, the built files otherwise.
 *
 * Vite may still be starting when the window opens, so the dev URL is retried
 * before falling back — but it does fall back, so `electron .` after a build
 * shows the app instead of a blank window.
 */
function loadRenderer(win, attempt = 0) {
  const built = path.join(__dirname, "..", "dist", "index.html");
  if (app.isPackaged) {
    win.loadFile(built);
    return;
  }

  win.loadURL(DEV_URL).catch(() => {
    if (win.isDestroyed()) return;
    if (attempt < 6) {
      setTimeout(() => loadRenderer(win, attempt + 1), 500);
      return;
    }
    if (require("node:fs").existsSync(built)) {
      console.log(`no dev server at ${DEV_URL} — loading the build in dist/`);
      win.loadFile(built);
      return;
    }
    setTimeout(() => loadRenderer(win, attempt + 1), 1000);
  });
}

app.whenReady().then(async () => {
  ipcMain.handle(`${CHANNEL}:request`, (_e, method, urlPath, body) => request(method, urlPath, body));
  ipcMain.handle(`${CHANNEL}:info`, () => channelInfo);
  ipcMain.handle(`${CHANNEL}:reveal`, (_e, target) => shell.openPath(target));
  ipcMain.handle(`${CHANNEL}:pickDirectory`, async () => {
    const result = await dialog.showOpenDialog({
      title: "Choose a memory root",
      properties: ["openDirectory", "createDirectory"],
    });
    return result.canceled ? null : result.filePaths[0];
  });

  const win = createWindow();

  try {
    channelInfo = await ensureDaemon();
    streamEvents(win);
  } catch (err) {
    // The window still opens and says what went wrong. A dialog on top of a
    // blank window would be the same information with nothing behind it.
    if (!win.isDestroyed()) {
      win.webContents.once("did-finish-load", () => {
        win.webContents.send(`${CHANNEL}:fatal`, String(err.message || err));
      });
    }
  }

  app.on("activate", () => {
    if (BrowserWindow.getAllWindows().length === 0) createWindow();
  });
});

app.on("window-all-closed", () => {
  if (eventStream) eventStream.destroy();
  // The daemon is deliberately left running: it is the same per-user process
  // the logon entry starts, and an agent's MCP session may be relying on it.
  if (process.platform !== "darwin") app.quit();
});
