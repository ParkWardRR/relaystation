<script lang="ts">
	import { onMount, onDestroy, tick } from 'svelte';
	import { toast } from 'svelte-sonner';
	import Badge from '$lib/components/ui/badge.svelte';
	import Button from '$lib/components/ui/button.svelte';
	import { streams, serverInfo, isLoading } from '$lib/stores/status';
	import { api, type StreamCharacteristics } from '$lib/api/client';
	import { formatBitrate, formatResolution, getStreamUrl } from '$lib/utils';
	import {
		Play,
		Pause,
		Volume2,
		VolumeX,
		Maximize,
		Copy,
		Check,
		ExternalLink,
		Settings,
		Radio,
		Tv,
		Info,
		Zap,
		Film,
		Layers,
		Clock,
		Subtitles,
		Music
	} from 'lucide-svelte';

	let videoElement: HTMLVideoElement;
	let hls: any = null;
	let isPlaying = false;
	let isMuted = true;
	let currentStreamUrl = '';
	let isVideoLoading = true;
	let videoError = false;
	let copied = false;
	let vlcCopied = false;
	let videoMounted = false;
	let characteristics: StreamCharacteristics | null = null;
	let loadingCharacteristics = false;

	// Get first live stream
	$: activeStream = $streams.find(s => s.enabled && s.profiles.some(p => p.live));
	$: activeProfile = activeStream?.profiles.find(p => p.live && p.enabled);

	async function loadCharacteristics() {
		if (!activeStream) return;
		loadingCharacteristics = true;
		try {
			characteristics = await api.getStreamCharacteristics(activeStream.id);
		} catch (e) {
			console.error('Failed to load characteristics:', e);
			characteristics = null;
		}
		loadingCharacteristics = false;
	}

	async function handleCopy() {
		if (!activeProfile) return;
		try {
			const url = getStreamUrl(activeProfile.path);
			if (navigator.clipboard && window.isSecureContext) {
				await navigator.clipboard.writeText(url);
			} else {
				const textArea = document.createElement('textarea');
				textArea.value = url;
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

	function getVlcUrl(): string {
		if (!activeProfile) return '';
		const streamUrl = getStreamUrl(activeProfile.path);
		// VLC can open URLs directly via vlc:// protocol or command line
		return `vlc://${streamUrl.replace(/^https?:\/\//, '')}`;
	}

	async function handleVlcCopy() {
		if (!activeProfile) return;
		try {
			const url = getStreamUrl(activeProfile.path);
			if (navigator.clipboard && window.isSecureContext) {
				await navigator.clipboard.writeText(url);
			} else {
				const textArea = document.createElement('textarea');
				textArea.value = url;
				textArea.style.position = 'fixed';
				textArea.style.left = '-999999px';
				document.body.appendChild(textArea);
				textArea.focus();
				textArea.select();
				document.execCommand('copy');
				textArea.remove();
			}
			vlcCopied = true;
			toast.success('Open VLC and paste this URL (Ctrl+N / Cmd+N)');
			setTimeout(() => (vlcCopied = false), 3000);
		} catch (err) {
			toast.error('Failed to copy');
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

	function selectStream() {
		if (!activeProfile) return;
		const url = getStreamUrl(activeProfile.path);
		currentStreamUrl = url;
		initHls(url);
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
		if (activeProfile && !currentStreamUrl) {
			selectStream();
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

	$: if (activeProfile && !currentStreamUrl && videoMounted) {
		selectStream();
	}

	$: if (activeStream && !characteristics && !loadingCharacteristics) {
		loadCharacteristics();
	}

	onDestroy(() => {
		if (hls) hls.destroy();
	});

	function formatBandwidth(bw: number): string {
		if (!bw) return '-';
		if (bw >= 1000000) return `${(bw / 1000000).toFixed(1)} Mbps`;
		if (bw >= 1000) return `${(bw / 1000).toFixed(0)} kbps`;
		return `${bw} bps`;
	}
</script>

<div class="min-h-screen bg-gradient-to-br from-zinc-950 via-zinc-900 to-zinc-950">
	<div class="container mx-auto px-4 py-8 max-w-5xl">
		<!-- Header -->
		<header class="flex items-center justify-between mb-8">
			<div class="flex items-center gap-3">
				<div class="p-2.5 rounded-xl bg-emerald-500/10 border border-emerald-500/20">
					<Radio class="h-6 w-6 text-emerald-500" />
				</div>
				<div>
					<h1 class="text-2xl font-bold text-white">RelayStation</h1>
					<p class="text-sm text-zinc-500">HLS Streaming Relay</p>
				</div>
			</div>
			<Button variant="ghost" href="/admin" class="text-zinc-400 hover:text-white">
				<Settings class="h-4 w-4 mr-2" />
				Admin
			</Button>
		</header>

		{#if $isLoading}
			<div class="flex items-center justify-center py-32">
				<div class="w-8 h-8 border-2 border-emerald-500/30 border-t-emerald-500 rounded-full animate-spin"></div>
			</div>
		{:else if !activeStream || !activeProfile}
			<!-- No Active Stream -->
			<div class="flex flex-col items-center justify-center py-24 text-center">
				<div class="p-6 rounded-2xl bg-zinc-800/50 mb-6">
					<Tv class="h-16 w-16 text-zinc-600" />
				</div>
				<h2 class="text-xl font-semibold text-white mb-2">No Active Stream</h2>
				<p class="text-zinc-500 mb-6 max-w-md">
					Configure and enable a stream in the admin panel to start watching.
				</p>
				<Button href="/admin" class="bg-emerald-600 hover:bg-emerald-700">
					<Settings class="h-4 w-4 mr-2" />
					Open Admin
				</Button>
			</div>
		{:else}
			<!-- Main Content -->
			<div class="space-y-6">
				<!-- Video Player -->
				<div class="relative rounded-2xl overflow-hidden bg-black border border-zinc-800 shadow-2xl">
					<div class="aspect-video">
						<!-- svelte-ignore a11y-media-has-caption -->
						<video
							use:onVideoMount
							class="w-full h-full object-contain bg-black"
							muted={isMuted}
							playsinline
							autoplay
							on:play={() => (isPlaying = true)}
							on:pause={() => (isPlaying = false)}
						></video>

						{#if isVideoLoading}
							<div class="absolute inset-0 flex items-center justify-center bg-black/60">
								<div class="w-10 h-10 border-3 border-emerald-500/30 border-t-emerald-500 rounded-full animate-spin"></div>
							</div>
						{/if}

						{#if videoError}
							<div class="absolute inset-0 flex flex-col items-center justify-center bg-black/80">
								<p class="text-red-400 mb-4">Failed to load stream</p>
								<Button variant="outline" size="sm" on:click={() => initHls(currentStreamUrl)}>
									Retry
								</Button>
							</div>
						{/if}

						<!-- Video Controls -->
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
									<div class="ml-3 flex items-center gap-2">
										<span class="w-2 h-2 bg-emerald-500 rounded-full animate-pulse"></span>
										<span class="text-sm text-white font-medium">{activeStream.name}</span>
									</div>
								</div>
								<div class="flex items-center gap-2">
									<Badge class="bg-zinc-800 text-zinc-300 border-zinc-700">
										{activeProfile.codec?.toUpperCase() || 'COPY'}
										{#if activeProfile.resolution}
											• {formatResolution(activeProfile.resolution)}
										{/if}
									</Badge>
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
				</div>

				<!-- Quick Actions -->
				<div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
					<button
						on:click={handleCopy}
						class="flex items-center justify-center gap-2 p-4 rounded-xl bg-zinc-800/50 border border-zinc-700/50 hover:bg-zinc-800 hover:border-zinc-600 transition-all group"
					>
						{#if copied}
							<Check class="h-5 w-5 text-emerald-500" />
							<span class="text-emerald-500 font-medium">Copied!</span>
						{:else}
							<Copy class="h-5 w-5 text-zinc-400 group-hover:text-white transition-colors" />
							<span class="text-zinc-300 group-hover:text-white transition-colors">Copy Stream URL</span>
						{/if}
					</button>

					<button
						on:click={handleVlcCopy}
						class="flex items-center justify-center gap-2 p-4 rounded-xl bg-zinc-800/50 border border-zinc-700/50 hover:bg-zinc-800 hover:border-zinc-600 transition-all group"
					>
						{#if vlcCopied}
							<Check class="h-5 w-5 text-emerald-500" />
							<span class="text-emerald-500 font-medium">Now paste in VLC!</span>
						{:else}
							<Play class="h-5 w-5 text-orange-400 group-hover:text-orange-300 transition-colors" />
							<span class="text-zinc-300 group-hover:text-white transition-colors">Open in VLC</span>
						{/if}
					</button>

					<button
						on:click={() => window.open(getStreamUrl(activeProfile.path), '_blank')}
						class="flex items-center justify-center gap-2 p-4 rounded-xl bg-zinc-800/50 border border-zinc-700/50 hover:bg-zinc-800 hover:border-zinc-600 transition-all group"
					>
						<ExternalLink class="h-5 w-5 text-zinc-400 group-hover:text-white transition-colors" />
						<span class="text-zinc-300 group-hover:text-white transition-colors">Direct Link</span>
					</button>
				</div>

				<!-- Stream Info Cards -->
				<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
					<!-- Stream Details -->
					<div class="p-5 rounded-xl bg-zinc-800/30 border border-zinc-700/50">
						<h3 class="text-sm font-medium text-zinc-400 mb-4 flex items-center gap-2">
							<Info class="h-4 w-4" />
							Stream Details
						</h3>
						<div class="space-y-3">
							<div class="flex justify-between">
								<span class="text-zinc-500">Name</span>
								<span class="text-white font-medium">{activeStream.name}</span>
							</div>
							<div class="flex justify-between">
								<span class="text-zinc-500">Codec</span>
								<span class="text-white">{activeProfile.codec?.toUpperCase() || 'Passthrough'}</span>
							</div>
							<div class="flex justify-between">
								<span class="text-zinc-500">Resolution</span>
								<span class="text-white">{formatResolution(activeProfile.resolution)}</span>
							</div>
							<div class="flex justify-between">
								<span class="text-zinc-500">Bitrate</span>
								<span class="text-white">{formatBitrate(activeProfile.bitrate)}</span>
							</div>
							<div class="flex justify-between">
								<span class="text-zinc-500">Status</span>
								<Badge class="bg-emerald-500/20 text-emerald-400 border-emerald-500/30">
									<span class="w-1.5 h-1.5 bg-emerald-500 rounded-full mr-1.5 animate-pulse"></span>
									Live
								</Badge>
							</div>
						</div>
					</div>

					<!-- Source Characteristics -->
					<div class="p-5 rounded-xl bg-zinc-800/30 border border-zinc-700/50">
						<h3 class="text-sm font-medium text-zinc-400 mb-4 flex items-center gap-2">
							<Zap class="h-4 w-4" />
							Source Info
						</h3>
						{#if loadingCharacteristics}
							<div class="flex items-center justify-center py-6">
								<div class="w-5 h-5 border-2 border-zinc-600 border-t-zinc-400 rounded-full animate-spin"></div>
							</div>
						{:else if characteristics}
							<div class="space-y-3">
								<div class="flex justify-between items-center">
									<span class="text-zinc-500 flex items-center gap-2">
										<Film class="h-3.5 w-3.5" />
										Type
									</span>
									<Badge class={characteristics.stream_type === 'live' ? 'bg-red-500/20 text-red-400 border-red-500/30' : 'bg-blue-500/20 text-blue-400 border-blue-500/30'}>
										{characteristics.stream_type.toUpperCase()}
									</Badge>
								</div>
								<div class="flex justify-between items-center">
									<span class="text-zinc-500 flex items-center gap-2">
										<Layers class="h-3.5 w-3.5" />
										Format
									</span>
									<span class="text-white">{characteristics.segment_format.toUpperCase()}</span>
								</div>
								{#if characteristics.is_multi_variant}
									<div class="flex justify-between items-center">
										<span class="text-zinc-500 flex items-center gap-2">
											<Layers class="h-3.5 w-3.5" />
											Variants
										</span>
										<span class="text-white">{characteristics.variant_count} quality levels</span>
									</div>
								{/if}
								{#if characteristics.max_bandwidth}
									<div class="flex justify-between items-center">
										<span class="text-zinc-500 flex items-center gap-2">
											<Zap class="h-3.5 w-3.5" />
											Max Bitrate
										</span>
										<span class="text-white">{formatBandwidth(characteristics.max_bandwidth)}</span>
									</div>
								{/if}
								{#if characteristics.max_resolution}
									<div class="flex justify-between items-center">
										<span class="text-zinc-500 flex items-center gap-2">
											<Tv class="h-3.5 w-3.5" />
											Max Resolution
										</span>
										<span class="text-white">{characteristics.max_resolution}</span>
									</div>
								{/if}
								<div class="flex justify-between items-center">
									<span class="text-zinc-500 flex items-center gap-2">
										<Music class="h-3.5 w-3.5" />
										Audio
									</span>
									<span class="text-white">{characteristics.has_audio ? 'Yes' : 'No'}</span>
								</div>
								{#if characteristics.has_subtitles}
									<div class="flex justify-between items-center">
										<span class="text-zinc-500 flex items-center gap-2">
											<Subtitles class="h-3.5 w-3.5" />
											Subtitles
										</span>
										<span class="text-white">Available</span>
									</div>
								{/if}
							</div>
						{:else}
							<p class="text-zinc-500 text-sm text-center py-4">Unable to load source info</p>
						{/if}
					</div>
				</div>

				<!-- URL Display -->
				<div class="p-4 rounded-xl bg-zinc-900 border border-zinc-800">
					<div class="flex items-center justify-between gap-4">
						<div class="flex-1 min-w-0">
							<p class="text-xs text-zinc-500 mb-1">Stream URL</p>
							<code class="text-sm text-emerald-400 font-mono break-all">{getStreamUrl(activeProfile.path)}</code>
						</div>
						<Button variant="ghost" size="sm" on:click={handleCopy} class="shrink-0">
							{#if copied}
								<Check class="h-4 w-4 text-emerald-500" />
							{:else}
								<Copy class="h-4 w-4" />
							{/if}
						</Button>
					</div>
				</div>
			</div>
		{/if}

		<!-- Footer -->
		<footer class="mt-12 text-center">
			<p class="text-sm text-zinc-600">
				RelayStation
				{#if $serverInfo?.version}
					<span class="text-zinc-700">•</span>
					<span>v{$serverInfo.version}</span>
				{/if}
				{#if $serverInfo?.public_ip}
					<span class="text-zinc-700">•</span>
					<span>{$serverInfo.public_ip}</span>
				{/if}
			</p>
		</footer>
	</div>
</div>
