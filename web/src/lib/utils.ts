import { type ClassValue, clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
	return twMerge(clsx(inputs));
}

export function copyToClipboard(text: string): Promise<void> {
	return navigator.clipboard.writeText(text);
}

export function formatBitrate(bitrate: string | undefined): string {
	if (!bitrate) return '-';
	const num = parseInt(bitrate.replace(/[^0-9]/g, ''));
	if (num >= 1000) {
		return `${(num / 1000).toFixed(1)} Mbps`;
	}
	return `${num} kbps`;
}

export function formatResolution(resolution: string | undefined): string {
	if (!resolution) return '-';
	const [w, h] = resolution.split('x');
	if (h === '1080') return '1080p';
	if (h === '720') return '720p';
	if (h === '480') return '480p';
	return resolution;
}

export function getStreamUrl(path: string): string {
	const base = window.location.origin;
	// Ensure path starts with /hls/ for the HLS static file server
	const hlsPath = path.startsWith('/hls/') ? path : `/hls${path}`;
	return `${base}${hlsPath}stream.m3u8`;
}
