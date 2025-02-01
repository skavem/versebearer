<script lang="ts">
  import ShownCouplet from "./lib/components/ShownCouplet.svelte";
  import ShownVerse from "./lib/components/ShownVerse.svelte";
  import type { IShownCouplet, IShownVerse } from "./types";

  let verse = $state<IShownVerse | null>(null);
  let couplet = $state<IShownCouplet | null>(null);
  let qr = $state<boolean>(false);
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
          break;
      }
    };
  });
</script>

<ShownVerse {verse} />
<ShownCouplet {couplet} {qr} />
