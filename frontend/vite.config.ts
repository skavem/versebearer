import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig, searchForWorkspaceRoot } from 'vite';

export default defineConfig({
    server: {
        // Только IPv4. По умолчанию vite слушает [::1], а прокси Wails для хоста
        // "localhost" принудительно набирает tcp4 (обход багов IPv6 на Windows) —
        // получается 127.0.0.1:9245 против слушателя на ::1 и окно ловит 502.
        host: '127.0.0.1',
        // Канал горячей перезагрузки — напрямую на vite, минуя Wails. Страница
        // открыта с wails.localhost:9245, и без этого клиент полез бы обновляться
        // туда же, а прокси Wails вебсокеты не проксирует — отвечает 501.
        hmr: { host: '127.0.0.1', port: 9245, protocol: 'ws' },
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
