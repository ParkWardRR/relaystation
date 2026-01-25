<script lang="ts">
	import { onMount, onDestroy, tick } from 'svelte';
	import { toast } from 'svelte-sonner';
	import Badge from '$lib/components/ui/badge.svelte';
	import Button from '$lib/components/ui/button.svelte';
	import { streams, serverInfo, isLoading, liveStreams } from '$lib/stores/status';
	import { api } from '$lib/api/client';
	import { formatBitrate, formatResolution, getStreamUrl } from '$lib/utils';
	import {
		Radio,
		Tv,
		Copy,
		ExternalLink,
		Settings,
		Server,
		Clock,
		Activity,
		Play,
		Pause,
		Volume2,
		VolumeX,
		Maximize,
		Wifi,
		WifiOff,
		Gauge,
		Film,
		MonitorPlay,
		Check,
		Globe
	} from 'lucide-svelte';

	let videoElement: HTMLVideoElement;
	let hls: any = null;
	let isPlaying = false;
	let isMuted = true;
	let currentStreamUrl = '';
	let selectedStreamId = '';
	let isVideoLoading = true;
	let videoError = false;
	let copied = false;
	let videoMounted = false;

	$: activeProfile = $streams
		.flatMap((s) => s.profiles.map((p) => ({ ...p, streamName: s.name, streamId: s.id })))
		.find((p) => p.live && p.enabled);

	$: totalStreams = $streams.length;
	$: liveCount = $liveStreams.length;
	$: totalProfiles = $streams.reduce((acc, s) => acc + s.profiles.length, 0);

	async function handleCopy(path: string) {
		try {
			const url = getStreamUrl(path);
			// Try modern clipboard API first
			if (navigator.clipboard && window.isSecureContext) {
				await navigator.clipboard.writeText(url);
			} else {
				// Fallback for HTTP or older browsers
				const textArea = document.createElement('textarea');
				textArea.value = url;
				textArea.style.position = 'fixed';
				textArea.style.left = '-999999px';
				textArea.style.top = '-999999px';
				document.body.appendChild(textArea);
				textArea.focus();
				textArea.select();
				document.execCommand('copy');
				textArea.remove();
			}
			copied = true;
			toast.success('Stream URL copied to clipboard');
			setTimeout(() => (copied = false), 2000);
		} catch (err) {
			console.error('Failed to copy:', err);
			toast.error('Failed to copy URL');
		}
	}

	async function initHls(url: string) {
		if (!url) return;

		// Wait for video element to be available
		await tick();
		if (!videoElement) {
			console.error('Video element not ready');
			return;
		}

		isVideoLoading = true;
		videoError = false;

		try {
			// Dynamically import HLS.js
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
					// Auto-play when stream loads
					videoElement.play().catch(() => {});
				});

				hls.on(Hls.Events.ERROR, (_event: any, data: any) => {
					console.error('HLS Error:', data);
					if (data.fatal) {
						switch (data.type) {
							case Hls.ErrorTypes.NETWORK_ERROR:
								console.error('Network error, trying to recover...');
								hls.startLoad();
								break;
							case Hls.ErrorTypes.MEDIA_ERROR:
								console.error('Media error, trying to recover...');
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
				// Safari native HLS support
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
				toast.error('HLS playback not supported in this browser');
			}
		} catch (err) {
			console.error('Failed to initialize HLS:', err);
			videoError = true;
			isVideoLoading = false;
		}
	}

	function selectStream(profile: any) {
		if (!profile) return;
		const url = getStreamUrl(profile.path);
		currentStreamUrl = url;
		selectedStreamId = profile.id;
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

	// Called when video element is mounted
	function onVideoMount(node: HTMLVideoElement) {
		videoElement = node;
		videoMounted = true;

		// If we already have an active profile, load it
		if (activeProfile && !currentStreamUrl) {
			selectStream(activeProfile);
		}

		return {
			destroy() {
				videoMounted = false;
			}
		};
	}

	onMount(async () => {
		try {
			await api.getStatus();
		} catch (e) {
			console.error('Failed to load status:', e);
		}
	});

	// Auto-select first live stream when data loads and video is ready
	$: if (activeProfile && !currentStreamUrl && videoMounted) {
		selectStream(activeProfile);
	}

	onDestroy(() => {
		if (hls) {
			hls.destroy();
		}
	});
</script>

<div class="min-h-screen animated-gradient relative overflow-hidden">
	<!-- Background effects -->
	<div class="noise-overlay"></div>
	<div class="hero-glow -top-40 -left-40 opacity-50"></div>
	<div class="hero-glow -bottom-40 -right-40 opacity-30"></div>
	<div class="grid-pattern fixed inset-0 opacity-30"></div>

	<div class="container mx-auto px-4 py-6 sm:py-8 max-w-7xl relative z-10">
		<!-- Header -->
		<header class="mb-8 sm:mb-12 animate-fade-in">
			<div class="flex flex-col sm:flex-row items-center justify-between gap-4 sm:gap-0">
				<div class="flex items-center gap-3">
					<div class="p-3 rounded-2xl bg-primary/10 border border-primary/20 glow-primary">
						<Radio class="h-7 w-7 sm:h-8 sm:w-8 text-primary" />
					</div>
					<div>
						<h1 class="text-3xl sm:text-4xl font-bold tracking-tight gradient-text">
							RelayStation
						</h1>
						<p class="text-muted-foreground text-sm sm:text-base">
							HLS Streaming Relay & Transcoder
						</p>
					</div>
				</div>

				<div class="flex items-center gap-3">
					{#if $serverInfo}
						<div class="hidden sm:flex items-center gap-4 px-4 py-2 rounded-full glass text-sm">
							<div class="flex items-center gap-2 text-muted-foreground">
								<div class="connection-dot connected"></div>
								<span>Connected</span>
							</div>
							<div class="h-4 w-px bg-border"></div>
							<div class="flex items-center gap-1.5">
								<Clock class="h-3.5 w-3.5 text-muted-foreground" />
								<span class="text-muted-foreground">{$serverInfo.uptime}</span>
							</div>
						</div>
					{/if}
					<Button variant="outline" href="/admin" class="glass-hover border-primary/20">
						<Settings class="h-4 w-4 mr-2" />
						<span class="hidden sm:inline">Admin</span>
					</Button>
				</div>
			</div>
		</header>

		<!-- Stats Row -->
		<div class="grid grid-cols-2 sm:grid-cols-4 gap-3 sm:gap-4 mb-8 stagger-children">
			<div class="stat-card glass rounded-xl p-4 border border-border/50">
				<div class="flex items-center gap-3">
					<div class="p-2 rounded-lg bg-primary/10">
						<Film class="h-5 w-5 text-primary" />
					</div>
					<div>
						<p class="text-2xl font-bold">{totalStreams}</p>
						<p class="text-xs text-muted-foreground">Total Streams</p>
					</div>
				</div>
			</div>
			<div class="stat-card glass rounded-xl p-4 border border-border/50">
				<div class="flex items-center gap-3">
					<div class="p-2 rounded-lg bg-emerald-500/10">
						<Activity class="h-5 w-5 text-emerald-500" />
					</div>
					<div>
						<p class="text-2xl font-bold text-emerald-500">{liveCount}</p>
						<p class="text-xs text-muted-foreground">Live Now</p>
					</div>
				</div>
			</div>
			<div class="stat-card glass rounded-xl p-4 border border-border/50">
				<div class="flex items-center gap-3">
					<div class="p-2 rounded-lg bg-blue-500/10">
						<Gauge class="h-5 w-5 text-blue-500" />
					</div>
					<div>
						<p class="text-2xl font-bold">{totalProfiles}</p>
						<p class="text-xs text-muted-foreground">Profiles</p>
					</div>
				</div>
			</div>
			<div class="stat-card glass rounded-xl p-4 border border-border/50">
				<div class="flex items-center gap-3">
					<div class="p-2 rounded-lg bg-purple-500/10">
						<Globe class="h-5 w-5 text-purple-500" />
					</div>
					<div>
						<p class="text-lg font-bold truncate" title={$serverInfo?.public_ip || $serverInfo?.hostname || '-'}>
							{$serverInfo?.public_ip || $serverInfo?.hostname || '-'}
						</p>
						<p class="text-xs text-muted-foreground truncate" title={$serverInfo?.reverse_dns || 'Server'}>
							{$serverInfo?.reverse_dns || 'Server'}
						</p>
					</div>
				</div>
			</div>
		</div>

		{#if $isLoading}
			<!-- Loading State -->
			<div class="space-y-6">
				<div class="aspect-video rounded-2xl skeleton"></div>
				<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
					{#each [1, 2, 3] as _}
						<div class="h-40 rounded-xl skeleton"></div>
					{/each}
				</div>
			</div>
		{:else if $streams.length === 0}
			<!-- Empty State -->
			<div class="flex flex-col items-center justify-center py-20 animate-fade-in">
				<div class="p-6 rounded-full bg-muted/50 mb-6">
					<Tv class="h-16 w-16 text-muted-foreground" />
				</div>
				<h2 class="text-2xl font-semibold mb-2">No Streams Configured</h2>
				<p class="text-muted-foreground mb-6 text-center max-w-md">
					Get started by adding your first stream in the admin panel. You can configure multiple
					streams with different transcoding profiles.
				</p>
				<Button href="/admin" size="lg" class="glow-primary">
					<Settings class="h-5 w-5 mr-2" />
					Open Admin Panel
				</Button>
			</div>
		{:else}
			<!-- Main Content -->
			<div class="grid lg:grid-cols-3 gap-6">
				<!-- Video Player Section -->
				<div class="lg:col-span-2 space-y-4 animate-fade-in-up">
					<div
						class="video-container aspect-video rounded-2xl border border-border/50 glow-primary-intense relative overflow-hidden"
					>
						{#if activeProfile}
							<!-- svelte-ignore a11y-media-has-caption -->
							<video
								use:onVideoMount
								class="w-full h-full"
								muted={isMuted}
								playsinline
								autoplay
								on:play={() => (isPlaying = true)}
								on:pause={() => (isPlaying = false)}
							></video>

							<!-- Video Overlay -->
							<div class="video-overlay"></div>

							<!-- Loading Spinner -->
							{#if isVideoLoading}
								<div class="absolute inset-0 flex items-center justify-center bg-black/50">
									<div
										class="w-12 h-12 border-4 border-primary/30 border-t-primary rounded-full animate-spin"
									></div>
								</div>
							{/if}

							<!-- Error State -->
							{#if videoError}
								<div class="absolute inset-0 flex flex-col items-center justify-center bg-black/80">
									<WifiOff class="h-12 w-12 text-destructive mb-4" />
									<p class="text-destructive font-medium">Failed to load stream</p>
									<Button
										variant="outline"
										size="sm"
										class="mt-4"
										on:click={() => initHls(currentStreamUrl)}
									>
										Retry
									</Button>
								</div>
							{/if}

							<!-- Controls -->
							<div
								class="absolute bottom-0 left-0 right-0 p-4 bg-gradient-to-t from-black/80 to-transparent"
							>
								<div class="flex items-center justify-between">
									<div class="flex items-center gap-3">
										<button
											on:click={togglePlay}
											class="p-2 rounded-full bg-white/10 hover:bg-white/20 transition-colors"
											aria-label={isPlaying ? 'Pause' : 'Play'}
										>
											{#if isPlaying}
												<Pause class="h-5 w-5" />
											{:else}
												<Play class="h-5 w-5" />
											{/if}
										</button>
										<button
											on:click={toggleMute}
											class="p-2 rounded-full bg-white/10 hover:bg-white/20 transition-colors"
											aria-label={isMuted ? 'Unmute' : 'Mute'}
										>
											{#if isMuted}
												<VolumeX class="h-5 w-5" />
											{:else}
												<Volume2 class="h-5 w-5" />
											{/if}
										</button>
									</div>

									<div class="flex items-center gap-3">
										{#if activeProfile}
											<Badge variant="secondary" class="bg-white/10 text-white border-0">
												{activeProfile.codec?.toUpperCase() || 'PASSTHROUGH'}
												{#if activeProfile.resolution}
													• {formatResolution(activeProfile.resolution)}
												{/if}
											</Badge>
										{/if}
										<button
											on:click={toggleFullscreen}
											class="p-2 rounded-full bg-white/10 hover:bg-white/20 transition-colors"
											aria-label="Toggle fullscreen"
										>
											<Maximize class="h-5 w-5" />
										</button>
									</div>
								</div>
							</div>
						{:else}
							<!-- No Active Stream -->
							<div class="absolute inset-0 flex flex-col items-center justify-center">
								<MonitorPlay class="h-16 w-16 text-muted-foreground mb-4" />
								<p class="text-muted-foreground">No active streams</p>
								<p class="text-sm text-muted-foreground/70 mt-1">
									Enable a stream to start watching
								</p>
							</div>
						{/if}
					</div>

					<!-- Stream Info Bar -->
					{#if activeProfile}
						<div
							class="glass rounded-xl p-4 border border-border/50 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4"
						>
							<div class="flex items-center gap-3">
								<div class="relative">
									<div
										class="w-3 h-3 bg-emerald-500 rounded-full animate-pulse"
									></div>
								</div>
								<div>
									<h3 class="font-semibold">{activeProfile.streamName}</h3>
									<p class="text-sm text-muted-foreground">
										{formatBitrate(activeProfile.bitrate)} •
										{formatResolution(activeProfile.resolution)}
									</p>
								</div>
							</div>
							<div class="flex items-center gap-2 w-full sm:w-auto">
								<Button
									variant="secondary"
									size="sm"
									class="flex-1 sm:flex-none"
									on:click={() => handleCopy(activeProfile.path)}
								>
									{#if copied}
										<Check class="h-4 w-4 mr-2 text-emerald-500" />
										Copied!
									{:else}
										<Copy class="h-4 w-4 mr-2" />
										Copy URL
									{/if}
								</Button>
								<Button
									variant="ghost"
									size="sm"
									on:click={() => window.open(getStreamUrl(activeProfile.path), '_blank')}
								>
									<ExternalLink class="h-4 w-4" />
								</Button>
							</div>
						</div>
					{/if}
				</div>

				<!-- Stream List Sidebar -->
				<div class="space-y-4 animate-fade-in-up" style="animation-delay: 0.2s;">
					<div class="flex items-center justify-between">
						<h2 class="text-lg font-semibold">Available Streams</h2>
						<Badge variant="outline" class="text-xs">
							{$streams.length} stream{$streams.length !== 1 ? 's' : ''}
						</Badge>
					</div>

					<div class="space-y-3 max-h-[600px] overflow-y-auto pr-2">
						{#each $streams as stream (stream.id)}
							<div
								class="stream-card glass rounded-xl p-4 border border-border/50"
								class:animate-border-glow={stream.profiles.some((p) => p.live)}
							>
								<div class="flex items-start justify-between mb-3">
									<div class="flex-1 min-w-0">
										<h3 class="font-semibold truncate">{stream.name}</h3>
										<p class="text-xs text-muted-foreground font-mono truncate">
											{stream.id}
										</p>
									</div>
									<div class="ml-2 flex-shrink-0">
										{#if stream.enabled && stream.profiles.some((p) => p.live)}
											<Badge variant="default" class="bg-emerald-500/20 text-emerald-400 border-emerald-500/30">
												<span class="relative flex h-2 w-2 mr-1.5">
													<span
														class="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"
													></span>
													<span class="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"
													></span>
												</span>
												Live
											</Badge>
										{:else if stream.enabled}
											<Badge variant="secondary">Starting</Badge>
										{:else}
											<Badge variant="outline">Disabled</Badge>
										{/if}
									</div>
								</div>

								<div class="mb-3 p-2 rounded-lg bg-background/30">
									<div class="flex items-center gap-2 text-xs">
										{#if stream.upstream_live}
											<Wifi class="h-3.5 w-3.5 text-emerald-500" />
											<span class="text-emerald-500">Upstream Connected</span>
										{:else}
											<WifiOff class="h-3.5 w-3.5 text-destructive" />
											<span class="text-destructive">Upstream Offline</span>
										{/if}
									</div>
								</div>

								{#if stream.profiles.length > 0}
									<div class="space-y-2">
										{#each stream.profiles as profile (profile.id)}
											<button
												class="w-full p-3 rounded-lg border bg-background/30 hover:bg-background/50 transition-all text-left group {selectedStreamId ===
												profile.id
													? 'border-primary bg-primary/10'
													: 'border-border/50'}"
												on:click={() => selectStream({ ...profile, streamName: stream.name })}
											>
												<div class="flex items-center justify-between mb-1">
													<div class="flex items-center gap-2">
														<span class="font-medium text-sm">
															{profile.codec?.toUpperCase() || 'Passthrough'}
														</span>
														{#if profile.resolution}
															<Badge variant="outline" class="text-xs py-0">
																{formatResolution(profile.resolution)}
															</Badge>
														{/if}
													</div>
													{#if profile.live}
														<span class="text-xs text-emerald-500 flex items-center gap-1">
															<span class="w-1.5 h-1.5 bg-emerald-500 rounded-full animate-pulse"
															></span>
															Live
														</span>
													{:else if profile.running}
														<span class="text-xs text-yellow-500">Buffering</span>
													{/if}
												</div>

												<div class="flex items-center justify-between">
													<span class="text-xs text-muted-foreground">
														{formatBitrate(profile.bitrate)}
													</span>
													<div
														class="opacity-0 group-hover:opacity-100 transition-opacity flex items-center gap-1"
													>
														<Play class="h-3 w-3 text-primary" />
														<span class="text-xs text-primary">Play</span>
													</div>
												</div>
											</button>
										{/each}
									</div>
								{:else}
									<p class="text-xs text-muted-foreground text-center py-2">
										No profiles configured
									</p>
								{/if}
							</div>
						{/each}
					</div>
				</div>
			</div>
		{/if}

		<!-- Footer -->
		<footer class="mt-12 sm:mt-16 text-center animate-fade-in" style="animation-delay: 0.4s;">
			<div class="glass rounded-full inline-flex items-center gap-4 px-6 py-3 text-sm text-muted-foreground">
				<span>RelayStation</span>
				<span class="text-border">•</span>
				<span>Self-hosted HLS Relay</span>
				{#if $serverInfo?.version}
					<span class="text-border">•</span>
					<span>v{$serverInfo.version}</span>
				{/if}
			</div>
		</footer>
	</div>
</div>
