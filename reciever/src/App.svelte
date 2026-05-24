<script lang="ts">
  import ShownCouplet from "./lib/components/ShownCouplet.svelte";
  import ShownVerse from "./lib/components/ShownVerse.svelte";
  import type { IFont, IShownCouplet, IShownVerse, IVisualStyle } from "./types";

  const defaultVerseStyle: IVisualStyle = {
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

  const defaultCoupletStyle: IVisualStyle = {
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

  let verse = $state<IShownVerse | null>(null);
  let couplet = $state<IShownCouplet | null>(null);
  let qr = $state<boolean>(false);
  let verseStyle = $state<IVisualStyle>({ ...defaultVerseStyle });
  let coupletStyle = $state<IVisualStyle>({ ...defaultCoupletStyle });
  let fonts = $state<IFont[]>([]);

  function cssSafe(s: string): string {
    return s.replace(/[\\"]/g, "\\$&").replace(/\s/g, " ").replace(/[;{}]/g, "");
  }

  function injectFonts(fontList: IFont[]) {
    let styleEl = document.getElementById("dynamic-fonts");
    if (!styleEl) {
      styleEl = document.createElement("style");
      styleEl.id = "dynamic-fonts";
      document.head.appendChild(styleEl);
    }
    styleEl.textContent = fontList
      .map((f) => {
        const ext = f.mimeType === "font/woff2" || f.mimeType === "application/font-woff2" ? "woff2" : "ttf";
        return `@font-face { font-family: "${cssSafe(f.name)}"; src: url("/font/${f.ID}.${ext}") format("${ext === "woff2" ? "woff2" : "truetype"}"); }`;
      })
      .join("\n");
  }

  function mergeStyle(base: IVisualStyle, patch: Partial<IVisualStyle>): IVisualStyle {
    return { ...base, ...patch };
  }

  $effect(() => {
    const sse = new EventSource("/sse?stream=main");
    sse.onmessage = (event) => {
      const data = JSON.parse(event.data);
      console.log(data);
      switch (data.type) {
        case "show_verse":
          verse = data.verse;
          break;
        case "show_couplet":
          couplet = data.couplet;
          break;
        case "hide_verse":
          verse = null;
          break;
        case "hide_couplet":
          couplet = null;
          break;
        case "hide_qr":
          qr = false;
          break;
        case "show_qr":
          qr = true;
          break;
        case "sync":
          verse = data.verse;
          couplet = data.couplet;
          qr = data.qr;
          if (data.verseStyle) verseStyle = data.verseStyle;
          if (data.coupletStyle) coupletStyle = data.coupletStyle;
          if (data.fonts) {
            fonts = data.fonts;
            injectFonts(data.fonts);
          }
          break;
        case "style_update":
          if (data.target === "verse" && data.style) {
            verseStyle = mergeStyle(verseStyle, data.style);
          } else if (data.target === "couplet" && data.style) {
            coupletStyle = mergeStyle(coupletStyle, data.style);
          }
          break;
        case "fonts_changed":
          if (data.fonts) {
            fonts = data.fonts;
            injectFonts(data.fonts);
          }
          break;
      }
    };
  });
</script>

<ShownVerse {verse} style={verseStyle} {fonts} />
<ShownCouplet {couplet} {qr} style={coupletStyle} {fonts} />
