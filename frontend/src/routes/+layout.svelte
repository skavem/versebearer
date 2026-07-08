<script>
	import '../app.css';
	import { screenStore } from '$lib/stores/screenStore.svelte';
	import { settingsStore } from '$lib/stores/settingsStore.svelte';

	const { children } = $props();

	// Keep the OS light/dark preference live so "auto" theme mode follows it.
	$effect(() => settingsStore.watchSystem());

	// Apply the resolved DaisyUI theme to <html data-theme>. Re-runs whenever
	// the chosen mode or the OS preference changes.
	$effect(() => {
		document.documentElement.setAttribute('data-theme', settingsStore.themeName);
	});

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

