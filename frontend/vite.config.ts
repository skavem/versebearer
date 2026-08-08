import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig, searchForWorkspaceRoot } from 'vite';

export default defineConfig({
    server: {
        // Только IPv4. По умолчанию vite слушает [::1], а прокси Wails для хоста
        // "localhost" принудительно набирает tcp4 (обход багов IPv6 на Windows) —
        // получается 127.0.0.1:9245 против слушателя на ::1 и окно ловит 502.
        host: '127.0.0.1',
        // Канал горячей перезагрузки — напрямую на vite, минуя Wails. Страница
        // открыта с wails.localhost, и без этого клиент полез бы обновляться
        // туда же, а вебсокеты Wails не проксирует. Порт не указан намеренно:
        // vite подставит тот, на котором сам слушает, а он задаётся снаружи
        // (WAILS_VITE_PORT / `wails3 dev -port`).
        hmr: { host: '127.0.0.1' },
        fs: {
          allow: [
            // search up for workspace root
            searchForWorkspaceRoot(process.cwd()),
            // your custom rules
            './bindings/*',
          ],
        },
    },
	plugins: [sveltekit()]
});
