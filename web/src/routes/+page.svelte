<script lang="ts">
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import Card from '$lib/components/ui/card.svelte';
	import Badge from '$lib/components/ui/badge.svelte';
	import Button from '$lib/components/ui/button.svelte';
	import { streams, serverInfo, isLoading } from '$lib/stores/status';
	import { api } from '$lib/api/client';
	import { copyToClipboard, formatBitrate, formatResolution, getStreamUrl } from '$lib/utils';
	import {
		Radio,
		Tv,
		Copy,
		ExternalLink,
		Settings,
		Zap,
		Server,
		Clock,
		Activity
	} from 'lucide-svelte';

	async function handleCopy(path: string) {
		const url = getStreamUrl(path);
		await copyToClipboard(url);
		toast.success('Stream URL copied to clipboard');
	}

	onMount(async () => {
		try {
			await api.getStatus();
		} catch (e) {
			console.error('Failed to load status:', e);
		}
	});
</script>

<div class="container mx-auto px-4 py-8 max-w-7xl">
	<!-- Header -->
	<header class="mb-12 text-center animate-fade-in">
		<div class="flex items-center justify-center gap-3 mb-4">
			<div class="p-3 rounded-xl bg-primary/10">
				<Radio class="h-8 w-8 text-primary" />
			</div>
			<h1 class="text-4xl font-bold tracking-tight">RelayStation</h1>
		</div>
		<p class="text-muted-foreground text-lg">HLS Streaming Relay & Transcoder</p>

		{#if $serverInfo}
			<div class="flex items-center justify-center gap-6 mt-6 text-sm text-muted-foreground">
				<div class="flex items-center gap-2">
					<Server class="h-4 w-4" />
					<span>{$serverInfo.hostname}</span>
				</div>
				<div class="flex items-center gap-2">
					<Clock class="h-4 w-4" />
					<span>Uptime: {$serverInfo.uptime}</span>
				</div>
				<div class="flex items-center gap-2">
					<Activity class="h-4 w-4" />
					<span>v{$serverInfo.version}</span>
				</div>
			</div>
		{/if}
	</header>

	<!-- Navigation -->
	<nav class="flex justify-center gap-4 mb-8">
		<Button variant="outline" href="/admin">
			<Settings class="h-4 w-4 mr-2" />
			Admin Panel
		</Button>
	</nav>

	<!-- Stream Grid -->
	{#if $isLoading}
		<div class="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
			{#each [1, 2, 3] as _}
				<Card class="p-6 animate-pulse">
					<div class="h-6 bg-muted rounded w-3/4 mb-4"></div>
					<div class="h-4 bg-muted rounded w-1/2 mb-2"></div>
					<div class="h-4 bg-muted rounded w-2/3"></div>
				</Card>
			{/each}
		</div>
	{:else if $streams.length === 0}
		<Card class="p-12 text-center">
			<Tv class="h-16 w-16 mx-auto text-muted-foreground mb-4" />
			<h2 class="text-xl font-semibold mb-2">No Streams Configured</h2>
			<p class="text-muted-foreground mb-4">
				Get started by adding your first stream in the admin panel.
			</p>
			<Button href="/admin">
				<Settings class="h-4 w-4 mr-2" />
				Open Admin Panel
			</Button>
		</Card>
	{:else}
		<div class="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
			{#each $streams as stream (stream.id)}
				<Card class="p-6 hover:shadow-lg transition-shadow animate-fade-in">
					<div class="flex items-start justify-between mb-4">
						<div>
							<h3 class="text-lg font-semibold">{stream.name}</h3>
							<p class="text-sm text-muted-foreground font-mono truncate max-w-[200px]">
								{stream.id}
							</p>
						</div>
						<div class="flex items-center gap-2">
							{#if stream.enabled && stream.profiles.some((p) => p.live)}
								<Badge variant="success" class="flex items-center gap-1">
									<span class="relative flex h-2 w-2">
										<span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
										<span class="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
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

					<div class="mb-4 p-3 rounded-lg bg-muted/50">
						<div class="flex items-center gap-2 text-sm">
							<Zap class="h-4 w-4 {stream.upstream_live ? 'text-emerald-500' : 'text-muted-foreground'}" />
							<span class="text-muted-foreground">Upstream:</span>
							<span class={stream.upstream_live ? 'text-emerald-500' : 'text-destructive'}>
								{stream.upstream_live ? 'Connected' : 'Offline'}
							</span>
						</div>
					</div>

					{#if stream.profiles.length > 0}
						<div class="space-y-3">
							{#each stream.profiles as profile (profile.id)}
								<div class="p-3 rounded-lg border bg-background/50 hover:bg-background transition-colors">
									<div class="flex items-center justify-between mb-2">
										<div class="flex items-center gap-2">
											<span class="font-medium text-sm">
												{profile.codec?.toUpperCase() || 'Passthrough'}
											</span>
											{#if profile.resolution}
												<Badge variant="outline" class="text-xs">
													{formatResolution(profile.resolution)}
												</Badge>
											{/if}
										</div>
										{#if profile.live}
											<span class="text-xs text-emerald-500">Streaming</span>
										{:else if profile.running}
											<span class="text-xs text-yellow-500">Buffering</span>
										{:else}
											<span class="text-xs text-muted-foreground">Idle</span>
										{/if}
									</div>

									<div class="flex items-center gap-2 text-xs text-muted-foreground mb-3">
										{#if profile.bitrate}
											<span>{formatBitrate(profile.bitrate)}</span>
										{/if}
										{#if profile.restart_count > 0}
											<span class="text-yellow-500">({profile.restart_count} restarts)</span>
										{/if}
									</div>

									<div class="flex gap-2">
										<Button size="sm" variant="secondary" class="flex-1" on:click={() => handleCopy(profile.path)}>
											<Copy class="h-3 w-3 mr-1" />
											Copy URL
										</Button>
										<Button size="sm" variant="ghost" on:click={() => window.open(getStreamUrl(profile.path), '_blank')}>
											<ExternalLink class="h-3 w-3" />
										</Button>
									</div>
								</div>
							{/each}
						</div>
					{:else}
						<p class="text-sm text-muted-foreground text-center py-4">No profiles configured</p>
					{/if}
				</Card>
			{/each}
		</div>
	{/if}

	<footer class="mt-16 text-center text-sm text-muted-foreground">
		<p>RelayStation - Self-hosted HLS streaming relay</p>
	</footer>
</div>
