<script lang="ts">
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import Card from '$lib/components/ui/card.svelte';
	import Button from '$lib/components/ui/button.svelte';
	import Input from '$lib/components/ui/input.svelte';
	import Switch from '$lib/components/ui/switch.svelte';
	import Badge from '$lib/components/ui/badge.svelte';
	import { api, type Stream, type Preset, type StreamCharacteristics } from '$lib/api/client';
	import { Plus, Trash2, Zap, RefreshCw, Pencil, Loader2, Radio, Tv, Film, Layers, Music, Subtitles, AlertCircle } from 'lucide-svelte';

	let streams: Stream[] = [];
	let presets: Preset[] = [];
	let loading = true;

	// Modal state
	let showModal = false;
	let modalMode: 'add' | 'edit' = 'add';
	let editingStreamId: string | null = null;

	// Form state
	let formData = {
		id: '',
		name: '',
		upstream: '',
		preset: 'legacy_ipad'
	};

	// Source probing state
	let probeLoading = false;
	let probeError: string | null = null;
	let sourceInfo: StreamCharacteristics | null = null;
	let probeDebounceTimer: ReturnType<typeof setTimeout> | null = null;

	onMount(async () => {
		await loadData();
	});

	async function loadData() {
		loading = true;
		try {
			[streams, presets] = await Promise.all([api.getStreams(), api.getPresets()]);
		} catch (e) {
			toast.error('Failed to load data');
		}
		loading = false;
	}

	function openAddModal() {
		modalMode = 'add';
		editingStreamId = null;
		formData = { id: '', name: '', upstream: '', preset: 'legacy_ipad' };
		sourceInfo = null;
		probeError = null;
		showModal = true;
	}

	function openEditModal(stream: Stream) {
		modalMode = 'edit';
		editingStreamId = stream.id;
		formData = {
			id: stream.id,
			name: stream.name,
			upstream: stream.upstream,
			preset: 'legacy_ipad' // Not used in edit mode
		};
		sourceInfo = null;
		probeError = null;
		showModal = true;
		// Auto-probe the existing URL
		probeURL(stream.upstream);
	}

	function closeModal() {
		showModal = false;
		editingStreamId = null;
		sourceInfo = null;
		probeError = null;
		if (probeDebounceTimer) {
			clearTimeout(probeDebounceTimer);
			probeDebounceTimer = null;
		}
	}

	async function probeURL(url: string) {
		if (!url || !url.includes('://')) {
			sourceInfo = null;
			probeError = null;
			return;
		}

		probeLoading = true;
		probeError = null;

		try {
			sourceInfo = await api.probeURL(url);
		} catch (e: any) {
			probeError = e.message || 'Failed to probe URL';
			sourceInfo = null;
		}

		probeLoading = false;
	}

	function handleUpstreamChange(e: Event) {
		const target = e.target as HTMLInputElement;
		formData.upstream = target.value;

		// Debounce the probe
		if (probeDebounceTimer) {
			clearTimeout(probeDebounceTimer);
		}

		probeDebounceTimer = setTimeout(() => {
			probeURL(formData.upstream);
		}, 800);
	}

	async function handleSubmit() {
		if (modalMode === 'add') {
			await createStream();
		} else {
			await updateStream();
		}
	}

	async function createStream() {
		if (!formData.id || !formData.upstream) {
			toast.error('ID and upstream URL are required');
			return;
		}

		try {
			await api.createStream({
				id: formData.id,
				name: formData.name || formData.id,
				upstream: formData.upstream,
				preset: formData.preset
			});
			toast.success('Stream created');
			closeModal();
			await loadData();
		} catch (e: any) {
			toast.error(e.message || 'Failed to create stream');
		}
	}

	async function updateStream() {
		if (!editingStreamId || !formData.upstream) {
			toast.error('Upstream URL is required');
			return;
		}

		try {
			await api.updateStream(editingStreamId, {
				name: formData.name,
				upstream: formData.upstream
			});
			toast.success('Stream updated');
			closeModal();
			await loadData();
		} catch (e: any) {
			toast.error(e.message || 'Failed to update stream');
		}
	}

	async function toggleStream(stream: Stream) {
		try {
			await api.updateStream(stream.id, { enabled: !stream.enabled });
			toast.success(`Stream ${stream.enabled ? 'disabled' : 'enabled'}`);
			await loadData();
		} catch (e) {
			toast.error('Failed to update stream');
		}
	}

	async function deleteStream(stream: Stream) {
		if (!confirm(`Delete stream "${stream.name}"?`)) return;

		try {
			await api.deleteStream(stream.id);
			toast.success('Stream deleted');
			await loadData();
		} catch (e) {
			toast.error('Failed to delete stream');
		}
	}

	function formatBandwidth(bw: number): string {
		if (!bw) return '-';
		if (bw >= 1000000) return `${(bw / 1000000).toFixed(1)} Mbps`;
		if (bw >= 1000) return `${(bw / 1000).toFixed(0)} kbps`;
		return `${bw} bps`;
	}

	function getStreamTypeLabel(type: string): string {
		switch (type) {
			case 'live': return 'Live Stream';
			case 'vod': return 'Video on Demand';
			default: return 'Unknown';
		}
	}

	function getFormatLabel(format: string): string {
		switch (format) {
			case 'fmp4': return 'Fragmented MP4';
			case 'mpegts': return 'MPEG-TS';
			default: return 'Unknown';
		}
	}
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h2 class="text-xl font-semibold">Streams</h2>
			<p class="text-sm text-muted-foreground">Manage your HLS stream sources</p>
		</div>
		<div class="flex gap-2">
			<Button variant="outline" on:click={loadData}>
				<RefreshCw class="h-4 w-4 mr-2" />
				Refresh
			</Button>
			<Button on:click={openAddModal}>
				<Plus class="h-4 w-4 mr-2" />
				Add Stream
			</Button>
		</div>
	</div>

	{#if loading}
		<div class="space-y-4">
			{#each [1, 2] as _}
				<Card class="p-6 animate-pulse">
					<div class="h-6 bg-muted rounded w-1/3 mb-2"></div>
					<div class="h-4 bg-muted rounded w-2/3"></div>
				</Card>
			{/each}
		</div>
	{:else if streams.length === 0}
		<Card class="p-12 text-center">
			<p class="text-muted-foreground mb-4">No streams configured yet.</p>
			<Button on:click={openAddModal}>
				<Plus class="h-4 w-4 mr-2" />
				Add Your First Stream
			</Button>
		</Card>
	{:else}
		<div class="space-y-4">
			{#each streams as stream (stream.id)}
				<Card class="p-6">
					<div class="flex items-center justify-between">
						<div class="flex items-center gap-4">
							<Switch checked={stream.enabled} on:change={() => toggleStream(stream)} />
							<div>
								<div class="flex items-center gap-2">
									<h3 class="font-semibold">{stream.name}</h3>
									<Badge variant="outline" class="font-mono text-xs">{stream.id}</Badge>
								</div>
								<p class="text-sm text-muted-foreground truncate max-w-md">{stream.upstream}</p>
							</div>
						</div>

						<div class="flex items-center gap-2">
							<Badge variant="secondary">
								{Object.keys(stream.profiles || {}).length} profiles
							</Badge>
							<Button variant="ghost" size="icon" on:click={() => openEditModal(stream)} title="Edit stream">
								<Pencil class="h-4 w-4" />
							</Button>
							<Button variant="ghost" size="icon" class="text-destructive" on:click={() => deleteStream(stream)}>
								<Trash2 class="h-4 w-4" />
							</Button>
						</div>
					</div>

					{#if Object.keys(stream.profiles || {}).length > 0}
						<div class="mt-4 pt-4 border-t">
							<p class="text-xs text-muted-foreground mb-2">Profiles:</p>
							<div class="flex flex-wrap gap-2">
								{#each Object.entries(stream.profiles) as [key, profile]}
									<Badge variant={profile.enabled ? 'default' : 'outline'}>
										{key}
										{#if profile.codec}
											- {profile.codec.toUpperCase()}
										{/if}
									</Badge>
								{/each}
							</div>
						</div>
					{/if}
				</Card>
			{/each}
		</div>
	{/if}
</div>

<!-- Add/Edit Modal -->
{#if showModal}
	<div
		class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
		on:click={closeModal}
		on:keydown={(e) => e.key === 'Escape' && closeModal()}
		role="dialog"
		aria-modal="true"
		tabindex="-1"
	>
		<Card class="w-full max-w-lg p-6 m-4 max-h-[90vh] overflow-y-auto" on:click={(e) => e.stopPropagation()}>
			<h3 class="text-lg font-semibold mb-4">
				{modalMode === 'add' ? 'Add New Stream' : 'Edit Stream'}
			</h3>

			<div class="space-y-4">
				{#if modalMode === 'add'}
					<div>
						<label for="stream-id" class="text-sm font-medium mb-1 block">Stream ID</label>
						<Input id="stream-id" placeholder="my-stream" bind:value={formData.id} />
						<p class="text-xs text-muted-foreground mt-1">Lowercase, no spaces (cannot be changed later)</p>
					</div>
				{:else}
					<div>
						<label class="text-sm font-medium mb-1 block">Stream ID</label>
						<div class="flex h-10 items-center px-3 rounded-md border border-input bg-muted/50">
							<span class="font-mono text-sm text-muted-foreground">{formData.id}</span>
						</div>
					</div>
				{/if}

				<div>
					<label for="stream-name" class="text-sm font-medium mb-1 block">Display Name</label>
					<Input id="stream-name" placeholder="My Stream" bind:value={formData.name} />
				</div>

				<div>
					<label for="stream-upstream" class="text-sm font-medium mb-1 block">Upstream URL</label>
					<Input
						id="stream-upstream"
						placeholder="https://example.com/stream.m3u8"
						value={formData.upstream}
						on:input={handleUpstreamChange}
						on:paste={handleUpstreamChange}
					/>
					<p class="text-xs text-muted-foreground mt-1">HLS (.m3u8) stream URL</p>
				</div>

				<!-- Source Info Display -->
				{#if probeLoading}
					<div class="p-4 rounded-lg bg-muted/50 border border-border">
						<div class="flex items-center gap-2 text-sm text-muted-foreground">
							<Loader2 class="h-4 w-4 animate-spin" />
							<span>Analyzing source stream...</span>
						</div>
					</div>
				{:else if probeError}
					<div class="p-4 rounded-lg bg-destructive/10 border border-destructive/20">
						<div class="flex items-center gap-2 text-sm text-destructive">
							<AlertCircle class="h-4 w-4" />
							<span>{probeError}</span>
						</div>
					</div>
				{:else if sourceInfo}
					<div class="p-4 rounded-lg bg-emerald-500/10 border border-emerald-500/20">
						<div class="flex items-center gap-2 mb-3">
							<Zap class="h-4 w-4 text-emerald-500" />
							<span class="text-sm font-medium text-emerald-400">Source Detected</span>
						</div>

						<div class="grid grid-cols-2 gap-3 text-sm">
							<div class="flex items-center gap-2">
								{#if sourceInfo.stream_type === 'live'}
									<Radio class="h-4 w-4 text-emerald-500" />
								{:else}
									<Film class="h-4 w-4 text-blue-500" />
								{/if}
								<div>
									<p class="text-xs text-muted-foreground">Type</p>
									<p class="font-medium">{getStreamTypeLabel(sourceInfo.stream_type)}</p>
								</div>
							</div>

							<div class="flex items-center gap-2">
								<Tv class="h-4 w-4 text-purple-500" />
								<div>
									<p class="text-xs text-muted-foreground">Format</p>
									<p class="font-medium">{getFormatLabel(sourceInfo.segment_format)}</p>
								</div>
							</div>

							{#if sourceInfo.is_multi_variant && sourceInfo.variant_count > 0}
								<div class="flex items-center gap-2">
									<Layers class="h-4 w-4 text-orange-500" />
									<div>
										<p class="text-xs text-muted-foreground">Variants</p>
										<p class="font-medium">{sourceInfo.variant_count} quality levels</p>
									</div>
								</div>
							{/if}

							{#if sourceInfo.max_bandwidth}
								<div class="flex items-center gap-2">
									<Zap class="h-4 w-4 text-yellow-500" />
									<div>
										<p class="text-xs text-muted-foreground">Max Bitrate</p>
										<p class="font-medium">{formatBandwidth(sourceInfo.max_bandwidth)}</p>
									</div>
								</div>
							{/if}

							{#if sourceInfo.max_resolution}
								<div class="flex items-center gap-2">
									<Tv class="h-4 w-4 text-cyan-500" />
									<div>
										<p class="text-xs text-muted-foreground">Max Resolution</p>
										<p class="font-medium">{sourceInfo.max_resolution}</p>
									</div>
								</div>
							{/if}

							{#if sourceInfo.target_duration}
								<div class="flex items-center gap-2">
									<Film class="h-4 w-4 text-pink-500" />
									<div>
										<p class="text-xs text-muted-foreground">Segment Duration</p>
										<p class="font-medium">{sourceInfo.target_duration}s</p>
									</div>
								</div>
							{/if}
						</div>

						<div class="flex gap-4 mt-3 pt-3 border-t border-emerald-500/20">
							{#if sourceInfo.has_audio}
								<div class="flex items-center gap-1 text-xs text-emerald-400">
									<Music class="h-3 w-3" />
									<span>Audio</span>
								</div>
							{/if}
							{#if sourceInfo.has_subtitles}
								<div class="flex items-center gap-1 text-xs text-emerald-400">
									<Subtitles class="h-3 w-3" />
									<span>Subtitles</span>
								</div>
							{/if}
						</div>
					</div>
				{/if}

				{#if modalMode === 'add'}
					<div>
						<label for="stream-preset" class="text-sm font-medium mb-1 block">Initial Preset</label>
						<select
							id="stream-preset"
							bind:value={formData.preset}
							class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
						>
							{#each presets.filter((p) => p.builtin) as preset}
								<option value={preset.id}>{preset.name}</option>
							{/each}
						</select>
						<p class="text-xs text-muted-foreground mt-1">Transcoding preset for the initial output profile</p>
					</div>
				{/if}
			</div>

			<div class="flex justify-end gap-2 mt-6">
				<Button variant="outline" on:click={closeModal}>Cancel</Button>
				<Button on:click={handleSubmit} disabled={probeLoading}>
					{modalMode === 'add' ? 'Create Stream' : 'Save Changes'}
				</Button>
			</div>
		</Card>
	</div>
{/if}
