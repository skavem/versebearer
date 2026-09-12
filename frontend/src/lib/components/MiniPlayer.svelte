<script lang="ts">
  // MiniPlayer — транспорт вкладки «Звук» (план, этап 6). Не выбирает трек
  // сам: старт конкретного элемента остаётся за медиатекой/плейлистом (И5,
  // "ровно один активный трек" — выбор трека живёт там, где он один), здесь
  // только управление уже загруженным: пауза/резюме, перемотка, громкость,
  // стоп с фейдом и индикатор уровня.
  import { audioStore } from "$lib/stores/audioStore.svelte";
  import { formatMinSec } from "$lib/timeFormat";
  import MuiIcon from "./MuiIcon.svelte";

  const player = $derived(audioStore.player.state);
  const tracks = $derived(audioStore.tracks);
  const isIdle = $derived(!player || player.status === "idle");
  const currentTrack = $derived(
    player ? (tracks.list.find((t) => t.ID === player.trackId) ?? null) : null,
  );

  // seekDraft — позиция во время перетаскивания ползунка прогресса.
  // Перекрывает player.positionMs, пока оператор тащит: без этого опрос раз
  // в 500 мс (audioStore.startPolling) выдёргивал бы ползунок обратно под
  // пальцем на каждый тик.
  let seekDraft = $state<number | null>(null);
  const durationMs = $derived(player?.durationMs ?? 0);
  const shownPositionMs = $derived(
    seekDraft ?? Math.min(player?.positionMs ?? 0, durationMs),
  );
  const remainingMs = $derived(Math.max(0, durationMs - shownPositionMs));
  // Подсветка последних 10 секунд — план, этап 6.
  const remainingSoon = $derived(
    !isIdle && remainingMs > 0 && remainingMs <= 10_000,
  );

  function onSeekInput(e: Event & { currentTarget: HTMLInputElement }) {
    seekDraft = Number(e.currentTarget.value);
  }

  function onSeekChange(e: Event & { currentTarget: HTMLInputElement }) {
    const ms = Number(e.currentTarget.value);
    seekDraft = null;
    audioStore.seek(ms);
    // Ловушка (план, этап 6): input[type=range] — это HTMLInputElement,
    // isTypingTarget (keyboard.ts) молча глушит Esc/Space/стрелки, пока
    // фокус остался на ползунке. blur() сразу после действия чинит это
    // локально, не трогая общий keyboard.ts.
    //
    // ⚠️ Этого blur() здесь недостаточно: `change` у range не возникает при
    // клике ровно по бегунку без изменения значения (щёлкнули туда, где он
    // уже стоял) — фокус тогда остаётся на ползунке, и isTypingTarget глушит
    // ВСЮ карту клавиш вкладки. onpointerup на самом инпуте (см. разметку
    // ниже) — подстраховка на отпускание указателя независимо от того,
    // сработал ли `change`.
    e.currentTarget.blur();
  }

  function onVolumeInput(e: Event & { currentTarget: HTMLInputElement }) {
    audioStore.setVolume(Number(e.currentTarget.value) / 100);
  }

  function onVolumeChange(e: Event & { currentTarget: HTMLInputElement }) {
    e.currentTarget.blur(); // та же ловушка, см. onSeekChange
  }

  // peakHold — экспоненциально спадающий пик-холд поверх текущего уровня
  // (план, этап 6). Peak уже приходит с обычным опросом PlayerState (этап
  // 2) — здесь только локальная анимация между двумя опросами, никакого
  // нового канала к бэку.
  //
  // ⚠️ Эффект читает isIdle ПЕРВОЙ строкой — это делает его реактивной
  // зависимостью: пока ничего не играет, requestAnimationFrame вообще не
  // планируется (60 раз/с впустую в простое — не нужно), а как только
  // isIdle меняется, Svelte сам отменяет предыдущий RAF (cleanup) и
  // перезапускает эффект.
  let peakHold = $state(0);
  $effect(() => {
    if (isIdle) {
      peakHold = 0;
      return;
    }
    let raf = 0;
    let last = performance.now();
    const decayPerSec = 2.2;
    const tick = (now: number) => {
      const dt = (now - last) / 1000;
      last = now;
      const current = audioStore.player.state?.peak ?? 0;
      peakHold = Math.max(current, peakHold * Math.exp(-decayPerSec * dt));
      raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  });
  const peakPct = $derived(Math.min(100, (player?.peak ?? 0) * 100));
  const peakHoldPct = $derived(Math.min(100, peakHold * 100));
  const volumePct = $derived(Math.round((player?.volume ?? 1) * 100));
</script>

<div class="flex flex-none flex-col gap-2 rounded-lg border border-base-300 bg-base-100 p-3">
  <div class="flex items-center gap-3">
    <div class="flex items-center gap-1">
      <button
        class="btn btn-ghost btn-circle btn-sm"
        disabled={isIdle || !player?.playlistId}
        onclick={() => audioStore.prev()}
        title="Предыдущий"
      >
        <MuiIcon name="skip_previous" style="font-size: 1.3rem" />
      </button>
      <button
        class="btn btn-circle btn-neutral"
        disabled={isIdle}
        onclick={() => audioStore.toggle()}
        title={player?.status === "playing" ? "Пауза" : "Продолжить"}
      >
        <MuiIcon
          name={player?.status === "playing" ? "pause" : "play_arrow"}
          style="font-size: 1.6rem"
        />
      </button>
      <button
        class="btn btn-ghost btn-circle btn-sm"
        disabled={isIdle || !player?.playlistId}
        onclick={() => audioStore.next()}
        title="Следующий"
      >
        <MuiIcon name="skip_next" style="font-size: 1.3rem" />
      </button>
      <button
        class="btn btn-outline btn-neutral btn-circle btn-sm"
        disabled={isIdle}
        onclick={() => audioStore.fadeOutStop()}
        title="Стоп (с фейдом)"
      >
        <MuiIcon name="stop" style="font-size: 1.2rem" />
      </button>
    </div>

    <div class="min-w-0 flex-1">
      <div class="truncate text-sm font-semibold">
        {isIdle ? "Ничего не играет" : currentTrack?.title || currentTrack?.fileName || "…"}
      </div>
      {#if !isIdle && player?.nextTitle}
        <div class="truncate text-xs opacity-60">далее: {player.nextTitle}</div>
      {/if}
    </div>
  </div>

  <div class="flex items-center gap-2">
    <span class="w-10 shrink-0 text-right font-mono text-xs opacity-70"
      >{formatMinSec(shownPositionMs)}</span
    >
    <input
      type="range"
      class="range range-xs flex-1"
      min="0"
      max={Math.max(durationMs, 1)}
      value={shownPositionMs}
      disabled={isIdle}
      oninput={onSeekInput}
      onchange={onSeekChange}
      onpointerup={(e) => e.currentTarget.blur()}
      aria-label="Позиция воспроизведения"
    />
    <span
      class={[
        "w-14 shrink-0 font-mono text-xs",
        remainingSoon ? "font-semibold text-error" : "opacity-70",
      ]}
    >
      -{formatMinSec(remainingMs)}
    </span>
  </div>

  <div class="flex items-center gap-3">
    <div class="flex flex-1 items-center gap-2">
      <MuiIcon
        name={volumePct === 0 ? "volume_off" : "volume_up"}
        style="font-size: 1.1rem"
        classes="opacity-70"
      />
      <input
        type="range"
        class="range range-xs w-32"
        min="0"
        max="100"
        value={volumePct}
        oninput={onVolumeInput}
        onchange={onVolumeChange}
        onpointerup={(e) => e.currentTarget.blur()}
        aria-label="Громкость"
      />
      <span class="w-8 font-mono text-xs opacity-60">{volumePct}%</span>
    </div>

    <div class="flex flex-1 items-center gap-2">
      <MuiIcon name="graphic_eq" style="font-size: 1.1rem" classes="opacity-70" />
      <div class="relative h-2 flex-1 overflow-hidden rounded-full bg-base-300">
        <div
          class={["absolute inset-y-0 left-0 rounded-full", peakPct > 90 ? "bg-error" : "bg-primary"]}
          style:width="{peakPct}%"
          style:transition="width 250ms ease-out"
        ></div>
        <div
          class="absolute inset-y-0 w-[2px] bg-base-content/70"
          style:left="{peakHoldPct}%"
        ></div>
      </div>
    </div>
  </div>
</div>
