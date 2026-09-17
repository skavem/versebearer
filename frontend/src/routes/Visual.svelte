<script lang="ts">
  import BackdropEditor from "$lib/components/BackdropEditor.svelte";
  import StyleEditor from "$lib/components/StyleEditor.svelte";
  import ThemeBar from "$lib/components/ThemeBar.svelte";
  import { visualStore } from "$lib/stores/visualStore.svelte";
</script>

<!-- gap-2 — тот же ритм, что на «Библии»/«Песнях»/«Звуке» (Bible.svelte:111,
Songs.svelte:101, Audio.svelte:141): там между блоками страницы всегда gap-2,
здесь раньше был gap-3 — единственное реальное отличие в отступах верхнего
уровня. -->
<div class="flex flex-col gap-2 p-4">
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

    <div class="grid grid-cols-1 gap-2 lg:grid-cols-2">
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
