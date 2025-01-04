<script lang="ts">
	import { CreateSong } from '$lib/bindings/changeme/dbhandler';
	import { songsStore } from '$lib/stores/songsStore.svelte';
	import MuiIcon from './MuiIcon.svelte';

	const songs = $derived(songsStore.songs);

	let isOpen = $state(false);

	let title = $state('');
	let number = $derived((songs.list.at(-1)?.number ?? 0) + 1);
</script>

<button
	class="w-full flex flex-row items-center justify-center border-zinc-100 hover:border-sealight gap-2 border-2 p-2 rounded"
	onclick={() => (isOpen = true)}>Добавить песню <MuiIcon name="add" /></button>

<dialog
	class="bg-black bg-opacity-50 z-10 top-0 left-0 m-0 h-full w-full flex items-center justify-center"
	class:flex={isOpen}
	class:hidden={!isOpen}>
	<div class="rounded flex shadow w-1/2 overflow-hidden flex-col bg-white">
		<div class="flex w-full justify-between bg-seawave text-white px-4 p-2 items-center">
			<div class="">Создать песню</div>
			<button class="flex items-center" onclick={() => (isOpen = false)}
				><MuiIcon name="close" /></button>
		</div>

		<div class="p-4 flex flex-col gap-2">
			<label
				class="flex flex-row gap-2 border border-zinc-100 hover:border-seawave rounded py-2 px-2">
				Номер: <input
					value={number}
					type="number"
					onchange={() => void 0}
					class="w-full outline-none" />
			</label>

			<label
				class="flex flex-row gap-2 border border-zinc-100 hover:border-seawave rounded py-2 px-2">
				Название: <input bind:value={title} class="w-full outline-none" />
			</label>
		</div>

		<div class="flex flex-grow flex-row items-center justify-end m-4">
			<button
				class="px-4 py-2 bg-sealight hover:bg-seawave rounded hover:text-white"
				onclick={() => {
					CreateSong(number, title).then(() => (isOpen = false));
				}}>Создать</button>
		</div>
	</div>
</dialog>
