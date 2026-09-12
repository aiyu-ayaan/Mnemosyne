// The only bridge between the renderer and Node.
//
// Node integration is off in the renderer, so this exposes a fixed, narrow
// surface instead: four calls and two subscriptions. The renderer cannot reach
// the filesystem, spawn anything, or open a socket of its own.

const { contextBridge, ipcRenderer } = require("electron");

const CHANNEL = "mnemosyne";

contextBridge.exposeInMainWorld(CHANNEL, {
  /**
   * One JSON request to the daemon.
   * @param {string} method
   * @param {string} path
   * @param {unknown} [body]
   * @returns {Promise<{status: number, body: unknown}>}
   */
  request: (method, path, body) => ipcRenderer.invoke(`${CHANNEL}:request`, method, path, body),

  /** Where the daemon is and which root it serves, or null if not connected. */
  info: () => ipcRenderer.invoke(`${CHANNEL}:info`),

  /** Opens a path in the OS file manager. */
  reveal: (target) => ipcRenderer.invoke(`${CHANNEL}:reveal`, target),

  /** Native directory picker for the memory root. Resolves null if cancelled. */
  pickDirectory: () => ipcRenderer.invoke(`${CHANNEL}:pickDirectory`),

  /** Runs mnemosyne install to place binary in Programs and update user PATH. */
  installBinary: () => ipcRenderer.invoke(`${CHANNEL}:installBinary`),

  /**
   * Subscribes to daemon change events. Returns an unsubscribe function.
   * @param {(event: {kind: string, project?: string, memory?: string}) => void} handler
   */
  onEvent: (handler) => {
    const listener = (_e, event) => handler(event);
    ipcRenderer.on(`${CHANNEL}:event`, listener);
    return () => ipcRenderer.off(`${CHANNEL}:event`, listener);
  },

  /** Subscribes to a startup failure the main process could not recover from. */
  onFatal: (handler) => {
    const listener = (_e, message) => handler(message);
    ipcRenderer.on(`${CHANNEL}:fatal`, listener);
    return () => ipcRenderer.off(`${CHANNEL}:fatal`, listener);
  },
});
