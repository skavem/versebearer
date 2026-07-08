<script lang="ts">
  import {
    settingsStore,
    type ThemeMode,
  } from "$lib/stores/settingsStore.svelte";
  import type { MuiIconNames } from "./MuiIcon";
  import MuiIcon from "./MuiIcon.svelte";

  let isOpen = $state(false);

  const options: { mode: ThemeMode; label: string; icon: MuiIconNames }[] = [
    { mode: "light", label: "Светлая", icon: "light_mode" },
    { mode: "dark", label: "Тёмная", icon: "dark_mode" },
    { mode: "auto", label: "Авто", icon: "brightness_auto" },
  ];
</script>

<button
  class="btn btn-ghost btn-square text-white"
  onclick={() => (isOpen = true)}
  aria-label="Настройки"
  title="Настройки"
>
  <MuiIcon name="settings" />
</button>

<svelte:window
  onkeydown={(e) => isOpen && e.key === "Escape" && (isOpen = false)}
/>

{#if isOpen}
  <div class="modal modal-open">
    <div class="modal-box">
      <div class="mb-4 flex items-center justify-between">
        <h3 class="text-lg font-bold">Настройки</h3>
        <button
          class="btn btn-ghost btn-sm btn-square"
          onclick={() => (isOpen = false)}
          aria-label="Закрыть"
        >
          <MuiIcon name="close" />
        </button>
      </div>

      <div class="form-control">
        <div class="label py-1">
          <span class="label-text font-medium">Тема оформления</span>
        </div>
        <div class="join w-full">
          {#each options as opt}
            <button
              class={[
                "btn join-item flex-1 gap-1",
                settingsStore.mode === opt.mode ? "btn-neutral" : "btn-outline",
              ]}
              onclick={() => settingsStore.setMode(opt.mode)}
            >
              <MuiIcon name={opt.icon} style="font-size: 1.15rem" />
              {opt.label}
            </button>
          {/each}
        </div>
        {#if settingsStore.mode === "auto"}
          <div class="label py-1">
            <span class="label-text-alt opacity-70">
              Следует за настройкой системы (сейчас {settingsStore.isDark
                ? "тёмная"
                : "светлая"})
            </span>
          </div>
        {/if}
      </div>
    </div>
    <button
      class="modal-backdrop"
      onclick={() => (isOpen = false)}
      aria-label="Закрыть"
    ></button>
  </div>
{/if}
