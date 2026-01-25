import { writable, derived } from 'svelte/store';
import type { StatusResponse, StreamStatus, ServerInfo } from '$lib/api/client';

export const statusStore = writable<StatusResponse | null>(null);
export const isLoading = writable(true);
export const error = writable<string | null>(null);

export const streams = derived(statusStore, ($status) => $status?.streams ?? []);
export const serverInfo = derived(statusStore, ($status) => $status?.server ?? null);

export const liveStreams = derived(streams, ($streams) =>
	$streams.filter((s) => s.enabled && s.profiles.some((p) => p.live))
);

export const offlineStreams = derived(streams, ($streams) =>
	$streams.filter((s) => !s.enabled || !s.profiles.some((p) => p.live))
);

let ws: WebSocket | null = null;
let reconnectTimeout: ReturnType<typeof setTimeout>;

export function connectWebSocket() {
	if (ws?.readyState === WebSocket.OPEN) return;

	const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
	const wsUrl = `${protocol}//${window.location.host}/ws/events`;

	ws = new WebSocket(wsUrl);

	ws.onopen = () => {
		console.log('[WebSocket] Connected');
		error.set(null);
	};

	ws.onmessage = (event) => {
		try {
			const data = JSON.parse(event.data);
			if (data.type === 'status_update' && data.data) {
				statusStore.set({
					streams: data.data,
					server: data.server || { hostname: '', uptime: '', version: '' }
				});
				isLoading.set(false);
			}
		} catch (e) {
			console.error('[WebSocket] Parse error:', e);
		}
	};

	ws.onerror = (e) => {
		console.error('[WebSocket] Error:', e);
		error.set('Connection error');
	};

	ws.onclose = () => {
		console.log('[WebSocket] Disconnected, reconnecting...');
		reconnectTimeout = setTimeout(connectWebSocket, 3000);
	};
}

export function disconnectWebSocket() {
	clearTimeout(reconnectTimeout);
	ws?.close();
	ws = null;
}
