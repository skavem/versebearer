<script lang="ts">
  import MuiIcon from "./MuiIcon.svelte";

  let {
    value,
    min,
    max,
    step = 1,
    label,
    unit,
    onChange,
  }: {
    value: number;
    min?: number;
    max?: number;
    step?: number;
    label?: string;
    unit?: string;
    onChange: (n: number) => void;
  } = $props();

  function clamp(n: number): number {
    let v = n;
    if (min !== undefined) v = Math.max(min, v);
    if (max !== undefined) v = Math.min(max, v);
    return v;
  }

  function bump(delta: number) {
    onChange(clamp((value || 0) + delta));
  }

  function handleInput(e: Event) {
    const raw = (e.target as HTMLInputElement).value;
    if (raw.trim() === "" || raw.trim() === "-") return;
    const n = parseInt(raw, 10);
    if (Number.isNaN(n)) return;
    onChange(clamp(n));
  }

  const fieldLabel = $derived(
    [label, unit].filter(Boolean).join(", ") || "Значение",
  );
</script>

<div class="num-stepper">
  {#if label}
    <span class="num-stepper__label">{label}</span>
  {/if}
  <button
    type="button"
    class="num-stepper__btn"
    onclick={() => bump(-step)}
    disabled={min !== undefined && value <= min}
    aria-label="Уменьшить{label ? `: ${label}` : ''}"
  >
    <MuiIcon name="remove" style="font-size: 0.9rem" />
  </button>
  <input
    type="number"
    class="num-stepper__input"
    {min}
    {max}
    {step}
    {value}
    oninput={handleInput}
    aria-label={fieldLabel}
  />
  <button
    type="button"
    class="num-stepper__btn"
    onclick={() => bump(step)}
    disabled={max !== undefined && value >= max}
    aria-label="Увеличить{label ? `: ${label}` : ''}"
  >
    <MuiIcon name="add" style="font-size: 0.9rem" />
  </button>
  {#if unit}
    <span class="num-stepper__unit">{unit}</span>
  {/if}
</div>

<style>
  .num-stepper {
    display: inline-flex;
    align-items: stretch;
    width: fit-content;
    height: 2rem;
    border: 1px solid oklch(var(--b3));
    border-radius: 0.5rem;
    background-color: oklch(var(--b1));
    overflow: hidden;
    transition: border-color 0.15s;
  }
  .num-stepper:focus-within {
    border-color: oklch(var(--p));
  }

  .num-stepper__label {
    display: flex;
    align-items: center;
    padding: 0 0.5rem;
    border-right: 1px solid oklch(var(--b3));
    background-color: oklch(var(--b2) / 0.6);
    font-size: 0.7rem;
    color: oklch(var(--bc) / 0.6);
    white-space: nowrap;
  }

  .num-stepper__btn {
    display: flex;
    flex: none;
    align-items: center;
    justify-content: center;
    width: 1.5rem;
    border: none;
    background: transparent;
    color: oklch(var(--bc) / 0.55);
    cursor: pointer;
    transition:
      background-color 0.12s,
      color 0.12s;
  }
  .num-stepper__btn:hover:not(:disabled) {
    background-color: oklch(var(--b2));
    color: oklch(var(--bc));
  }
  .num-stepper__btn:active:not(:disabled) {
    background-color: oklch(var(--b3));
  }
  .num-stepper__btn:disabled {
    cursor: not-allowed;
    opacity: 0.3;
  }

  .num-stepper__input {
    width: 2.75rem;
    min-width: 0;
    border: none;
    background: transparent;
    outline: none;
    text-align: center;
    font-size: 0.8125rem;
    font-variant-numeric: tabular-nums;
    color: inherit;
    appearance: none;
    -moz-appearance: textfield;
  }
  .num-stepper__input::-webkit-inner-spin-button,
  .num-stepper__input::-webkit-outer-spin-button {
    appearance: none;
    -webkit-appearance: none;
    margin: 0;
  }

  .num-stepper__unit {
    display: flex;
    align-items: center;
    padding: 0 0.5rem 0 0.125rem;
    font-size: 0.7rem;
    color: oklch(var(--bc) / 0.4);
    white-space: nowrap;
  }
</style>
