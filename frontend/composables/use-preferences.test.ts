import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { effectScope, reactive, ref, watch, type EffectScope, type Ref } from "vue";

let scope: EffectScope;
let preferences: Ref<Record<string, unknown>>;
let auth: { token: boolean; isAuthorized: () => boolean };
let serverSettings: Record<string, unknown> | null;
let serverEvent: () => void;
let getSettings: ReturnType<typeof vi.fn>;
let setSettings: ReturnType<typeof vi.fn>;

async function settle() {
  await vi.advanceTimersByTimeAsync(500);
}

async function start(local: Record<string, unknown> = {}) {
  vi.stubGlobal("useLocalStorage", (_key: string, defaults: Record<string, unknown>) => {
    preferences = ref({ ...defaults, ...local });
    return preferences;
  });
  const module = await import("./use-preferences");
  scope.run(() => module.useViewPreferencesSync());
  await settle();
  return module;
}

beforeEach(() => {
  vi.resetModules();
  vi.useFakeTimers();
  scope = effectScope();
  serverSettings = {};
  auth = reactive({ token: true, isAuthorized: () => auth.token });
  getSettings = vi.fn(async () => ({
    data: { item: serverSettings },
    error: false,
  }));
  setSettings = vi.fn(async (settings: Record<string, unknown>) => {
    serverSettings = settings;
    return { error: false };
  });
  vi.stubGlobal("watch", watch);
  vi.stubGlobal("useAuthContext", () => auth);
  vi.stubGlobal("useUserApi", () => ({ user: { getSettings, setSettings } }));
  vi.stubGlobal("useDebounceFn", (callback: () => void, delay: number) => {
    let timer: ReturnType<typeof setTimeout>;
    return () => {
      clearTimeout(timer);
      timer = setTimeout(callback, delay);
    };
  });
  vi.stubGlobal("ServerEvent", { UserMutation: "user" });
  vi.stubGlobal("onServerEvent", (_event: string, callback: () => void) => {
    serverEvent = callback;
  });
});

afterEach(() => {
  scope.stop();
  vi.clearAllTimers();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("account theme preferences", () => {
  it.each([null, {}, { theme: "cinematic" }, { theme: "light" }])(
    "loads server settings on a clean device without writing defaults: %j",
    async settings => {
      serverSettings = settings;
      await start();
      expect(preferences.value.theme).toBe(settings?.theme ?? "homebox");
      expect(setSettings).not.toHaveBeenCalled();
    }
  );

  it("replaces stale browser themes with an implicit legacy default or saved account theme", async () => {
    await start({ theme: "cinematic" });
    expect(preferences.value.theme).toBe("homebox");
    auth.token = false;
    serverSettings = { theme: "light" };
    auth.token = true;
    await settle();
    expect(preferences.value.theme).toBe("light");
    auth.token = false;
    serverSettings = null;
    auth.token = true;
    await settle();
    expect(preferences.value.theme).toBe("homebox");
    expect(setSettings).not.toHaveBeenCalled();
  });

  it("loads a new account's stored cinematic default instead of stale browser preferences", async () => {
    serverSettings = { theme: "cinematic" };
    await start({ theme: "light" });
    expect(preferences.value.theme).toBe("cinematic");
    expect(setSettings).not.toHaveBeenCalled();
  });

  it("persists manual selection and receives theme changes from another device", async () => {
    serverSettings = { theme: "cinematic" };
    await start();
    preferences.value.theme = "light";
    await settle();
    expect(setSettings).toHaveBeenCalledWith(expect.objectContaining({ theme: "light" }));
    serverSettings = { theme: "dark" };
    serverEvent();
    await settle();
    expect(preferences.value.theme).toBe("dark");
    expect(setSettings).toHaveBeenCalledTimes(1);
  });

  it("preserves an intentional theme selection while a snapshot is loading", async () => {
    await start();
    let resolve!: (value: unknown) => void;
    getSettings.mockImplementationOnce(() => new Promise(r => (resolve = r)));
    serverEvent();
    preferences.value.theme = "light";
    resolve({ data: { item: { theme: "cinematic" } }, error: false });
    await settle();
    expect(preferences.value.theme).toBe("light");
    expect(setSettings).toHaveBeenCalledWith(expect.objectContaining({ theme: "light" }));
  });

  it("ignores an old account's delayed snapshot after switching sessions", async () => {
    await start();
    let resolve!: (value: unknown) => void;
    getSettings.mockImplementationOnce(() => new Promise(r => (resolve = r)));
    serverEvent();
    auth.token = false;
    serverSettings = null;
    auth.token = true;
    resolve({ data: { item: { theme: "cinematic" } }, error: false });
    await settle();
    expect(preferences.value.theme).toBe("homebox");
    expect(setSettings).not.toHaveBeenCalled();
  });
});
