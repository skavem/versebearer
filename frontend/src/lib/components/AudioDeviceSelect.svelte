<script lang="ts">
  // AudioDeviceSelect — правка 4: раньше была выпадашка (PopoverSelect-
  // образная) с разъезжающимися отступами; теперь модалка со списком
  // устройств и максимумом технической информации по каждому — оператору
  // проще выбрать интерфейс, когда видно, на чём он реально заиграет.
  import type { AudioDevice } from "$lib/bindings/changeme/models";
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
    // Перечитываем список: activeSampleRate ("открыто на N Гц") приходит из
    // p.openDeviceSnapshot — устройство реально открывается лениво, при
    // первом Play ПОСЛЕ выбора (см. audio_device.go/ensureDevice), а не
    // прямо в SetDevice. Без перечитывания карточка ещё какое-то время
    // показывала бы частоту ПРЕЖНЕГО перечисления (старого устройства или
    // "ещё не открыто").
    await audioStore.refreshDevices();
  }

  // sampleRatesOf/channelsOf/formatsOf — device.formats приходит как список
  // отдельных РАЗРЕШЁННЫХ комбинаций частота/каналы/формат (malgo.DataFormat
  // как есть, без агрегации на бэке — audio_devices.go), здесь схлопываем в
  // три независимых множества для компактного отображения: оператору важнее
  // "поддерживает 44100/48000/96000" одной строкой, чем полная матрица
  // комбинаций.
  function sampleRatesOf(d: AudioDevice): number[] {
    return [...new Set((d.formats ?? []).map((f) => f.sampleRate))].sort(
      (a, b) => a - b,
    );
  }
  function channelsOf(d: AudioDevice): number[] {
    return [...new Set((d.formats ?? []).map((f) => f.channels))].sort(
      (a, b) => a - b,
    );
  }
  function formatsOf(d: AudioDevice): string[] {
    return [...new Set((d.formats ?? []).map((f) => f.format))];
  }
</script>

<button
  type="button"
  class="btn btn-outline btn-sm gap-1"
  onclick={() => (open = true)}
  aria-haspopup="dialog"
  aria-label="Устройство вывода звука"
>
  <MuiIcon
    name={selected?.missing ? "speaker_notes_off" : "speaker"}
    style="font-size: 1rem"
  />
  <span class="max-w-48 truncate">
    {selected?.name ?? "Системное по умолчанию"}
  </span>
  {#if selected?.missing}
    <span class="text-error">пропало</span>
  {/if}
</button>

{#if open}
  <div class="modal modal-open">
    <div class="modal-box max-w-2xl">
      <div class="mb-3 flex items-center justify-between">
        <h3 class="text-lg font-bold">Устройство вывода звука</h3>
        <button
          class="btn btn-ghost btn-sm btn-square"
          onclick={() => (open = false)}
          aria-label="Закрыть"
        >
          <MuiIcon name="close" />
        </button>
      </div>

      {#if devices.loading}
        <div class="py-6 text-center opacity-60">Загрузка…</div>
      {:else}
        <ul class="flex flex-col gap-2">
          {#each devices.list as d (d.id)}
            {@const isSelected = d.id === devices.selectedId}
            {@const rates = sampleRatesOf(d)}
            {@const channels = channelsOf(d)}
            {@const formats = formatsOf(d)}
            <li>
              <button
                type="button"
                class={[
                  "flex w-full flex-col gap-1 rounded-lg border-2 p-3 text-left transition-colors",
                  isSelected
                    ? "border-primary bg-primary/5"
                    : "border-base-300 hover:bg-base-200",
                  d.missing && "opacity-70",
                ]}
                onclick={() => pick(d.id)}
              >
                <div class="flex items-center gap-2">
                  <MuiIcon
                    name={d.missing ? "speaker_notes_off" : "speaker"}
                    style="font-size: 1.1rem"
                    classes={isSelected ? "text-primary" : "opacity-70"}
                  />
                  <span class="font-medium">
                    {d.name || "Устройство недоступно"}
                  </span>
                  {#if isSelected}
                    <span class="badge badge-primary badge-sm">выбрано сейчас</span>
                  {/if}
                  {#if d.isDefault && d.id !== ""}
                    <span class="badge badge-ghost badge-sm">системное по умолчанию</span>
                  {/if}
                  {#if d.id === ""}
                    <span class="badge badge-ghost badge-sm">следует за Windows</span>
                  {/if}
                  {#if d.missing}
                    <span class="badge badge-error badge-sm">недоступно</span>
                  {/if}
                </div>

                <!-- Техданные — деградируем мягко: нет Formats, значит
                просто не показываем эти строки, а не рисуем "0 Гц" (правка
                4, явное требование). -->
                {#if d.activeSampleRate}
                  <div class="text-xs text-secondary">
                    сейчас открыто на {d.activeSampleRate} Гц
                  </div>
                {/if}
                {#if rates.length > 0}
                  <div class="text-xs opacity-70">
                    Частоты: {rates.join(", ")} Гц
                  </div>
                {/if}
                {#if channels.length > 0 || formats.length > 0}
                  <div class="text-xs opacity-70">
                    {#if channels.length > 0}
                      Каналы: {channels.join(", ")}
                    {/if}
                    {#if channels.length > 0 && formats.length > 0}
                      {" · "}
                    {/if}
                    {#if formats.length > 0}
                      Формат: {formats.join(", ")}
                    {/if}
                  </div>
                {/if}
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
    <button
      class="modal-backdrop"
      onclick={() => (open = false)}
      aria-label="Закрыть"
    ></button>
  </div>
{/if}

<svelte:window
  onkeydown={(e) => open && e.key === "Escape" && (open = false)}
/>
