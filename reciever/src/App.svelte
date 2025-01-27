<script lang="ts">
  let text = $state("");
  $effect(() => {
    const sse = new EventSource("/sse?stream=main");
    sse.onmessage = (event) => {
      text = event.data;
    };
  });
</script>

<main>{text}</main>
