import {
  ApplyTheme,
  CreateTheme,
  DeleteTheme,
  DuplicateTheme,
  GetActiveThemeId,
  ListThemes,
  RenameTheme,
} from "$lib/bindings/changeme/dbhandler";
import type { Theme } from "$lib/bindings/changeme/backend/models";
import { visualStore } from "./visualStore.svelte";

function createThemesStore() {
  let themes = $state<Theme[]>([]);
  let activeId = $state<number>(0);
  let loaded = $state(false);

  const active = $derived(themes.find((t) => t.ID === activeId) ?? null);

  async function refresh() {
    themes = (await ListThemes()) ?? [];
    activeId = await GetActiveThemeId();
    loaded = true;
  }

  refresh();

  return {
    get themes() {
      return themes;
    },
    get activeId() {
      return activeId;
    },
    get active() {
      return active;
    },
    get loaded() {
      return loaded;
    },

    // Switch the active theme; re-broadcasts styles and reloads the editors.
    async apply(id: number) {
      if (id === activeId) return;
      themes = (await ApplyTheme(id)) ?? themes;
      activeId = id;
      await visualStore.reload();
    },

    // New theme as a copy of the active one; becomes active for editing.
    async create(name: string) {
      const t = await CreateTheme(name);
      if (t) {
        await refresh();
        await visualStore.reload();
      }
      return t;
    },

    // Copy of a specific theme; becomes active for editing.
    async duplicate(id: number) {
      const t = await DuplicateTheme(id);
      if (t) {
        await refresh();
        await visualStore.reload();
      }
      return t;
    },

    async rename(id: number, name: string) {
      themes = (await RenameTheme(id, name)) ?? themes;
    },

    // Delete a theme (default theme is protected backend-side). If the active
    // theme is removed the backend falls back to the default one.
    async remove(id: number) {
      themes = (await DeleteTheme(id)) ?? themes;
      activeId = await GetActiveThemeId();
      await visualStore.reload();
    },
  };
}

export const themesStore = createThemesStore();
