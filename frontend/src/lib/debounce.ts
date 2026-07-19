/**
 * A per-key debouncer: repeated calls with the same key reset that key's timer,
 * so independent inputs (bgColor, opacity, …) don't cancel each other.
 * Call `cancelAll()` from a component `$effect` cleanup to clear pending timers.
 */
export function createDebouncer() {
  const timers: Record<string, ReturnType<typeof setTimeout>> = {};
  return {
    debounce(key: string, fn: () => void, ms = 150) {
      clearTimeout(timers[key]);
      timers[key] = setTimeout(fn, ms);
    },
    cancelAll() {
      Object.values(timers).forEach(clearTimeout);
    },
  };
}
