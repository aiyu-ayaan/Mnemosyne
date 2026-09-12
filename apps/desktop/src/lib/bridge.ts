import type { ChangeEvent, ChannelInfo } from "./types";

// The renderer's only capability. Everything it can do to the outside world is
// on this object, injected by the preload script.
type Bridge = {
  request(method: string, path: string, body?: unknown): Promise<{ status: number; body: unknown }>;
  info(): Promise<ChannelInfo | null>;
  reveal(target: string): Promise<string>;
  pickDirectory(): Promise<string | null>;
  installBinary?(): Promise<string>;
  onEvent(handler: (event: ChangeEvent) => void): () => void;
  onFatal(handler: (message: string) => void): () => void;
};

declare global {
  interface Window {
    mnemosyne: Bridge;
  }
}

/** ApiError carries the status so callers can tell a bad slug from a crash. */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

/**
 * One request to the daemon, with the error shape the API promises unwrapped
 * into an exception. Callers get typed data or a throw — never a status code to
 * remember to check.
 */
export async function api<T>(method: string, path: string, body?: unknown): Promise<T> {
  const { status, body: payload } = await window.mnemosyne.request(method, path, body);

  if (status < 200 || status >= 300) {
    const message =
      payload && typeof payload === "object" && "error" in payload
        ? String((payload as { error: unknown }).error)
        : `request failed with ${status}`;
    throw new ApiError(status, message);
  }
  return payload as T;
}

export const bridge = {
  info: () => window.mnemosyne.info(),
  reveal: (target: string) => window.mnemosyne.reveal(target),
  pickDirectory: () => window.mnemosyne.pickDirectory(),
  installBinary: () =>
    window.mnemosyne.installBinary
      ? window.mnemosyne.installBinary()
      : Promise.reject(new Error("installBinary not supported in this environment")),
  onEvent: (handler: (event: ChangeEvent) => void) => window.mnemosyne.onEvent(handler),
  onFatal: (handler: (message: string) => void) => window.mnemosyne.onFatal(handler),
};
