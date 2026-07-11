<script lang="ts">
  import BackdropEditor from "$lib/components/BackdropEditor.svelte";
  import StyleEditor from "$lib/components/StyleEditor.svelte";
  import ThemeBar from "$lib/components/ThemeBar.svelte";
  import { visualStore } from "$lib/stores/visualStore.svelte";
</script>

<div class="flex h-[calc(100vh-4rem)] flex-col gap-3 overflow-y-auto p-4">
  <ThemeBar />

  {#if !visualStore.loaded}
    <div class="flex justify-center py-8">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
  {:else}
    <BackdropEditor
      backdrop={visualStore.backdrop}
      images={visualStore.images}
      onUpdate={(patch) => visualStore.updateBackdrop(patch)}
      onReset={() => visualStore.resetBackdrop()}
      onUploadImage={(file) => visualStore.uploadImage(file)}
      onDeleteImage={(id) => visualStore.deleteImage(id)}
    />

    <div class="grid grid-cols-1 gap-3 lg:grid-cols-2">
      <StyleEditor
        title="Стих"
        style={visualStore.verseStyle}
        fonts={visualStore.fonts}
        onUpdate={(patch) => visualStore.updateVerse(patch)}
        onReset={() => visualStore.resetVerse()}
        onUploadFont={(file) => visualStore.uploadFont(file)}
        onDeleteFont={(id) => visualStore.deleteFont(id)}
      />
      <StyleEditor
        title="Куплет"
        style={visualStore.coupletStyle}
        fonts={visualStore.fonts}
        onUpdate={(patch) => visualStore.updateCouplet(patch)}
        onReset={() => visualStore.resetCouplet()}
        onUploadFont={(file) => visualStore.uploadFont(file)}
        onDeleteFont={(id) => visualStore.deleteFont(id)}
      />
    </div>
  {/if}
</div>
