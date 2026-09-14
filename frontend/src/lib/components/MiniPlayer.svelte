<script lang="ts">
  // MiniPlayer — транспорт вкладки «Звук» (план, этап 6). Не выбирает трек
  // сам: старт конкретного элемента остаётся за медиатекой/плейлистом (И5,
  // "ровно один активный трек" — выбор трека живёт там, где он один), здесь
  // только управление уже загруженным: пауза/резюме, перемотка, громкость,
  // стоп с фейдом и индикатор уровня. Исключение — кнопка play/pause
  // (правка 4, второй раунд): она умеет ЗАПУСТИТЬ выбранный на вкладке
  // трек, если сейчас ничего не загружено, — через
  // audioStore.playOrToggleSelected, ту же функцию, что вызывает Space на
  // вкладке (Audio.svelte), чтобы поведение этих двух мест не расходилось.
  import { audioStore } from "$lib/stores/audioStore.svelte";
  import { formatMinSec } from "$lib/timeFormat";
  import MuiIcon from "./MuiIcon.svelte";

  // selectedItemId — выбор строки плейлиста (Audio.svelte, тот же
  // selectedItemId, которым управляют ArrowUp/ArrowDown и который
  // подсвечивает PlaylistPanel) — второе понятие "выбранного" здесь не
  // заводим, просто читаем то же самое сверху.
  let { selectedItemId = null }: { selectedItemId?: number | null } = $props();

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

  // canPlayPause/playPauseTitle — логика кнопки play/pause (правка 4,
  // второй раунд): играет — пауза; на паузе — продолжить; ничего не
  // загружено, но что-то выделено на вкладке — можно запустить выделенное;
  // ничего не загружено и ничего не выделено — кнопка недоступна. Сам
  // переход между этими случаями делает audioStore.playOrToggleSelected —
  // здесь только то, что видно оператору (доступность и подпись).
  const canPlayPause = $derived(!isIdle || selectedItemId !== null);
  const playPauseTitle = $derived(
    player?.status === "playing" ? "Пауза" : isIdle ? "Играть" : "Продолжить",
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
  <div class="min-w-0">
    <div class="truncate text-sm font-semibold">
      {isIdle ? "Ничего не играет" : currentTrack?.title || currentTrack?.fileName || "…"}
    </div>
    {#if !isIdle && player?.nextTitle}
      <div class="truncate text-xs opacity-60">далее: {player.nextTitle}</div>
    {/if}
  </div>

  <!-- Транспорт — по центру (правка 6, второй раунд): раньше делил строку
  с названием трека и оттого жался к левому краю (название забирало
  оставшееся место через flex-1); у названия теперь своя строка сверху, а
  этот ряд центрируется независимо от её ширины. -->
  <div class="flex items-center justify-center gap-1">
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
      disabled={!canPlayPause}
      onclick={() => audioStore.playOrToggleSelected(selectedItemId)}
      title={playPauseTitle}
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

  <!-- Таймлайн и громкость — общая grid-раскладка (правки 1 и 5, второй
  раунд; правки 1 и 2, третий раунд): одна и та же сетка колонок применяется
  к обеим строкам, поэтому левая иконка/подпись, средняя колонка и правая
  колонка совпадают по ширине и позиции между строками. Колонки —
  фиксированной ширины (не auto): иначе при смене цифр в "1:23"/"-0:05"
  ширина колонки дёргалась бы на каждый тик опроса. Третья колонка расширена
  с 4.5rem до 9rem (правка 2, третий раунд) — раньше она подходила только под
  короткую подпись "-0:00", а громкость (иконка+ползунок+проценты) в неё не
  помещалась и жила отдельным ad hoc flex-рядом с max-w-xs, из-за чего
  ползунок обрывался на трети ширины окна и строки визуально расходились. -->
  <!-- Третья колонка шире 9rem: за вычетом иконки и подписи процентов на сам
  ползунок громкости оставалось около 78px — он снова оказывался огрызком. -->
  <div class="grid grid-cols-[2.5rem_1fr_13rem] items-center gap-x-2 gap-y-2">
    <!-- text-left, а не text-right: содержимое первой колонки в обеих строках
    прижато к ЛЕВОМУ краю, иначе ряды начинаются на разном x. При прижатии
    вправо подпись и иконка уровня упираются в общий правый край колонки, но
    глифы разной ширины — иконка уже текста, — и нижний ряд визуально
    начинается на 11px правее. Меряется именно общая ширина строки, от первого
    элемента до последнего, а не края ползунков. -->
    <span class="text-left font-mono text-xs opacity-70"
      >{formatMinSec(shownPositionMs)}</span
    >
    <input
      type="range"
      class="range range-xs min-w-0"
      min="0"
      max={Math.max(durationMs, 1)}
      value={shownPositionMs}
      disabled={isIdle}
      oninput={onSeekInput}
      onchange={onSeekChange}
      onpointerup={(e) => e.currentTarget.blur()}
      aria-label="Позиция воспроизведения"
    />
    <!-- text-right обязателен: ячейка сетки растягивается на всю колонку, и без
    него подпись прижималась бы к её левому краю, тогда как блок громкости во
    второй строке той же колонки прижат вправо (justify-end). Именно это
    расхождение и читалось как «строки разной ширины»: правые края не совпадали
    примерно на 135px. -->
    <span
      class={[
        "text-right font-mono text-xs",
        remainingSoon ? "font-semibold text-error" : "opacity-70",
      ]}
    >
      -{formatMinSec(remainingMs)}
    </span>

    <!-- Иконка уровня живёт в ПЕРВОЙ колонке и прижата вправо — ровно как
    "0:00" у таймлайна. Пока уровень был одним блоком на col-span-2, он
    начинался от левого края первой колонки, а подпись таймлайна — от правого:
    нижний ряд визуально начинался на 12px левее и читался как более широкий
    (замерено по скриншоту). -->
    <MuiIcon
      name="graphic_eq"
      style="font-size: 1.1rem"
      classes="justify-self-start shrink-0 opacity-70"
    />

    <!-- Сама полоска — во ВТОРОЙ колонке, то есть стартует там же, где ползунок
    таймлайна. Ширина фиксирована (w-56), а не на всю колонку: растянутый на
    всю строку уровень уже отвергался как слишком широкий, а w-40 — как узкий.
    Тултип переехал с общего блока на полоску: объясняет не только ЧТО это
    (уровень на выходе, а не положение ползунка громкости), но и ЗАЧЕМ —
    увидеть глазами, что звук реально идёт в зал, даже когда его не слышно на
    мониторе оператора. -->
    <div
      class="tooltip tooltip-top w-56 justify-self-start"
      data-tip="Уровень сигнала, который реально уходит в звуковую карту — громкость звука на выходе, а не положение ползунка. Нужен, чтобы увидеть глазами: звук действительно идёт в зал, даже когда не слышно на мониторе."
    >
      <div class="relative h-2 w-full min-w-0 overflow-hidden rounded-full bg-base-300">
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

    <!-- Громкость — теперь справа, в третьей колонке (правка 1, третий
    раунд). Прежний max-w-xs (потолок ширины ползунка, правка 4 первого
    раунда) убран: он и был причиной расхождения строк — обрубал ползунок
    задолго до правого края растягивающейся (1fr) колонки. Колонка сама по
    себе теперь фиксированной ширины (9rem, см. grid-cols выше), поэтому
    потолок ей не нужен — она и так не может расти бесконечно, а ползунок
    (flex-1) заполняет её целиком без разрыва с соседней строкой.
    blur()/onpointerup трогать нельзя — без них умирает вся карта клавиш
    вкладки (isTypingTarget в keyboard.ts глушит Esc/Space/стрелки, пока
    фокус на input[type=range]; onchange не всегда стреляет при клике точно
    по текущему положению бегунка). -->
    <div class="flex min-w-0 items-center justify-end gap-2">
      <MuiIcon
        name={volumePct === 0 ? "volume_off" : "volume_up"}
        style="font-size: 1.1rem"
        classes="shrink-0 opacity-70"
      />
      <input
        type="range"
        class="range range-xs volume-range min-w-0 flex-1"
        min="0"
        max="100"
        value={volumePct}
        oninput={onVolumeInput}
        onchange={onVolumeChange}
        onpointerup={(e) => e.currentTarget.blur()}
        aria-label="Громкость"
      />
      <span class="w-8 shrink-0 text-right font-mono text-xs opacity-60">{volumePct}%</span>
    </div>
  </div>
</div>

<style>
  /* Громкость ниже, чем range-xs: это самый мелкий размер в daisyUI, дальше
     только своими правилами. Уменьшаем и высоту дорожки, и бегунок — если
     тронуть только дорожку, бегунок продолжит задавать высоту всей строки.
     Толщина самой дорожки у daisyUI задаётся box-shadow'ом бегунка
     (--range-shdw), поэтому её отдельно уменьшать не нужно. */
  :global(.volume-range) {
    height: 0.75rem;
  }
  :global(.volume-range::-webkit-slider-thumb) {
    height: 0.75rem;
    width: 0.75rem;
  }
  :global(.volume-range::-moz-range-thumb) {
    height: 0.75rem;
    width: 0.75rem;
  }
</style>
