<script>
	import '../app.css';
	import { screenStore } from '$lib/stores/screenStore.svelte';

	const { children } = $props();

	// Global shortcut: Ctrl+Shift+W toggles projection on the non-primary screen.
	// Uses event.code ("KeyW") so it fires on any keyboard layout (e.g. Cyrillic "ц").
	$effect(() => {
		function onKeyDown(/** @type {KeyboardEvent} */ e) {
			if (e.ctrlKey && e.shiftKey && !e.altKey && e.code === 'KeyW') {
				e.preventDefault();
				screenStore.toggleSecondary();
			}
		}
		document.addEventListener('keydown', onKeyDown);
		return () => document.removeEventListener('keydown', onKeyDown);
	});
</script>

{@render children()}

