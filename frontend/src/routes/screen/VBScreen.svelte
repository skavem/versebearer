<script lang="ts">
	import ShowCouplet from './ShowCouplet.svelte';
	import ShowVerse from './ShowVerse.svelte';
</script>

<ShowCouplet />
<ShowVerse />

<style>
	:global(html) {
		height: 100%;
	}
	:global(body) {
		height: 100%;
		display: flex;
	}
</style>
