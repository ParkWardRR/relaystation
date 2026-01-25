<script lang="ts">
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import Badge from '$lib/components/ui/badge.svelte';
	import Button from '$lib/components/ui/button.svelte';
	import { streams, serverInfo, isLoading } from '$lib/stores/status';
	import { api, type StreamCharacteristics } from '$lib/api/client';
	import { formatBitrate, formatResolution, getStreamUrl } from '$lib/utils';
	import {
		Radio,
		Copy,
		Check,
		ExternalLink,
		Settings,
		Play,
		Wifi,
		WifiOff,
		Tv,
		Globe,
		Server,
		ArrowRight,
		Layers,
		Zap
	} from 'lucide-svelte';

	let copiedId: string | null = null;
	let characteristics: Record<string, StreamCharacteristics> = {};
	let loadingChars: Record<string, boolean> = {};

	async function loadCharacteristics(streamId: string) {
		if (characteristics[streamId] || loadingChars[streamId]) return;
		loadingChars[streamId] = true;
		try {
			characteristics[streamId] = await api.getStreamCharacteristics(streamId);
		} catch (e) {
			console.error(`Failed to load characteristics for ${streamId}:`, e);
		}
		loadingChars[streamId] = false;
		loadingChars = loadingChars; // trigger reactivity
		characteristics = characteristics;
	}

	async function handleCopy(url: string, id: string) {
		try {
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
			copiedId = id;
			toast.success('URL copied to clipboard');
			setTimeout(() => (copiedId = null), 2000);
		} catch (err) {
			toast.error('Failed to copy URL');
		}
	}

	function openInNewWindow(url: string) {
		window.open(url, '_blank');
	}

	onMount(async () => {
		try {
			await api.getStatus();
		} catch (e) {
			console.error('Failed to load status:', e);
		}
	});

	// Load characteristics for all streams
	$: if ($streams.length > 0) {
		$streams.forEach(s => loadCharacteristics(s.id));
	}

	function formatBandwidth(bw: number): string {
		if (!bw) return '-';
		if (bw >= 1000000) return `${(bw / 1000000).toFixed(1)} Mbps`;
		if (bw >= 1000) return `${(bw / 1000).toFixed(0)} kbps`;
		return `${bw} bps`;
	}
</script>

<div class="min-h-screen bg-zinc-950">
	<div class="container mx-auto px-4 py-8 max-w-6xl">
		<!-- Header -->
		<header class="flex items-center justify-between mb-8 pb-6 border-b border-zinc-800">
			<div class="flex items-center gap-3">
				<div class="p-2.5 rounded-xl bg-emerald-500/10 border border-emerald-500/20">
					<Radio class="h-6 w-6 text-emerald-500" />
				</div>
				<div>
					<h1 class="text-2xl font-bold text-white">RelayStation</h1>
					<p class="text-sm text-zinc-500">Stream Directory</p>
				</div>
			</div>
			<div class="flex items-center gap-4">
				{#if $serverInfo}
					<div class="hidden sm:flex items-center gap-3 text-sm text-zinc-500">
						<div class="flex items-center gap-1.5">
							<Globe class="h-4 w-4" />
							<span>{$serverInfo.public_ip || $serverInfo.hostname}</span>
						</div>
						{#if $serverInfo.version}
							<span class="text-zinc-700">•</span>
							<span>v{$serverInfo.version}</span>
						{/if}
					</div>
				{/if}
				<Button variant="outline" href="/admin" class="border-zinc-700 text-zinc-300 hover:text-white hover:bg-zinc-800">
					<Settings class="h-4 w-4 mr-2" />
					Admin
				</Button>
			</div>
		</header>

		{#if $isLoading}
			<div class="flex items-center justify-center py-32">
				<div class="w-8 h-8 border-2 border-emerald-500/30 border-t-emerald-500 rounded-full animate-spin"></div>
			</div>
		{:else if $streams.length === 0}
			<!-- No Streams -->
			<div class="flex flex-col items-center justify-center py-24 text-center">
				<div class="p-6 rounded-2xl bg-zinc-900 mb-6">
					<Tv class="h-16 w-16 text-zinc-600" />
				</div>
				<h2 class="text-xl font-semibold text-white mb-2">No Streams Configured</h2>
				<p class="text-zinc-500 mb-6 max-w-md">
					Add your first stream in the admin panel to get started.
				</p>
				<Button href="/admin" class="bg-emerald-600 hover:bg-emerald-700">
					<Settings class="h-4 w-4 mr-2" />
					Open Admin
				</Button>
			</div>
		{:else}
			<!-- Stream Directory -->
			<div class="space-y-6">
				{#each $streams as stream (stream.id)}
					{@const chars = characteristics[stream.id]}
					<div class="rounded-xl border border-zinc-800 bg-zinc-900/50 overflow-hidden">
						<!-- Stream Header -->
						<div class="px-6 py-4 border-b border-zinc-800 bg-zinc-900">
							<div class="flex items-center justify-between">
								<div class="flex items-center gap-4">
									<div class="flex items-center gap-3">
										<h2 class="text-lg font-semibold text-white">{stream.name}</h2>
										{#if stream.enabled && stream.profiles.some(p => p.live)}
											<Badge class="bg-emerald-500/20 text-emerald-400 border-emerald-500/30">
												<span class="w-1.5 h-1.5 bg-emerald-500 rounded-full mr-1.5 animate-pulse"></span>
												Live
											</Badge>
										{:else if stream.enabled}
											<Badge class="bg-yellow-500/20 text-yellow-400 border-yellow-500/30">
												Starting
											</Badge>
										{:else}
											<Badge variant="outline" class="text-zinc-500 border-zinc-700">
												Disabled
											</Badge>
										{/if}
									</div>
								</div>
								<div class="flex items-center gap-2 text-sm">
									{#if stream.upstream_live}
										<div class="flex items-center gap-1.5 text-emerald-400">
											<Wifi class="h-4 w-4" />
											<span>Source Online</span>
										</div>
									{:else}
										<div class="flex items-center gap-1.5 text-red-400">
											<WifiOff class="h-4 w-4" />
											<span>Source Offline</span>
										</div>
									{/if}
								</div>
							</div>
						</div>

						<div class="p-6">
							<div class="grid lg:grid-cols-2 gap-6">
								<!-- Input Section -->
								<div>
									<h3 class="text-xs font-medium text-zinc-500 uppercase tracking-wider mb-3 flex items-center gap-2">
										<Server class="h-3.5 w-3.5" />
										Input Source
									</h3>
									<div class="space-y-3">
										<div class="p-3 rounded-lg bg-zinc-800/50 border border-zinc-700/50">
											<p class="text-xs text-zinc-500 mb-1">Upstream URL</p>
											<code class="text-sm text-zinc-300 font-mono break-all">{stream.upstream}</code>
										</div>

										{#if chars}
											<div class="grid grid-cols-2 gap-2">
												<div class="p-2 rounded-lg bg-zinc-800/30 border border-zinc-700/30">
													<p class="text-xs text-zinc-500">Type</p>
													<p class="text-sm text-white font-medium">
														{chars.stream_type === 'live' ? 'Live' : chars.stream_type === 'vod' ? 'VOD' : 'Unknown'}
													</p>
												</div>
												<div class="p-2 rounded-lg bg-zinc-800/30 border border-zinc-700/30">
													<p class="text-xs text-zinc-500">Format</p>
													<p class="text-sm text-white font-medium">{chars.segment_format.toUpperCase()}</p>
												</div>
												{#if chars.is_multi_variant}
													<div class="p-2 rounded-lg bg-zinc-800/30 border border-zinc-700/30">
														<p class="text-xs text-zinc-500">Variants</p>
														<p class="text-sm text-white font-medium">{chars.variant_count} qualities</p>
													</div>
												{/if}
												{#if chars.max_bandwidth}
													<div class="p-2 rounded-lg bg-zinc-800/30 border border-zinc-700/30">
														<p class="text-xs text-zinc-500">Max Bitrate</p>
														<p class="text-sm text-white font-medium">{formatBandwidth(chars.max_bandwidth)}</p>
													</div>
												{/if}
											</div>
										{:else if loadingChars[stream.id]}
											<div class="flex items-center gap-2 text-zinc-500 text-sm">
												<div class="w-4 h-4 border-2 border-zinc-700 border-t-zinc-500 rounded-full animate-spin"></div>
												<span>Loading source info...</span>
											</div>
										{/if}
									</div>
								</div>

								<!-- Output Section -->
								<div>
									<h3 class="text-xs font-medium text-zinc-500 uppercase tracking-wider mb-3 flex items-center gap-2">
										<Layers class="h-3.5 w-3.5" />
										Output Streams
									</h3>
									<div class="space-y-2">
										{#if stream.profiles.length === 0}
											<p class="text-sm text-zinc-500 italic">No output profiles configured</p>
										{:else}
											{#each stream.profiles as profile (profile.id)}
												{@const streamUrl = getStreamUrl(profile.path)}
												<div class="p-3 rounded-lg bg-zinc-800/50 border border-zinc-700/50 hover:border-zinc-600 transition-colors">
													<div class="flex items-center justify-between mb-2">
														<div class="flex items-center gap-2">
															<span class="text-sm font-medium text-white">
																{profile.passthrough ? 'Passthrough' : profile.codec?.toUpperCase() || 'Transcode'}
															</span>
															{#if profile.resolution}
																<Badge variant="outline" class="text-xs text-zinc-400 border-zinc-700">
																	{formatResolution(profile.resolution)}
																</Badge>
															{/if}
															{#if profile.bitrate}
																<Badge variant="outline" class="text-xs text-zinc-400 border-zinc-700">
																	{formatBitrate(profile.bitrate)}
																</Badge>
															{/if}
														</div>
														<div class="flex items-center gap-1">
															{#if profile.live}
																<span class="flex items-center gap-1 text-xs text-emerald-400">
																	<span class="w-1.5 h-1.5 bg-emerald-500 rounded-full animate-pulse"></span>
																	Live
																</span>
															{:else if profile.running}
																<span class="text-xs text-yellow-400">Buffering</span>
															{:else}
																<span class="text-xs text-zinc-500">Stopped</span>
															{/if}
														</div>
													</div>

													<div class="flex items-center gap-2">
														<code class="flex-1 text-xs text-emerald-400/80 font-mono truncate bg-zinc-900/50 px-2 py-1 rounded">
															{streamUrl}
														</code>
														<button
															on:click={() => handleCopy(streamUrl, profile.id)}
															class="p-1.5 rounded-md bg-zinc-700/50 hover:bg-zinc-700 text-zinc-400 hover:text-white transition-colors"
															title="Copy URL"
														>
															{#if copiedId === profile.id}
																<Check class="h-4 w-4 text-emerald-500" />
															{:else}
																<Copy class="h-4 w-4" />
															{/if}
														</button>
														<button
															on:click={() => openInNewWindow(streamUrl)}
															class="p-1.5 rounded-md bg-zinc-700/50 hover:bg-zinc-700 text-zinc-400 hover:text-white transition-colors"
															title="Open in new window"
														>
															<ExternalLink class="h-4 w-4" />
														</button>
														{#if profile.live}
															<a
																href="/watch/{stream.id}/{profile.id.split('_').pop()}"
																class="flex items-center gap-1 px-2 py-1.5 rounded-md bg-emerald-600/20 hover:bg-emerald-600/30 text-emerald-400 text-xs font-medium transition-colors"
																title="Watch stream"
															>
																<Play class="h-3.5 w-3.5" />
																Watch
															</a>
														{/if}
													</div>
												</div>
											{/each}
										{/if}
									</div>
								</div>
							</div>
						</div>
					</div>
				{/each}
			</div>

			<!-- Quick Reference -->
			<div class="mt-8 p-4 rounded-xl bg-zinc-900/30 border border-zinc-800">
				<h3 class="text-sm font-medium text-zinc-400 mb-3 flex items-center gap-2">
					<Zap class="h-4 w-4" />
					Quick Reference
				</h3>
				<div class="grid sm:grid-cols-3 gap-4 text-sm">
					<div>
						<p class="text-zinc-500 mb-1">VLC / Media Player</p>
						<p class="text-zinc-300">Copy URL → Open Network Stream → Paste</p>
					</div>
					<div>
						<p class="text-zinc-500 mb-1">OBS Studio</p>
						<p class="text-zinc-300">Media Source → URL → Paste stream URL</p>
					</div>
					<div>
						<p class="text-zinc-500 mb-1">Browser</p>
						<p class="text-zinc-300">Click "Open" button or use HLS.js player</p>
					</div>
				</div>
			</div>
		{/if}

		<!-- Footer -->
		<footer class="mt-12 pt-6 border-t border-zinc-800 text-center">
			<p class="text-sm text-zinc-600">
				RelayStation • Self-hosted HLS Streaming Relay
				{#if $serverInfo?.uptime}
					<span class="text-zinc-700">•</span>
					<span>Uptime: {$serverInfo.uptime}</span>
				{/if}
			</p>
		</footer>
	</div>
</div>
