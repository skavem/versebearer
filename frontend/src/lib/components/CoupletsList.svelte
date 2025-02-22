<script lang="ts" generics="T extends {ID: number}">
  import {
    HideCouplet,
    RemoveCouplet,
    ShowCouplet,
    UpdateCouplet,
  } from "$lib/bindings/changeme/dbhandler";
  import { songsStore } from "$lib/stores/songsStore.svelte";
  import CoupletItem from "./CoupletItem.svelte";
  import CreateEditCoupletModal from "./CreateEditCoupletModal.svelte";
  import MuiIcon from "./MuiIcon.svelte";

  let shown = $derived(songsStore.couplets.shown);
  let couplets = $derived(songsStore.couplets);
  let activeCoupletInd = $derived.by(() =>
    couplets.list.findIndex((v) => v.ID === couplets.active?.ID),
  );

  let isModalOpen = $state(false);
  let isEditMode = $state(false);
  let number = $state(1);
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="flex h-2/3 min-h-0 flex-row gap-2">
  <div
    class="group/list flex flex-grow select-none flex-col overflow-y-scroll border-2 border-zinc-100"
    onkeydown={(e) => {
      if (
        ["Space", "ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight"].indexOf(
          e.code,
        ) > -1
      ) {
        e.preventDefault();
      }
    }}
  >
    {#each couplets.list as couplet}
      <CoupletItem
        isActive={(couplets.active?.ID || 0) === couplet.ID}
        ondblclick={() => {
          if (shown?.ID === couplet.ID) {
            HideCouplet();
          } else {
            ShowCouplet(couplet.ID);
          }
        }}
        onclick={() => (couplets.active = couplet)}
        getName={() => couplet.text}
        item={couplet}
        multiline={true}
      >
        {#snippet rightMark(i)}
          {#if i.ID === shown?.ID}
            <MuiIcon name={"visibility"} />
          {/if}
        {/snippet}
        {#snippet leftMark()}
          <span class="badge badge-neutral badge-lg font-semibold"
            >{couplet.label}</span
          >
        {/snippet}
      </CoupletItem>
    {/each}
  </div>

  <div class="flex h-full">
    <div class="my-auto flex flex-col gap-2">
      <button
        class="btn btn-neutral btn-sm btn-square"
        disabled={activeCoupletInd <= 0}
        onclick={async () => {
          if (activeCoupletInd === 0 || !couplets.active) return;
          const prevCouplet = couplets.list.at(activeCoupletInd - 1)!;
          console.log(
            prevCouplet.text,
            couplets.active.number,
            couplets.active.text,
            prevCouplet.number,
          );
          await UpdateCouplet(
            prevCouplet.ID,
            prevCouplet.label,
            prevCouplet.text,
            couplets.active.number,
          );
          await UpdateCouplet(
            couplets.active.ID,
            couplets.active.label,
            couplets.active.text,
            prevCouplet.number,
          );
        }}
      >
        <MuiIcon name="arrow_upward" />
      </button>
      <button
        class="btn btn-neutral btn-sm btn-square"
        disabled={activeCoupletInd === couplets.list.length - 1}
        onclick={async () => {
          if (activeCoupletInd === couplets.list.length - 1 || !couplets.active)
            return;
          const nextCouplet = couplets.list.at(activeCoupletInd + 1)!;
          console.log(
            nextCouplet.text,
            couplets.active.number,
            couplets.active.text,
            nextCouplet.number,
          );
          await UpdateCouplet(
            nextCouplet.ID,
            nextCouplet.label,
            nextCouplet.text,
            couplets.active.number,
          );
          await UpdateCouplet(
            couplets.active.ID,
            couplets.active.label,
            couplets.active.text,
            nextCouplet.number,
          );
        }}
      >
        <MuiIcon name="arrow_downward" />
      </button>

      <div class="divider m-0"></div>

      <button
        class="btn btn-neutral btn-sm btn-square"
        disabled={!couplets.active}
        onclick={() => {
          isEditMode = true;
          isModalOpen = true;
          number = couplets.active!.number;
        }}
      >
        <MuiIcon name="edit" />
      </button>

      <div class="divider m-0"></div>

      <button
        class="btn btn-neutral btn-sm btn-square"
        onclick={() => {
          if (couplets.active?.ID) {
            RemoveCouplet(couplets.active.ID);
          }
        }}
      >
        <MuiIcon name="delete" />
      </button>
    </div>
  </div>
</div>

<button
  class="hover:border-black/40 flex w-full flex-row items-center justify-center gap-2 rounded border-2 border-zinc-100 p-2"
  onclick={() => {
    isEditMode = false;
    isModalOpen = true;
    number = (couplets.active?.number ?? 0) + 1;
  }}
>
  <p>{couplets.active ? "Добавить после выбранного" : "Добавить в конец"}</p>
  <MuiIcon name="add" />
</button>

<CreateEditCoupletModal
  bind:isModalOpen
  bind:selected={couplets.active}
  bind:isEdit={isEditMode}
  bind:number
/>
