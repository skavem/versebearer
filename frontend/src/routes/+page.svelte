<script lang="ts">
  import Bible from "./Bible.svelte";
  import FreeText from "./FreeText.svelte";
  import Songs from "./Songs.svelte";

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
      component: FreeText,
    },
  ];
  let activeTabIndex = $state(1);
  const activeTab = $derived(tabs[activeTabIndex]);
</script>

<div class="flex w-full flex-grow flex-col bg-white">
  <div class="navbar bg-neutral text-white">
    <div class="navbar-start">
      <button
        class="btn btn-ghost text-lg"
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
    <div class="navbar-end"></div>
  </div>

  <activeTab.component />
</div>
