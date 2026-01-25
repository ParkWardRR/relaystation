<script lang="ts">
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import Card from '$lib/components/ui/card.svelte';
	import Button from '$lib/components/ui/button.svelte';
	import Badge from '$lib/components/ui/badge.svelte';
	import { api, type Preset } from '$lib/api/client';
	import { Trash2, Cpu, Lock } from 'lucide-svelte';
	import { formatBitrate, formatResolution } from '$lib/utils';

	let presets: Preset[] = [];
	let loading = true;

	$: builtinPresets = presets.filter((p) => p.builtin);
	$: customPresets = presets.filter((p) => !p.builtin);

	onMount(async () => {
		await loadPresets();
	});

	async function loadPresets() {
		loading = true;
		try {
			presets = await api.getPresets();
		} catch (e) {
			toast.error('Failed to load presets');
		}
		loading = false;
	}

	async function deletePreset(preset: Preset) {
		if (!confirm(`Delete preset "${preset.name}"?`)) return;

		try {
			await api.deletePreset(preset.id);
			toast.success('Preset deleted');
			await loadPresets();
		} catch (e) {
			toast.error('Failed to delete preset');
		}
	}

	function getCodecColor(codec: string): string {
		switch (codec?.toLowerCase()) {
			case 'h264':
				return 'bg-blue-500/20 text-blue-400';
			case 'h265':
			case 'hevc':
				return 'bg-purple-500/20 text-purple-400';
			default:
				return 'bg-gray-500/20 text-gray-400';
		}
	}
</script>

<div class="space-y-8">
	<!-- Built-in Presets -->
	<section>
		<div class="flex items-center gap-2 mb-4">
			<h2 class="text-xl font-semibold">Built-in Presets</h2>
			<Badge variant="secondary" class="flex items-center gap-1">
				<Lock class="h-3 w-3" />
				Read-only
			</Badge>
		</div>

		{#if loading}
			<div class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
				{#each [1, 2, 3, 4, 5, 6] as _}
					<Card class="p-4 animate-pulse">
						<div class="h-5 bg-muted rounded w-2/3 mb-2"></div>
						<div class="h-4 bg-muted rounded w-1/2"></div>
					</Card>
				{/each}
			</div>
		{:else}
			<div class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
				{#each builtinPresets as preset (preset.id)}
					<Card class="p-4 border-primary/20 bg-primary/5">
						<div class="flex items-start justify-between mb-2">
							<div>
								<h3 class="font-semibold">{preset.name}</h3>
								<p class="text-xs text-muted-foreground">{preset.subtitle}</p>
							</div>
							<Badge class={getCodecColor(preset.codec)}>
								{preset.codec?.toUpperCase() || 'Copy'}
							</Badge>
						</div>

						<div class="flex flex-wrap gap-2 mt-3 text-xs">
							{#if preset.resolution}
								<Badge variant="outline">{formatResolution(preset.resolution)}</Badge>
							{/if}
							{#if preset.bitrate}
								<Badge variant="outline">{formatBitrate(preset.bitrate)}</Badge>
							{/if}
							{#if preset.fps}
								<Badge variant="outline">{preset.fps}fps</Badge>
							{/if}
						</div>

						{#if preset.description}
							<p class="text-xs text-muted-foreground mt-3 line-clamp-2">
								{preset.description}
							</p>
						{/if}
					</Card>
				{/each}
			</div>
		{/if}
	</section>

	<!-- Custom Presets -->
	<section>
		<div class="flex items-center justify-between mb-4">
			<h2 class="text-xl font-semibold">Custom Presets</h2>
		</div>

		{#if customPresets.length === 0}
			<Card class="p-8 text-center">
				<Cpu class="h-12 w-12 mx-auto text-muted-foreground mb-3" />
				<p class="text-muted-foreground">No custom presets yet.</p>
				<p class="text-sm text-muted-foreground mt-1">
					Custom presets can be created via the API.
				</p>
			</Card>
		{:else}
			<div class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
				{#each customPresets as preset (preset.id)}
					<Card class="p-4">
						<div class="flex items-start justify-between mb-2">
							<div>
								<h3 class="font-semibold">{preset.name}</h3>
								<p class="text-xs text-muted-foreground font-mono">{preset.id}</p>
							</div>
							<div class="flex items-center gap-2">
								<Badge class={getCodecColor(preset.codec)}>
									{preset.codec?.toUpperCase() || 'Copy'}
								</Badge>
								<Button
									variant="ghost"
									size="icon"
									class="h-8 w-8 text-destructive"
									on:click={() => deletePreset(preset)}
								>
									<Trash2 class="h-4 w-4" />
								</Button>
							</div>
						</div>

						<div class="flex flex-wrap gap-2 mt-3 text-xs">
							{#if preset.resolution}
								<Badge variant="outline">{formatResolution(preset.resolution)}</Badge>
							{/if}
							{#if preset.bitrate}
								<Badge variant="outline">{formatBitrate(preset.bitrate)}</Badge>
							{/if}
							{#if preset.fps}
								<Badge variant="outline">{preset.fps}fps</Badge>
							{/if}
						</div>
					</Card>
				{/each}
			</div>
		{/if}
	</section>
</div>
