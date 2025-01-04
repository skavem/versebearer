<script lang="ts">
	import { GetShownCouplet } from '$lib/bindings/changeme/dbhandler';
	import { songsStore } from '$lib/stores/songsStore.svelte';

	let couplet = $derived(songsStore.couplets.shown);
	GetShownCouplet().then((sc) => (songsStore.couplets.shown = sc));

	const isOverflown = ({
		clientWidth,
		clientHeight,
		scrollWidth,
		scrollHeight,
	}: HTMLDivElement) => {
		return scrollHeight > clientHeight || scrollWidth > clientWidth;
	};

	let coupletDiv = $state<null | HTMLDivElement>(null);
	let outerDiv = $state<null | HTMLDivElement>(null);
	$effect(() => {
		if (!couplet) return;
		console.log('run');
		if (!coupletDiv || !outerDiv) return;

		let size = 14;
		coupletDiv.style.fontSize = `${size}px`;
		while (!isOverflown(outerDiv)) {
			console.log(size, coupletDiv.scrollHeight, coupletDiv.scrollWidth, isOverflown(outerDiv));
			coupletDiv.style.fontSize = `${size}px`;
			size++;
			if (size > 120) break;
		}
		while (isOverflown(outerDiv)) {
			console.log(size, coupletDiv.scrollHeight, coupletDiv.scrollWidth, isOverflown(outerDiv));
			coupletDiv.style.fontSize = `${size}px`;
			size--;
		}
	});
</script>

{#if couplet}
	<div class="flex w-full flex-grow flex-col max-h-[100vh]">
		<div
			class="flex items-center justify-center bg-black bg-opacity-85 w-[calc(100%-2rem)] h-[calc(100%-2rem)] m-4 rounded-xl overflow-hidden"
			bind:this={outerDiv}>
			<div
				bind:this={coupletDiv}
				class="text-wrap font-bold text-white whitespace-pre text-center p-8 leading-none">
				{couplet.text}
			</div>
		</div>
	</div>
{/if}
