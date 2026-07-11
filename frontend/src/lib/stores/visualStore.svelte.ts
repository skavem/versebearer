import {
  DeleteFont,
  DeleteImage,
  ResetBackdrop,
  ResetCoupletStyle,
  ResetVerseStyle,
  GetVisualSettings,
  UpdateBackdrop,
  UpdateCoupletStyle,
  UpdateVerseStyle,
  UploadFont,
  UploadImage,
} from "$lib/bindings/changeme/dbhandler";
import type {
  Backdrop,
  BackdropInput,
  StyleInput,
  VisualStyle,
} from "$lib/bindings/changeme/models";
import type { Font, Image } from "$lib/bindings/changeme/backend/models";

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
  margin: 0,
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
  margin: 0,
  textShadow: "",
};

// Build a full StyleInput from a partial patch (backend treats null as
// "leave unchanged"). Kept in one place so verse/couplet stay in sync.
function toStyleInput(patch: Partial<StyleInput>): StyleInput {
  return {
    bgColor: patch.bgColor ?? null,
    bgOpacity: patch.bgOpacity ?? null,
    textColor: patch.textColor ?? null,
    fontId: patch.fontId ?? null,
    borderColor: patch.borderColor ?? null,
    borderWidth: patch.borderWidth ?? null,
    borderRadius: patch.borderRadius ?? null,
    borderStyle: patch.borderStyle ?? null,
    padding: patch.padding ?? null,
    margin: patch.margin ?? null,
    textShadow: patch.textShadow ?? null,
  };
}

function applyPatch(base: VisualStyle, patch: Partial<VisualStyle>): VisualStyle {
  return {
    ...base,
    ...Object.fromEntries(
      Object.entries(patch).filter(([, v]) => v !== null && v !== undefined),
    ),
  } as VisualStyle;
}

function createVisualStore() {
  let verseStyle = $state<VisualStyle>({ ...defaultVerseStyle });
  let coupletStyle = $state<VisualStyle>({ ...defaultCoupletStyle });
  let fonts = $state<Font[]>([]);
  let images = $state<Image[]>([]);
  let backdrop = $state<Backdrop>({ bgType: "none", bgGradient: "", bgImageId: null });
  let loaded = $state(false);

  async function reload() {
    const s = await GetVisualSettings();
    verseStyle = s.verseStyle;
    coupletStyle = s.coupletStyle;
    backdrop = s.backdrop;
    fonts = s.fonts ?? [];
    images = s.images ?? [];
    loaded = true;
  }

  reload();

  return {
    // Re-fetch verse/couplet styles from the backend. Called after the active
    // theme changes (switch/create/duplicate/delete) so the editors reflect it.
    reload,

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
    get images() {
      return images;
    },
    get backdrop() {
      return backdrop;
    },
    get loaded() {
      return loaded;
    },

    async updateVerse(patch: Partial<StyleInput>) {
      // optimistic update
      verseStyle = applyPatch(verseStyle, patch as Partial<VisualStyle>);
      const result = await UpdateVerseStyle(toStyleInput(patch));
      verseStyle = result;
    },

    async updateCouplet(patch: Partial<StyleInput>) {
      coupletStyle = applyPatch(coupletStyle, patch as Partial<VisualStyle>);
      const result = await UpdateCoupletStyle(toStyleInput(patch));
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

    // Theme-level always-on backdrop.
    async updateBackdrop(patch: Partial<BackdropInput>) {
      backdrop = {
        ...backdrop,
        ...Object.fromEntries(
          Object.entries(patch).filter(([, v]) => v !== null && v !== undefined),
        ),
      } as Backdrop;
      const input: BackdropInput = {
        bgType: patch.bgType ?? null,
        bgGradient: patch.bgGradient ?? null,
        bgImageId: patch.bgImageId ?? null,
      };
      backdrop = await UpdateBackdrop(input);
    },

    async resetBackdrop() {
      backdrop = await ResetBackdrop();
    },

    async uploadFont(file: File): Promise<string | null> {
      if (file.size > 5 * 1024 * 1024) {
        return "Файл слишком большой (макс. 5 МБ)";
      }
      const ext = file.name.split(".").pop()?.toLowerCase();
      if (ext !== "woff2" && ext !== "ttf") {
        return "Поддерживаются только .woff2 и .ttf";
      }
      const data = await fileToBase64(file);
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

    async uploadImage(file: File): Promise<string | null> {
      if (file.size > 10 * 1024 * 1024) {
        return "Файл слишком большой (макс. 10 МБ)";
      }
      const ext = file.name.split(".").pop()?.toLowerCase();
      const mimeByExt: Record<string, string> = {
        png: "image/png",
        jpg: "image/jpeg",
        jpeg: "image/jpeg",
        webp: "image/webp",
        gif: "image/gif",
      };
      const mimeType = ext ? mimeByExt[ext] : undefined;
      if (!mimeType) {
        return "Поддерживаются .png, .jpg, .webp, .gif";
      }
      const data = await fileToBase64(file);
      const result = await UploadImage(file.name, mimeType, data);
      if (!result) {
        return "Ошибка загрузки картинки";
      }
      const updated = await GetVisualSettings();
      images = updated.images ?? [];
      return null;
    },

    async deleteImage(id: number) {
      await DeleteImage(id);
      const updated = await GetVisualSettings();
      images = updated.images ?? [];
      // refresh backdrop (bgImageId may have been nulled)
      backdrop = updated.backdrop;
    },
  };
}

// base64-encode a file's bytes in chunks to avoid stack overflow on large files.
async function fileToBase64(file: File): Promise<string> {
  const buffer = await file.arrayBuffer();
  const uint8 = new Uint8Array(buffer);
  let b64 = "";
  const chunkSize = 8192;
  for (let i = 0; i < uint8.length; i += chunkSize) {
    b64 += String.fromCharCode.apply(
      null,
      uint8.slice(i, i + chunkSize) as unknown as number[],
    );
  }
  return btoa(b64);
}

export const visualStore = createVisualStore();
