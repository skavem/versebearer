<script lang="ts">
	import { GetShownCouplet, GetShownVerse } from '$lib/bindings/changeme/dbhandler';
	import { BibleStore } from '$lib/stores/BibleStore.svelte';

	let verse = $derived(BibleStore.verses.shown);
	GetShownVerse().then((sc) => (BibleStore.verses.shown = sc));

	const isOverflown = ({
		clientWidth,
		clientHeight,
		scrollWidth,
		scrollHeight,
	}: HTMLDivElement) => {
		return scrollHeight > clientHeight || scrollWidth > clientWidth;
	};

	let verseDiv = $state<null | HTMLDivElement>(null);
	let outerDiv = $state<null | HTMLDivElement>(null);
	$effect(() => {
		if (!verse) return;
		console.log('run');
		if (!verseDiv || !outerDiv) return;

		let size = 4;
		outerDiv.style.fontSize = `${size}em`;
		while (!isOverflown(outerDiv)) {
			console.log(size, verseDiv.scrollHeight, verseDiv.scrollWidth, isOverflown(outerDiv));
			outerDiv.style.fontSize = `${size}em`;
			size++;
			if (size > 8) break;
		}
		while (isOverflown(outerDiv)) {
			console.log(size, verseDiv.scrollHeight, verseDiv.scrollWidth, isOverflown(outerDiv));
			outerDiv.style.fontSize = `${size}em`;
			size--;
		}
	});
</script>

{#if verse}
	<div class="flex w-full flex-grow flex-col max-h-[100vh] font-bold text-white">
		<div
			class="flex flex-col items-center justify-center bg-black bg-opacity-85 w-[calc(100%-2rem)] h-[calc(100%-2rem)] m-4 rounded-xl overflow-hidden p-8"
			bind:this={outerDiv}>
			<div bind:this={verseDiv} class="text-wrap whitespace-pre text-center leading-none">
				{verse.text}
			</div>

			<span class="text-white w-full text-right text-4xl pt-4"
				>{verse.Book.shortName} {verse.Chapter.number}:{verse.number}</span>
		</div>
	</div>
{/if}
