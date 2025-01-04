<script lang="ts">
  import type { ClassValue } from "clsx";
  import type { MuiIconNames } from "./MuiIcon";

  let {
    name,
    classes,
    style,
  }: { name: MuiIconNames; classes?: ClassValue; style?: string } = $props();
</script>

<span class={["material-icons", classes]} {style}>{name}</span>
