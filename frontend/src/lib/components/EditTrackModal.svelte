<script lang="ts">
  import type { AudioTrack } from "$lib/bindings/changeme/backend/models";
  import { audioStore } from "$lib/stores/audioStore.svelte";
  import { formatMinSec } from "$lib/timeFormat";
  import MuiIcon from "./MuiIcon.svelte";

  let { track = $bindable(null) }: { track: AudioTrack | null } = $props();

  let title = $state("");
  let artist = $state("");
  let trimStartMs = $state(0);
  let trimEndMs = $state(0);
  let gainDb = $state(0);

  // trimStartError/trimEndError — ревью: minSecToMs раньше тихо давала 0 на
  // неразобранном вводе ("15" вместо "0:15"), поле оставалось "неконтроли-
  // руемым" (текст на экране не совпадал с фактическим trimStartMs/
  // trimEndMs), оператор сохранял и на служении трек стартовал с тишины.
  // Теперь неразобранный ввод НЕ подменяется на 0 — сохраняется прежнее
  // значение и показывается ошибка рядом с полем.
  let trimStartError = $state("");
  let trimEndError = $state("");

  $effect(() => {
    if (track) {
      title = track.title;
      artist = track.artist;
      trimStartMs = track.trimStartMs;
      trimEndMs = track.trimEndMs;
      gainDb = track.gainDb;
      trimStartError = "";
      trimEndError = "";
    }
  });

  // Границы обрезки оператор задаёт на слух, поэтому поля — в "м:сс"
  // (formatMinSec, $lib/timeFormat), а не в сырых миллисекундах (план,
  // этап 5). Обратный разбор — здесь же, рядом с полями, которые его ждут.
  //
  // Принимает "м:сс" И просто "сс" (сырые секунды без минут — оператору
  // проще ввести "15", чем "0:15"). null — вход не разобран: вызывающий
  // обязан показать ошибку и НЕ подменять текущее значение на 0 (см.
  // trimStartError/trimEndError выше).
  function minSecToMs(text: string): number | null {
    const trimmed = text.trim();
    const withColon = trimmed.match(/^(\d+):([0-5]?\d)$/);
    if (withColon) {
      const min = parseInt(withColon[1], 10);
      const sec = parseInt(withColon[2], 10);
      return (min * 60 + sec) * 1000;
    }
    const secondsOnly = trimmed.match(/^\d+$/);
    if (secondsOnly) {
      return parseInt(trimmed, 10) * 1000;
    }
    return null;
  }

  function onTrimStartChange(e: Event & { currentTarget: HTMLInputElement }) {
    const parsed = minSecToMs(e.currentTarget.value);
    if (parsed === null) {
      trimStartError = "Формат: м:сс или секунды";
      e.currentTarget.value = formatMinSec(trimStartMs); // не даём неверному тексту зависнуть в поле
      return;
    }
    trimStartError = "";
    trimStartMs = parsed;
  }

  function onTrimEndChange(e: Event & { currentTarget: HTMLInputElement }) {
    const parsed = minSecToMs(e.currentTarget.value);
    if (parsed === null) {
      trimEndError = "Формат: м:сс или секунды";
      e.currentTarget.value = formatMinSec(trimEndMs);
      return;
    }
    trimEndError = "";
    trimEndMs = parsed;
  }

  // invalidTrimRange — план: TrimEndMs<=TrimStartMs даёт trimmedDurationMs()
  // == 0 -> beep.Take(0) -> трек не звучит и мгновенно отдаёт done, с
  // AutoAdvance+Loop плейлист пролетит по кругу за секунду. TrimEndMs==0
  // значит "до конца файла" — не граница, проверять нечего. Источник истины
  // — бэк (UpdateTrack, audio_service.go); эта проверка — только для
  // мгновенной обратной связи до сохранения.
  const invalidTrimRange = $derived(trimEndMs > 0 && trimEndMs <= trimStartMs);
  const canSave = $derived(!trimStartError && !trimEndError && !invalidTrimRange);

  // «Взять текущую позицию» доступна, только пока играет (или на паузе)
  // именно ЭТОТ трек: PlayerState.PositionMs считается ОТ TrimStartMs
  // (audio-playlist-implementation.md, этап 5: "прогресс считается от
  // TrimStartMs"), поэтому абсолютная позиция в исходном файле — это
  // ИСХОДНЫЙ (ещё не сохранённый правкой) track.trimStartMs плюс текущий
  // PositionMs, а не сам сырой PositionMs и не редактируемый trimStartMs.
  const player = $derived(audioStore.player.state);
  const isThisTrackLoaded = $derived(!!track && player?.trackId === track.ID);
  const currentAbsoluteMs = $derived(
    isThisTrackLoaded && track
      ? track.trimStartMs + (player?.positionMs ?? 0)
      : null,
  );

  const close = () => {
    track = null;
  };

  const save = async () => {
    if (!track) return;
    const updated = await audioStore.update(track.ID, {
      title: title.trim(),
      artist: artist.trim(),
      trimStartMs,
      trimEndMs,
      gainDb,
    });
    // Отказ (audioStore.update ловит его сам и кладёт в audioStore.error) —
    // не закрываем модалку молча: закрытие выглядело бы как «сохранено».
    if (updated) close();
  };
</script>

<svelte:window onkeydown={(e) => track && e.key === "Escape" && close()} />

{#if track}
  <div class="modal modal-open">
    <div class="modal-box">
      <div class="mb-2 flex items-center justify-between">
        <h3 class="text-lg font-bold">Правка фонограммы</h3>
        <button
          class="btn btn-ghost btn-sm btn-square"
          onclick={close}
          aria-label="Закрыть"
        >
          <MuiIcon name="close" />
        </button>
      </div>

      {#if isThisTrackLoaded}
        <!-- Trim применяется только при следующем запуске трека, НЕ на
        лету (см. audio_library.go/UpdateTrack): пересборка цепочки
        воспроизведения "как при перемотке" посреди эфира — риск щелчка и
        прыжка позиции. Оператор должен это знать до сохранения, не после. -->
        <div class="alert alert-warning mb-2 py-2 text-sm">
          <MuiIcon name="info" style="font-size: 1.1rem" />
          <span>Трек сейчас в эфире — обрезка применится со следующего запуска</span>
        </div>
      {/if}

      <div class="flex flex-col gap-3">
        <label class="form-control">
          <div class="label py-1">
            <span class="label-text font-medium">Название</span>
          </div>
          <input
            type="text"
            bind:value={title}
            class="input input-bordered w-full"
          />
        </label>

        <label class="form-control">
          <div class="label py-1">
            <span class="label-text font-medium">Исполнитель</span>
          </div>
          <input
            type="text"
            bind:value={artist}
            class="input input-bordered w-full"
          />
        </label>

        <div class="flex gap-2">
          <label class="form-control flex-1">
            <div class="label py-1">
              <span class="label-text text-xs">Начало, м:сс</span>
            </div>
            <div class="join w-full">
              <input
                type="text"
                value={formatMinSec(trimStartMs)}
                onchange={onTrimStartChange}
                placeholder="0:00"
                class="input input-sm input-bordered join-item w-full"
              />
              <button
                type="button"
                class="btn btn-sm join-item"
                disabled={currentAbsoluteMs === null}
                title="Взять текущую позицию воспроизведения"
                onclick={() => {
                  if (currentAbsoluteMs !== null) trimStartMs = currentAbsoluteMs;
                }}
              >
                <MuiIcon name="my_location" style="font-size: 1rem" />
              </button>
            </div>
            {#if trimStartError}
              <div class="label py-0.5">
                <span class="label-text-alt text-error">{trimStartError}</span>
              </div>
            {/if}
          </label>
          <label class="form-control flex-1">
            <div class="label py-1">
              <span class="label-text text-xs">Конец, м:сс (0 = до конца)</span>
            </div>
            <div class="join w-full">
              <input
                type="text"
                value={formatMinSec(trimEndMs)}
                onchange={onTrimEndChange}
                placeholder="0:00"
                class="input input-sm input-bordered join-item w-full"
              />
              <button
                type="button"
                class="btn btn-sm join-item"
                disabled={currentAbsoluteMs === null}
                title="Взять текущую позицию воспроизведения"
                onclick={() => {
                  if (currentAbsoluteMs !== null) trimEndMs = currentAbsoluteMs;
                }}
              >
                <MuiIcon name="my_location" style="font-size: 1rem" />
              </button>
            </div>
            {#if trimEndError}
              <div class="label py-0.5">
                <span class="label-text-alt text-error">{trimEndError}</span>
              </div>
            {/if}
          </label>
        </div>

        {#if !trimStartError && !trimEndError && invalidTrimRange}
          <div class="text-xs text-error">
            Конец обрезки должен быть позже начала.
          </div>
        {/if}

        <label class="form-control">
          <div class="label py-1">
            <span class="label-text font-medium">Громкость</span>
            <span class="label-text-alt font-mono"
              >{gainDb > 0 ? "+" : ""}{gainDb.toFixed(1)} дБ</span
            >
          </div>
          <input
            type="range"
            step="0.5"
            min="-12"
            max="12"
            bind:value={gainDb}
            class="range range-sm"
          />
        </label>
      </div>

      <div class="modal-action">
        <button class="btn btn-ghost" onclick={close}>Отмена</button>
        <button class="btn btn-neutral" disabled={!canSave} onclick={save}>
          <MuiIcon name="save" />
          Сохранить
        </button>
      </div>
    </div>
    <button class="modal-backdrop" onclick={close} aria-label="Закрыть"></button>
  </div>
{/if}
