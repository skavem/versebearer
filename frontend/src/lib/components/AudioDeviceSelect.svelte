<script lang="ts">
  // AudioDeviceSelect — по образцу PopoverSelect.svelte, но со своей разметкой
  // опций: PopoverSelect рассчитан на плоский список value/label/hint, а
  // здесь нужно ещё подсветить "пропавшее" устройство (Missing) отдельным
  // цветом/иконкой — обычный hint для этого недостаточно нагляден.
  import { audioStore } from "$lib/stores/audioStore.svelte";
  import MuiIcon from "./MuiIcon.svelte";

  const devices = $derived(audioStore.devices);
  const selected = $derived(
    devices.list.find((d) => d.id === devices.selectedId) ?? null,
  );

  let open = $state(false);

  async function pick(id: string) {
    open = false;
    await audioStore.setDevice(id);
  }
</script>

<div class="dd">
  <button
    type="button"
    class="dd-trigger"
    onclick={() => (open = !open)}
    aria-haspopup="listbox"
    aria-expanded={open}
    aria-label="Устройство вывода звука"
  >
    <MuiIcon
      name={selected?.missing ? "speaker_notes_off" : "speaker"}
      style="font-size: 1rem"
    />
    <span class="dd-trigger__name">
      {selected?.name ?? "Системное по умолчанию"}
      {#if selected?.missing}
        <span class="text-error">(пропало)</span>
      {/if}
    </span>
    <MuiIcon name="expand_more" style="font-size: 1.1rem" />
  </button>

  {#if open}
    <button
      class="dd-backdrop"
      onclick={() => (open = false)}
      aria-label="Закрыть"
    ></button>
    <div class="dd-panel" role="listbox" aria-label="Устройство вывода звука">
      {#if devices.loading}
        <div class="dd-option opacity-60">Загрузка…</div>
      {/if}
      {#each devices.list as d (d.id)}
        <button
          type="button"
          class="dd-option {d.id === devices.selectedId
            ? 'dd-option--active'
            : ''}"
          onclick={() => pick(d.id)}
          role="option"
          aria-selected={d.id === devices.selectedId}
        >
          {#if d.id === devices.selectedId}
            <MuiIcon name="check" style="font-size: 0.9rem" />
          {:else}
            <span class="dd-spacer"></span>
          {/if}
          <span class="dd-option__name"
            >{d.name || "Устройство недоступно"}</span
          >
          {#if d.missing}
            <span class="dd-option__hint text-error">пропало</span>
          {:else if d.isDefault && d.id !== ""}
            <span class="dd-option__hint">системное</span>
          {/if}
        </button>
      {/each}
    </div>
  {/if}
</div>

<svelte:window
  onkeydown={(e) => open && e.key === "Escape" && (open = false)}
/>
