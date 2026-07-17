import {
  CreateOutput,
  DeleteOutput,
  GetCurrentScreenID,
  ListOutputs,
  RenameOutput,
  SetOutputScreen,
  SetOutputTheme,
  SetOutputTransparent,
  StartOutput,
  StopOutput,
} from "$lib/bindings/changeme/dbhandler";
import type { Output } from "$lib/bindings/changeme/backend/models";
import { Events, Screens } from "@wailsio/runtime";

type Screen = Screens.Screen;

// Wails window name for an output's projector window — mirrors
// outputWindowName(id) in dbHandler.go ("out-3").
const OUTPUT_WINDOW_RE = /^out-(\d+)$/;

const createOutputStore = () => {
  let outputs = $state<Output[]>([]);
  let monitors = $state<Screen[]>([]);
  let currentScreenID = $state<string>("");
  // Ids of outputs whose projector window is currently open.
  let activeOutputIds = $state<number[]>([]);
  let pendingOutput = $state<Output | null>(null);
  let loaded = $state(false);

  Events.On("current_screen", ({ data }: { data: string }) => {
    currentScreenID = data;
  });
  // The backend emits the Wails window name on close for both the legacy
  // "screen <ID>" windows and the "out-<id>" output windows — only react to
  // the latter here.
  Events.On("screen_closed", ({ data }: { data: string }) => {
    const m = OUTPUT_WINDOW_RE.exec(data);
    if (!m) return;
    const id = Number(m[1]);
    activeOutputIds = activeOutputIds.filter((v) => v !== id);
  });

  function monitorFor(o: Output): Screen | undefined {
    return monitors.find((m) => m.ID === o.screenId);
  }

  function applyStart(o: Output) {
    const m = monitorFor(o);
    if (!m) return;
    StartOutput(o.ID, m.Bounds.X, m.Bounds.Y, m.Bounds.Width, m.Bounds.Height);
    activeOutputIds = [...activeOutputIds, o.ID];
  }

  function applyStop(o: Output) {
    StopOutput(o.ID);
    activeOutputIds = activeOutputIds.filter((v) => v !== o.ID);
  }

  async function refresh() {
    outputs = (await ListOutputs()) ?? [];
    loaded = true;
  }

  return {
    get outputs() {
      return outputs;
    },
    get monitors() {
      return monitors;
    },
    set monitors(v: Screen[]) {
      monitors = v;
    },
    get currentScreenID() {
      return currentScreenID;
    },
    set currentScreenID(v: string) {
      currentScreenID = v;
    },
    get activeOutputIds() {
      return activeOutputIds;
    },
    get pendingOutput() {
      return pendingOutput;
    },
    get loaded() {
      return loaded;
    },

    isActive(o: Output): boolean {
      return activeOutputIds.includes(o.ID);
    },
    monitorFor,

    refresh,

    // New output pointing at the default theme (see Go CreateOutput), no
    // monitor assigned yet.
    async add() {
      const created = await CreateOutput(`Выход ${outputs.length + 1}`);
      if (created) outputs = [...outputs, created];
      return created;
    },

    async rename(id: number, name: string) {
      outputs = (await RenameOutput(id, name)) ?? outputs;
    },

    // Stop the projector window (if open) before deleting the row, so we
    // never leak an orphaned window keyed to an id that no longer exists.
    async remove(id: number) {
      const o = outputs.find((x) => x.ID === id);
      if (o && activeOutputIds.includes(id)) applyStop(o);
      if (pendingOutput?.ID === id) pendingOutput = null;
      outputs = (await DeleteOutput(id)) ?? outputs;
    },

    async setScreen(id: number, screenId: string) {
      outputs = (await SetOutputScreen(id, screenId)) ?? outputs;
    },

    async setTransparent(id: number, v: boolean) {
      outputs = (await SetOutputTransparent(id, v)) ?? outputs;
    },

    async setTheme(id: number, themeId: number) {
      outputs = (await SetOutputTheme(id, themeId)) ?? outputs;
    },

    // Same UX guard as the old monitor-centric store: warn before covering
    // the monitor the operator UI itself lives on.
    requestToggle(o: Output) {
      const active = activeOutputIds.includes(o.ID);
      if (!active && o.screenId !== "" && o.screenId === currentScreenID) {
        pendingOutput = o;
        return;
      }
      if (active) applyStop(o);
      else applyStart(o);
    },
    confirmPending() {
      if (pendingOutput) {
        applyStart(pendingOutput);
        pendingOutput = null;
      }
    },
    cancelPending() {
      pendingOutput = null;
    },

    // Hotkey target (Ctrl+Shift+W, wired in +layout.svelte): toggle the
    // first output assigned to a non-primary monitor — that's "the other
    // screen" the operator isn't looking at, so it's safe to skip the
    // cover-my-own-monitor confirmation. Falls back to the single output
    // when there's exactly one overall (even if it shares the operator's
    // monitor), since a lone output is an unambiguous hotkey target. No-op
    // if the resolved target has no monitor assigned/found.
    toggleSecondary() {
      const nonPrimary = outputs.find((o) => {
        const m = monitorFor(o);
        return m && !m.IsPrimary;
      });
      const target = nonPrimary ?? (outputs.length === 1 ? outputs[0] : undefined);
      if (!target || !monitorFor(target)) return;
      if (activeOutputIds.includes(target.ID)) applyStop(target);
      else applyStart(target);
    },
  };
};

export const outputStore = createOutputStore();
outputStore.refresh();
Screens.GetAll().then((s) => (outputStore.monitors = s));
GetCurrentScreenID().then((id) => (outputStore.currentScreenID = id));
