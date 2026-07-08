import { browser } from "$app/environment";

export type ThemeMode = "light" | "dark" | "auto";

// DaisyUI theme names defined in tailwind.config.js.
const LIGHT_THEME = "mytheme";
const DARK_THEME = "mydark";
const STORAGE_KEY = "vb-theme-mode";

const isThemeMode = (v: unknown): v is ThemeMode =>
  v === "light" || v === "dark" || v === "auto";

const prefersDark = () =>
  browser && window.matchMedia("(prefers-color-scheme: dark)").matches;

const createSettingsStore = () => {
  // User choice: light / dark / auto (follows the OS). Persisted locally —
  // the operator UI theme is device-local and does NOT affect projector output.
  let mode = $state<ThemeMode>("auto");
  // Reactive mirror of the OS preference, kept live by watchSystem().
  let systemDark = $state(false);

  if (browser) {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (isThemeMode(saved)) mode = saved;
    systemDark = prefersDark();
  }

  const isDark = $derived(mode === "dark" || (mode === "auto" && systemDark));
  const themeName = $derived(isDark ? DARK_THEME : LIGHT_THEME);

  return {
    get mode() {
      return mode;
    },
    setMode(m: ThemeMode) {
      mode = m;
      if (browser) localStorage.setItem(STORAGE_KEY, m);
    },
    get isDark() {
      return isDark;
    },
    get themeName() {
      return themeName;
    },
    // Wire once from the root layout. Tracks the OS light/dark preference so
    // "auto" mode flips live. Returns a cleanup function for the $effect.
    watchSystem() {
      if (!browser) return () => {};
      const mq = window.matchMedia("(prefers-color-scheme: dark)");
      systemDark = mq.matches;
      const handler = (e: MediaQueryListEvent) => {
        systemDark = e.matches;
      };
      mq.addEventListener("change", handler);
      return () => mq.removeEventListener("change", handler);
    },
  };
};

export const settingsStore = createSettingsStore();
