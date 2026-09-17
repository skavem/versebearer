<script lang="ts">
  import Audio from "./Audio.svelte";
  import Bible from "./Bible.svelte";
  import Screens from "./Screens.svelte";
  import Songs from "./Songs.svelte";
  import Visual from "./Visual.svelte";
  import SettingsModal from "$lib/components/SettingsModal.svelte";

  const tabs = [
    {
      name: "Библия",
      component: Bible,
    },
    {
      name: "Песни",
      component: Songs,
    },
    {
      name: "Экраны",
      component: Screens,
    },
    {
      name: "Визуал",
      component: Visual,
    },
    {
      name: "Звук",
      component: Audio,
    },
  ];
  let activeTabIndex = $state(1);
  const activeTab = $derived(tabs[activeTabIndex]);
</script>

<!-- Каркас задаёт высоту один раз на всё приложение: экран целиком, документ
     не прокручивается (см. html/body в app.css). Шапка не сжимается, всё
     остальное — прокручиваемая область под ней. Раньше высоту прибивала себе
     каждая вкладка отдельной строкой h-[calc(100vh-4rem)], где 4rem — угаданная
     высота шапки: вкладка, забывшая эту строку, тянула за собой весь документ,
     а вместе с ним и полосу прокрутки, которая исчезала при открытии модалки и
     дёргала вёрстку вбок. -->
<div class="flex h-full w-full flex-col overflow-hidden bg-base-100">
  <!-- Белый цвет — на самих элементах шапки, а не на контейнере: модалка
       настроек рендерится изнутри навбара и, хоть и выглядит отдельным слоем,
       по дереву остаётся его потомком. Общий text-white наследовался внутрь и
       делал нецветные надписи в настройках белыми на светлом фоне. -->
  <div class="navbar shrink-0 bg-neutral">
    <div class="navbar-start">
      <button
        class="btn btn-ghost text-lg text-white"
        onclick={() => (activeTabIndex = 0)}
      >
        VerseBearer
      </button>
    </div>
    <div class="navbar-center flex">
      <div class="tabs tabs-bordered">
        {#each tabs as tab, i}
          {@const active = activeTabIndex === i}
          <button
            onclick={() => (activeTabIndex = i)}
            class={[
              "tab text-lg font-medium text-white",
              {
                "[--fallback-bc:white]": active,
              },
            ]}
          >
            {tab.name}
          </button>
        {/each}
      </div>
    </div>
    <div class="navbar-end">
      <SettingsModal />
    </div>
  </div>

  <!-- min-h-0 обязателен: у flex-элемента min-height по умолчанию auto, то есть
       он не даёт себе стать ниже содержимого, и flex-1 тогда не ограничивает
       высоту — прокрутка уезжает на документ вместо этой области. -->
  <div class="min-h-0 flex-1 overflow-y-auto">
    <activeTab.component />
  </div>
</div>
