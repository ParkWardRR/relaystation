const API_BASE = '/api';

export interface Stream {
	id: string;
	name: string;
	upstream: string;
	enabled: boolean;
	profiles: Record<string, Profile>;
}

export interface Profile {
	id?: string;
	enabled: boolean;
	passthrough: boolean;
	path: string;
	codec?: string;
	profile?: string;
	level?: string;
	resolution?: string;
	bitrate?: string;
	maxrate?: string;
	fps?: number;
	audio_bitrate?: string;
	audio_sample?: string;
}

export interface Preset {
	id: string;
	name: string;
	subtitle?: string;
	description?: string;
	codec: string;
	profile: string;
	level: string;
	resolution: string;
	bitrate: string;
	maxrate: string;
	fps: number;
	audio_bitrate: string;
	audio_sample: string;
	builtin: boolean;
}

export interface StreamStatus {
	id: string;
	name: string;
	upstream: string;
	upstream_live: boolean;
	enabled: boolean;
	profiles: ProfileStatus[];
}

export interface ProfileStatus {
	id: string;
	path: string;
	passthrough: boolean;
	enabled: boolean;
	live: boolean;
	running: boolean;
	restart_count: number;
	codec: string;
	resolution: string;
	bitrate: string;
}

export interface ServerInfo {
	hostname: string;
	uptime: string;
	version: string;
}

export interface StatusResponse {
	streams: StreamStatus[];
	server: ServerInfo;
}

export interface Defaults {
	segment_time: number;
	playlist_size: number;
	preset: string;
}

export interface SourceInfo {
	variants: SourceVariant[];
	max_quality: string;
}

export interface SourceVariant {
	bandwidth: number;
	resolution: string;
	codecs: string;
	uri?: string;
}

export interface StreamCharacteristics {
	stream_type: 'live' | 'vod' | 'unknown';
	segment_format: 'mpegts' | 'fmp4' | 'unknown';
	is_multi_variant: boolean;
	has_subtitles: boolean;
	has_audio: boolean;
	target_duration: number;
	max_bandwidth: number;
	max_resolution: string;
	variant_count: number;
}

async function fetchAPI<T>(endpoint: string, options?: RequestInit): Promise<T> {
	const response = await fetch(`${API_BASE}${endpoint}`, {
		headers: {
			'Content-Type': 'application/json',
			...options?.headers
		},
		...options
	});

	if (!response.ok) {
		const error = await response.json().catch(() => ({ error: 'Request failed' }));
		throw new Error(error.error || 'Request failed');
	}

	return response.json();
}

export const api = {
	// Status
	getStatus: () => fetchAPI<StatusResponse>('/status'),
	health: () => fetchAPI<string>('/health'),

	// Streams
	getStreams: () => fetchAPI<Stream[]>('/streams'),
	getStream: (id: string) => fetchAPI<Stream>(`/streams/${id}`),
	createStream: (data: { id: string; name: string; upstream: string; preset?: string }) =>
		fetchAPI<Stream>('/streams', {
			method: 'POST',
			body: JSON.stringify(data)
		}),
	updateStream: (id: string, data: Partial<Stream>) =>
		fetchAPI<Stream>(`/streams/${id}`, {
			method: 'PUT',
			body: JSON.stringify(data)
		}),
	deleteStream: (id: string) =>
		fetchAPI<{ message: string }>(`/streams/${id}`, {
			method: 'DELETE'
		}),
	applyPreset: (id: string, preset: Preset) =>
		fetchAPI<{ message: string }>(`/streams/${id}/preset`, {
			method: 'PUT',
			body: JSON.stringify(preset)
		}),
	getSourceInfo: (id: string) => fetchAPI<SourceInfo>(`/streams/${id}/source-info`),
	getStreamCharacteristics: (id: string) => fetchAPI<StreamCharacteristics>(`/streams/${id}/characteristics`),
	probeURL: (url: string) =>
		fetchAPI<StreamCharacteristics>('/probe-url', {
			method: 'POST',
			body: JSON.stringify({ url })
		}),

	// Presets
	getPresets: () => fetchAPI<Preset[]>('/presets'),
	createPreset: (preset: Omit<Preset, 'builtin'>) =>
		fetchAPI<Preset>('/presets', {
			method: 'POST',
			body: JSON.stringify(preset)
		}),
	deletePreset: (id: string) =>
		fetchAPI<{ message: string }>(`/presets/${id}`, {
			method: 'DELETE'
		}),

	// Settings
	getDefaults: () => fetchAPI<Defaults>('/defaults'),
	updateDefaults: (defaults: Defaults) =>
		fetchAPI<Defaults>('/defaults', {
			method: 'PUT',
			body: JSON.stringify(defaults)
		}),
	reload: () =>
		fetchAPI<{ message: string }>('/reload', {
			method: 'POST'
		})
};
