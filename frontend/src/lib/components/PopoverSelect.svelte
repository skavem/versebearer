<script lang="ts" generics="V extends string | number">
  import type { ClassValue } from "clsx";
  import MuiIcon from "./MuiIcon.svelte";
  import type { MuiIconNames } from "./MuiIcon";

  let {
    value,
    options,
    onChange,
    ariaLabel,
    icon,
    classes,
  }: {
    value: V;
    options: { value: V; label: string; hint?: string }[];
    onChange: (v: V) => void;
    ariaLabel: string;
    icon?: MuiIconNames;
    classes?: ClassValue;
  } = $props();

  let open = $state(false);

  const selected = $derived(options.find((o) => o.value === value));

  function pick(v: V) {
    onChange(v);
    open = false;
  }
</script>

<div class={["dd", classes]}>
  <button
    type="button"
    class="dd-trigger"
    onclick={() => (open = !open)}
    aria-haspopup="listbox"
    aria-expanded={open}
    aria-label={ariaLabel}
  >
    {#if icon}
      <MuiIcon name={icon} style="font-size: 1rem" />
    {/if}
    <span class="dd-trigger__name">{selected?.label ?? ""}</span>
    <MuiIcon name="expand_more" style="font-size: 1.1rem" />
  </button>

  {#if open}
    <button class="dd-backdrop" onclick={() => (open = false)} aria-label="Закрыть"></button>
    <div class="dd-panel" role="listbox" aria-label={ariaLabel}>
      {#each options as opt (opt.value)}
        <button
          type="button"
          class="dd-option {opt.value === value ? 'dd-option--active' : ''}"
          onclick={() => pick(opt.value)}
          role="option"
          aria-selected={opt.value === value}
        >
          {#if opt.value === value}
            <MuiIcon name="check" style="font-size: 0.9rem" />
          {:else}
            <span class="dd-spacer"></span>
          {/if}
          <span class="dd-option__name">{opt.label}</span>
          {#if opt.hint}
            <span class="dd-option__hint">{opt.hint}</span>
          {/if}
        </button>
      {/each}
    </div>
  {/if}
</div>

<svelte:window onkeydown={(e) => open && e.key === "Escape" && (open = false)} />
