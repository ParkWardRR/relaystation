<script lang="ts">
	import { page } from '$app/stores';
	import { onMount, onDestroy, tick } from 'svelte';
	import { toast } from 'svelte-sonner';
	import Badge from '$lib/components/ui/badge.svelte';
	import Button from '$lib/components/ui/button.svelte';
	import { streams } from '$lib/stores/status';
	import { api } from '$lib/api/client';
	import { formatBitrate, formatResolution, getStreamUrl } from '$lib/utils';
	import {
		Play,
		Pause,
		Volume2,
		VolumeX,
		Maximize,
		Copy,
		Check,
		ArrowLeft,
		Radio,
		ExternalLink
	} from 'lucide-svelte';

	$: streamId = $page.params.streamId;
	$: profileId = $page.params.profileId;

	let videoElement: HTMLVideoElement;
	let hls: any = null;
	let isPlaying = false;
	let isMuted = true;
	let currentStreamUrl = '';
	let isVideoLoading = true;
	let videoError = false;
	let copied = false;
	let videoMounted = false;

	$: stream = $streams.find(s => s.id === streamId);
	$: profile = stream?.profiles.find(p => p.id.endsWith(`_${profileId}`));
	$: streamUrl = profile ? getStreamUrl(profile.path) : '';

	async function handleCopy() {
		if (!streamUrl) return;
		try {
			if (navigator.clipboard && window.isSecureContext) {
				await navigator.clipboard.writeText(streamUrl);
			} else {
				const textArea = document.createElement('textarea');
				textArea.value = streamUrl;
				textArea.style.position = 'fixed';
				textArea.style.left = '-999999px';
				document.body.appendChild(textArea);
				textArea.focus();
				textArea.select();
				document.execCommand('copy');
				textArea.remove();
			}
			copied = true;
			toast.success('Stream URL copied!');
			setTimeout(() => (copied = false), 2000);
		} catch (err) {
			toast.error('Failed to copy URL');
		}
	}

	async function initHls(url: string) {
		if (!url) return;
		await tick();
		if (!videoElement) return;

		isVideoLoading = true;
		videoError = false;

		try {
			const Hls = (await import('hls.js')).default;

			if (hls) {
				hls.destroy();
				hls = null;
			}

			if (Hls.isSupported()) {
				hls = new Hls({
					enableWorker: true,
					lowLatencyMode: true,
					backBufferLength: 30,
					maxBufferLength: 30,
					maxMaxBufferLength: 60
				});

				hls.loadSource(url);
				hls.attachMedia(videoElement);

				hls.on(Hls.Events.MANIFEST_PARSED, () => {
					isVideoLoading = false;
					videoElement.play().catch(() => {});
				});

				hls.on(Hls.Events.ERROR, (_event: any, data: any) => {
					if (data.fatal) {
						switch (data.type) {
							case Hls.ErrorTypes.NETWORK_ERROR:
								hls.startLoad();
								break;
							case Hls.ErrorTypes.MEDIA_ERROR:
								hls.recoverMediaError();
								break;
							default:
								videoError = true;
								isVideoLoading = false;
								hls.destroy();
								break;
						}
					}
				});
			} else if (videoElement.canPlayType('application/vnd.apple.mpegurl')) {
				videoElement.src = url;
				videoElement.addEventListener('loadedmetadata', () => {
					isVideoLoading = false;
					videoElement.play().catch(() => {});
				}, { once: true });
				videoElement.addEventListener('error', () => {
					videoError = true;
					isVideoLoading = false;
				}, { once: true });
			} else {
				videoError = true;
				isVideoLoading = false;
			}
		} catch (err) {
			videoError = true;
			isVideoLoading = false;
		}
	}

	function togglePlay() {
		if (!videoElement) return;
		if (isPlaying) {
			videoElement.pause();
		} else {
			videoElement.play().catch(() => {});
		}
	}

	function toggleMute() {
		if (!videoElement) return;
		videoElement.muted = !videoElement.muted;
		isMuted = videoElement.muted;
	}

	function toggleFullscreen() {
		if (!videoElement) return;
		if (document.fullscreenElement) {
			document.exitFullscreen();
		} else {
			videoElement.requestFullscreen();
		}
	}

	function onVideoMount(node: HTMLVideoElement) {
		videoElement = node;
		videoMounted = true;
		if (streamUrl && !currentStreamUrl) {
			currentStreamUrl = streamUrl;
			initHls(streamUrl);
		}
		return { destroy() { videoMounted = false; } };
	}

	onMount(async () => {
		try {
			await api.getStatus();
		} catch (e) {
			console.error('Failed to load status:', e);
		}
	});

	$: if (streamUrl && !currentStreamUrl && videoMounted) {
		currentStreamUrl = streamUrl;
		initHls(streamUrl);
	}

	onDestroy(() => {
		if (hls) hls.destroy();
	});
</script>

<svelte:head>
	<title>{stream?.name || 'Watch'} - RelayStation</title>
</svelte:head>

<div class="min-h-screen bg-zinc-950 flex flex-col">
	<!-- Header -->
	<header class="px-4 py-3 border-b border-zinc-800 bg-zinc-900/50">
		<div class="container mx-auto max-w-6xl flex items-center justify-between">
			<div class="flex items-center gap-4">
				<Button variant="ghost" href="/" class="text-zinc-400 hover:text-white -ml-2">
					<ArrowLeft class="h-4 w-4 mr-2" />
					Back
				</Button>
				<div class="h-6 w-px bg-zinc-800"></div>
				<div class="flex items-center gap-2">
					<Radio class="h-5 w-5 text-emerald-500" />
					<span class="text-white font-medium">{stream?.name || 'Loading...'}</span>
					{#if profile?.live}
						<Badge class="bg-emerald-500/20 text-emerald-400 border-emerald-500/30">
							<span class="w-1.5 h-1.5 bg-emerald-500 rounded-full mr-1.5 animate-pulse"></span>
							Live
						</Badge>
					{/if}
				</div>
			</div>
			<div class="flex items-center gap-2">
				<button
					on:click={handleCopy}
					class="flex items-center gap-2 px-3 py-1.5 rounded-md bg-zinc-800 hover:bg-zinc-700 text-zinc-300 text-sm transition-colors"
				>
					{#if copied}
						<Check class="h-4 w-4 text-emerald-500" />
						<span class="text-emerald-400">Copied!</span>
					{:else}
						<Copy class="h-4 w-4" />
						<span>Copy URL</span>
					{/if}
				</button>
				<button
					on:click={() => window.open(streamUrl, '_blank')}
					class="p-1.5 rounded-md bg-zinc-800 hover:bg-zinc-700 text-zinc-400 hover:text-white transition-colors"
					title="Open direct link"
				>
					<ExternalLink class="h-4 w-4" />
				</button>
			</div>
		</div>
	</header>

	<!-- Video Container -->
	<div class="flex-1 flex items-center justify-center p-4 bg-black">
		{#if !stream || !profile}
			<div class="text-center">
				<p class="text-zinc-500 mb-4">Stream not found</p>
				<Button variant="outline" href="/">
					<ArrowLeft class="h-4 w-4 mr-2" />
					Back to Directory
				</Button>
			</div>
		{:else}
			<div class="relative w-full max-w-6xl aspect-video rounded-lg overflow-hidden bg-zinc-900 border border-zinc-800">
				<!-- svelte-ignore a11y-media-has-caption -->
				<video
					use:onVideoMount
					class="w-full h-full object-contain"
					muted={isMuted}
					playsinline
					autoplay
					on:play={() => (isPlaying = true)}
					on:pause={() => (isPlaying = false)}
				></video>

				{#if isVideoLoading}
					<div class="absolute inset-0 flex items-center justify-center bg-black/60">
						<div class="w-12 h-12 border-3 border-emerald-500/30 border-t-emerald-500 rounded-full animate-spin"></div>
					</div>
				{/if}

				{#if videoError}
					<div class="absolute inset-0 flex flex-col items-center justify-center bg-black/80">
						<p class="text-red-400 mb-4">Failed to load stream</p>
						<Button variant="outline" size="sm" on:click={() => initHls(streamUrl)}>
							Retry
						</Button>
					</div>
				{/if}

				<!-- Controls -->
				<div class="absolute bottom-0 left-0 right-0 p-4 bg-gradient-to-t from-black/90 via-black/50 to-transparent">
					<div class="flex items-center justify-between">
						<div class="flex items-center gap-2">
							<button
								on:click={togglePlay}
								class="p-2 rounded-full bg-white/10 hover:bg-white/20 transition-colors"
							>
								{#if isPlaying}
									<Pause class="h-5 w-5 text-white" />
								{:else}
									<Play class="h-5 w-5 text-white" />
								{/if}
							</button>
							<button
								on:click={toggleMute}
								class="p-2 rounded-full bg-white/10 hover:bg-white/20 transition-colors"
							>
								{#if isMuted}
									<VolumeX class="h-5 w-5 text-white" />
								{:else}
									<Volume2 class="h-5 w-5 text-white" />
								{/if}
							</button>
						</div>
						<div class="flex items-center gap-2">
							{#if profile}
								<Badge class="bg-zinc-800/80 text-zinc-300 border-zinc-700">
									{profile.codec?.toUpperCase() || 'COPY'}
									{#if profile.resolution}
										• {formatResolution(profile.resolution)}
									{/if}
									{#if profile.bitrate}
										• {formatBitrate(profile.bitrate)}
									{/if}
								</Badge>
							{/if}
							<button
								on:click={toggleFullscreen}
								class="p-2 rounded-full bg-white/10 hover:bg-white/20 transition-colors"
							>
								<Maximize class="h-5 w-5 text-white" />
							</button>
						</div>
					</div>
				</div>
			</div>
		{/if}
	</div>

	<!-- Stream URL -->
	{#if streamUrl}
		<div class="px-4 py-3 border-t border-zinc-800 bg-zinc-900/30">
			<div class="container mx-auto max-w-6xl">
				<div class="flex items-center gap-2 text-sm">
					<span class="text-zinc-500">Stream URL:</span>
					<code class="flex-1 text-emerald-400/80 font-mono truncate">{streamUrl}</code>
				</div>
			</div>
		</div>
	{/if}
</div>
