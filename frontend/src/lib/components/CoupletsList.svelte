<script lang="ts" generics="T extends {ID: number}">
	import { songsStore } from '$lib/stores/songsStore.svelte';
	import CoupletItem from './CoupletItem.svelte';
	import MuiIcon from './MuiIcon.svelte';
	import { HideCouplet, RemoveCouplet, ShowCouplet } from '$lib/bindings/changeme/dbhandler';
	import CreateCoupletModal from './CreateCoupletModal.svelte';
	import type { Couplet } from '$lib/bindings/changeme/backend/models';

	let items = $derived(songsStore.couplets.list);
	let activeItem = $derived(songsStore.couplets.active);
	let shown = $derived(songsStore.couplets.shown);
	let couplets = $derived(songsStore.couplets);

	type Point = { x: number; y: number };
	let contextCoordinates = $state<Point | null>(null);
	let contextItem = $state<Couplet | null>(null);

	let isModalOpen = $state(false);
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
	class="h-full border-zinc-100 border-2 overflow-y-scroll select-none group/list"
	oncontextmenu={(e) => {
		contextCoordinates = { x: e.x, y: e.y };
		e.preventDefault();
	}}
	onkeydown={(e) => {
		if (['Space', 'ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight'].indexOf(e.code) > -1) {
			e.preventDefault();
		}
	}}>
	{#each items as couplet}
		<CoupletItem
			isActive={(activeItem?.ID || 0) === couplet.ID}
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
			oncontextmenu={(e) => {
				contextItem = couplet;
				contextCoordinates = { x: e.x, y: e.y };
				e.preventDefault();
			}}
			multiline={true}>
			{#snippet rightMark(i)}
				{#if i.ID === shown?.ID}
					<MuiIcon name={'visibility'} />
				{/if}
			{/snippet}
			{#snippet leftMark()}
				<span class="px-1 bg-sealight rounded w-fit text-white">{couplet.label}</span>
			{/snippet}
		</CoupletItem>
	{/each}
</div>

{#if contextCoordinates}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="absolute bg-white shadow-xl min-w-32 rounded-xl flex flex-col overflow-hidden"
		oncontextmenu={(e) => e.preventDefault()}
		style={`top: ${contextCoordinates.y}px; left: ${contextCoordinates.x}px`}>
		<button
			class="hover:bg-zinc-100 p-2"
			onclick={() => {
				isModalOpen = true;
				contextCoordinates = null;
			}}>Добавить куплет</button>

		{#if contextItem}
			<button
				class="hover:bg-zinc-100 p-2"
				onclick={() => {
					RemoveCouplet(contextItem!.ID);
					contextCoordinates = null;
				}}>
				Удалить куплет
			</button>
		{/if}

		<button onclick={() => (contextCoordinates = null)} class="hover:bg-zinc-100 p-2"
			>Закрыть</button>
	</div>
{/if}

<CreateCoupletModal bind:isModalOpen />
