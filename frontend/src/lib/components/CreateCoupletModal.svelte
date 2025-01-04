<script lang="ts">
	import { CreateCouplet, CreateSong } from '$lib/bindings/changeme/dbhandler';
	import { songsStore } from '$lib/stores/songsStore.svelte';
	import MuiIcon from './MuiIcon.svelte';

	let { isModalOpen = $bindable() }: { isModalOpen: boolean } = $props();

	let couplets = $derived(songsStore.couplets);
	let songId = $derived(songsStore.songs.active?.ID);

	let number = $derived(couplets.list.at(-1)?.number ?? 0);
	let text = $state('');
	let label = $state('');
</script>

<button
	class="w-full flex flex-row items-center justify-center border-zinc-100 hover:border-sealight gap-2 border-2 p-2 rounded"
	onclick={() => (isModalOpen = true)}>Добавить Куплет<MuiIcon name="add" /></button>

<dialog
	class="bg-black bg-opacity-50 z-10 top-0 left-0 m-0 h-full w-full flex items-center justify-center"
	class:flex={isModalOpen}
	class:hidden={!isModalOpen}>
	<div class="rounded flex shadow w-1/2 overflow-hidden flex-col bg-white">
		<div class="flex w-full justify-between bg-seawave text-white px-4 p-2 items-center">
			<div class="">Создать Куплет</div>
			<button class="flex items-center" onclick={() => (isModalOpen = false)}
				><MuiIcon name="close" /></button>
		</div>

		<div class="p-4 flex flex-col gap-2">
			<label class="flex flex-col">
				Имя
				<input
					bind:value={label}
					class="outline-none border border-zinc-100 hover:border-seawave rounded p-1" />
			</label>

			<label class="flex flex-col">
				Текст
				<textarea
					bind:value={text}
					class="outline-none border border-zinc-100 hover:border-seawave rounded p-1"
					rows="6"></textarea>
			</label>
		</div>

		<div class="flex flex-grow flex-row items-center justify-end m-4">
			<button
				class="px-4 py-2 bg-sealight hover:bg-seawave rounded hover:text-white"
				onclick={() => {
					if (songId) {
						CreateCouplet(text, label, number, songId).then(() => (isModalOpen = false));
					}
				}}>Создать</button>
		</div>
	</div>
</dialog>
