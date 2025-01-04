<script lang="ts">
	import { GetTranslations, HideVerse, ShowVerse } from '$lib/bindings/changeme/dbhandler';
	import { Events } from '@wailsio/runtime';
	import List from '$lib/components/List.svelte';
	import Select from '$lib/components/Select.svelte';
	import { BibleStore } from '$lib/stores/BibleStore.svelte';
	import MuiIcon from '$lib/components/MuiIcon.svelte';

	GetTranslations()
		.then((tr) => (BibleStore.translations.list = tr))
		.catch(console.error);

	let translations = $derived(BibleStore.translations);
	let books = $derived(BibleStore.books);
	$inspect(books);
	let chapters = $derived(BibleStore.chapters);
	let verses = $derived(BibleStore.verses);
	let history = $derived(BibleStore.history);
	let shown = $derived(verses.shown);

	const showVerse = () => {
		const activeId = verses.active?.ID;
		if (activeId) {
			ShowVerse(activeId);
		}
	};

	$effect(() => {
		const onKeyDown = (e: KeyboardEvent) => {
			switch (e.code) {
				case 'Escape':
					HideVerse();
					e.preventDefault();
					return;
				case 'Enter':
					showVerse();
					e.preventDefault();
					return;
				case 'ArrowDown':
					verses.next();
					e.preventDefault();
					return;
				case 'ArrowUp':
					verses.prev();
					e.preventDefault();
					return;
				case 'ArrowLeft':
					chapters.prev();
					e.preventDefault;
					return;
				case 'ArrowRight':
					chapters.next();
					e.preventDefault();
					return;
			}
		};
		document.addEventListener('keydown', onKeyDown);
		return () => document.removeEventListener('keydown', onKeyDown);
	});
</script>

<div class="flex flex-row p-4 h-[calc(100vh-4rem)] gap-2">
	<div class="flex flex-col gap-2">
		<Select
			bind:items={translations.list}
			getName={(i) => i.name}
			activeItem={translations.active}
			setActiveItem={(i) => (translations.active = i)} />
		<Select
			bind:items={books.list}
			getName={(i) => i.title}
			activeItem={books.active}
			setActiveItem={(i) => (books.active = i)} />
		<List
			items={books.list}
			getName={(i) => i.title}
			onClick={(i) => (books.active = i)}
			activeItem={books.active}>
			{#snippet dividerBefore(item)}
				{#if item.dividerBefore}
					<div class="relative flex p-2 items-center cursor-default">
						<div class="flex-grow border-t-2 border-gray-400"></div>
						<span class="flex-shrink mx-1 text-gray-700">{item.dividerBefore}</span>
						<div class="flex-grow border-t-2 border-gray-400"></div>
					</div>
				{/if}
			{/snippet}
		</List>
	</div>

	<div>
		<List
			items={chapters.list}
			getName={(i) => i.number.toString()}
			onClick={(i) => (chapters.active = i)}
			activeItem={chapters.active} />
	</div>

	<div class="flex flex-col gap-2">
		<List
			items={verses.list}
			getName={(i) => i.text}
			onClick={(i) => (verses.active = i)}
			onDoubleClick={(i) => {
				if (i.ID !== shown?.ID) {
					showVerse();
				} else {
					HideVerse();
				}
			}}
			activeItem={verses.active}>
			{#snippet rightMark(i)}
				{#if i.ID === shown?.ID}
					<MuiIcon name={'remove_red_eye'} />
				{/if}
			{/snippet}
			{#snippet leftMark(i)}
				<div class="bg-sealight rounded px-1 text-white">
					{i.number.toString()}
				</div>
			{/snippet}
		</List>

		<div class="flex justify-center gap-2">
			<!-- svelte-ignore a11y_click_events_have_key_events -->
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="bg-seawave text-white rounded p-2 px-4 flex gap-2 cursor-pointer"
				onclick={() => {
					if (shown) {
						HideVerse();
					} else {
						showVerse();
					}
				}}>
				<MuiIcon name={shown ? 'visibility_off' : 'visibility'} />
				{shown ? 'СКРЫТЬ' : 'ПОКАЗАТЬ'}
			</div>
		</div>

		<List
			items={history.list}
			getName={(i) => i.text}
			onClick={history.restore}
			activeItem={history.active}>
			{#snippet leftMark(i)}
				<span class="bg-sealight rounded px-1 text-white text-nowrap">
					{`${i.Book.shortName} ${i.Chapter.number}:${i.number}`}
				</span>
			{/snippet}
		</List>
	</div>
</div>
