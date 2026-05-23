import {
  DeleteFont,
  GetVisualSettings,
  ResetCoupletStyle,
  ResetVerseStyle,
  UpdateCoupletStyle,
  UpdateVerseStyle,
  UploadFont,
} from "$lib/bindings/changeme/dbhandler";
import type { StyleInput, VisualSettings, VisualStyle } from "$lib/bindings/changeme/models";
import type { Font } from "$lib/bindings/changeme/backend/models";

const defaultVerseStyle: VisualStyle = {
  bgColor: "#000000",
  bgOpacity: 0.95,
  textColor: "#ffffff",
  fontId: null,
  borderColor: "#000000",
  borderWidth: 0,
  borderRadius: 16,
  borderStyle: "solid",
  padding: 32,
  textShadow: "",
};

const defaultCoupletStyle: VisualStyle = {
  bgColor: "#000000",
  bgOpacity: 0.95,
  textColor: "#ffffff",
  fontId: null,
  borderColor: "#000000",
  borderWidth: 0,
  borderRadius: 0,
  borderStyle: "solid",
  padding: 64,
  textShadow: "",
};

function createVisualStore() {
  let verseStyle = $state<VisualStyle>({ ...defaultVerseStyle });
  let coupletStyle = $state<VisualStyle>({ ...defaultCoupletStyle });
  let fonts = $state<Font[]>([]);
  let loaded = $state(false);

  GetVisualSettings().then((s: VisualSettings) => {
    verseStyle = s.verseStyle;
    coupletStyle = s.coupletStyle;
    fonts = s.fonts ?? [];
    loaded = true;
  });

  return {
    get verseStyle() {
      return verseStyle;
    },
    set verseStyle(v: VisualStyle) {
      verseStyle = v;
    },
    get coupletStyle() {
      return coupletStyle;
    },
    set coupletStyle(v: VisualStyle) {
      coupletStyle = v;
    },
    get fonts() {
      return fonts;
    },
    set fonts(v: Font[]) {
      fonts = v;
    },
    get loaded() {
      return loaded;
    },

    async updateVerse(patch: Partial<StyleInput>) {
      const input: StyleInput = {
        bgColor: patch.bgColor ?? null,
        bgOpacity: patch.bgOpacity ?? null,
        textColor: patch.textColor ?? null,
        fontId: patch.fontId ?? null,
        borderColor: patch.borderColor ?? null,
        borderWidth: patch.borderWidth ?? null,
        borderRadius: patch.borderRadius ?? null,
        borderStyle: patch.borderStyle ?? null,
        padding: patch.padding ?? null,
        textShadow: patch.textShadow ?? null,
      };
      // optimistic update
      verseStyle = { ...verseStyle, ...Object.fromEntries(Object.entries(patch).filter(([, v]) => v !== null && v !== undefined)) } as VisualStyle;
      const result = await UpdateVerseStyle(input);
      verseStyle = result;
    },

    async updateCouplet(patch: Partial<StyleInput>) {
      const input: StyleInput = {
        bgColor: patch.bgColor ?? null,
        bgOpacity: patch.bgOpacity ?? null,
        textColor: patch.textColor ?? null,
        fontId: patch.fontId ?? null,
        borderColor: patch.borderColor ?? null,
        borderWidth: patch.borderWidth ?? null,
        borderRadius: patch.borderRadius ?? null,
        borderStyle: patch.borderStyle ?? null,
        padding: patch.padding ?? null,
        textShadow: patch.textShadow ?? null,
      };
      coupletStyle = { ...coupletStyle, ...Object.fromEntries(Object.entries(patch).filter(([, v]) => v !== null && v !== undefined)) } as VisualStyle;
      const result = await UpdateCoupletStyle(input);
      coupletStyle = result;
    },

    async resetVerse() {
      const result = await ResetVerseStyle();
      verseStyle = result;
    },

    async resetCouplet() {
      const result = await ResetCoupletStyle();
      coupletStyle = result;
    },

    async uploadFont(file: File): Promise<string | null> {
      if (file.size > 5 * 1024 * 1024) {
        return "Файл слишком большой (макс. 5 МБ)";
      }
      const ext = file.name.split(".").pop()?.toLowerCase();
      if (ext !== "woff2" && ext !== "ttf") {
        return "Поддерживаются только .woff2 и .ttf";
      }
      const buffer = await file.arrayBuffer();
      const uint8 = new Uint8Array(buffer);
      // base64 encode in chunks to avoid stack overflow on large files
      let b64 = "";
      const chunkSize = 8192;
      for (let i = 0; i < uint8.length; i += chunkSize) {
        b64 += String.fromCharCode.apply(null, uint8.slice(i, i + chunkSize) as unknown as number[]);
      }
      const data = btoa(b64);
      const mimeType = ext === "woff2" ? "font/woff2" : "font/ttf";
      const result = await UploadFont(file.name, mimeType, data);
      if (!result) {
        return "Ошибка загрузки шрифта";
      }
      const updated = await GetVisualSettings();
      fonts = updated.fonts ?? [];
      return null;
    },

    async deleteFont(id: number) {
      await DeleteFont(id);
      const updated = await GetVisualSettings();
      fonts = updated.fonts ?? [];
      // refresh styles (fontId may have been nulled)
      verseStyle = updated.verseStyle;
      coupletStyle = updated.coupletStyle;
    },
  };
}

export const visualStore = createVisualStore();
