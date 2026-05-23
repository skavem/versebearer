<script lang="ts">
  import FontUploader from "$lib/components/FontUploader.svelte";
  import StyleEditor from "$lib/components/StyleEditor.svelte";
  import { visualStore } from "$lib/stores/visualStore.svelte";
</script>

<div class="flex flex-col gap-4 p-4 overflow-y-auto">
  <FontUploader
    fonts={visualStore.fonts}
    onUpload={(file) => visualStore.uploadFont(file)}
    onDelete={(id) => visualStore.deleteFont(id)}
  />

  {#if !visualStore.loaded}
    <div class="flex justify-center py-8">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
  {:else}
    <div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
      <StyleEditor
        title="Стих"
        style={visualStore.verseStyle}
        fonts={visualStore.fonts}
        onUpdate={(patch) => visualStore.updateVerse(patch)}
        onReset={() => visualStore.resetVerse()}
      />
      <StyleEditor
        title="Куплет"
        style={visualStore.coupletStyle}
        fonts={visualStore.fonts}
        onUpdate={(patch) => visualStore.updateCouplet(patch)}
        onReset={() => visualStore.resetCouplet()}
      />
    </div>
  {/if}
</div>
