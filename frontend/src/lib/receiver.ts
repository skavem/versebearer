// The backdrop images are served by the in-process SSE server (see sse.go),
// which listens on :9093 — a different origin than the operator webview. Used
// for thumbnails/previews in the operator UI.
export const imageUrl = (id: number): string => `http://localhost:9093/image/${id}`;
