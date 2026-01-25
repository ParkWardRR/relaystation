<script lang="ts">
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import Card from '$lib/components/ui/card.svelte';
	import Button from '$lib/components/ui/button.svelte';
	import Input from '$lib/components/ui/input.svelte';
	import Switch from '$lib/components/ui/switch.svelte';
	import Badge from '$lib/components/ui/badge.svelte';
	import { api, type Stream, type Preset } from '$lib/api/client';
	import { Plus, Trash2, Zap, RefreshCw } from 'lucide-svelte';

	let streams: Stream[] = [];
	let presets: Preset[] = [];
	let loading = true;
	let showAddModal = false;

	let newStream = {
		id: '',
		name: '',
		upstream: '',
		preset: 'legacy_ipad'
	};

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

	async function createStream() {
		if (!newStream.id || !newStream.upstream) {
			toast.error('ID and upstream URL are required');
			return;
		}

		try {
			await api.createStream({
				id: newStream.id,
				name: newStream.name || newStream.id,
				upstream: newStream.upstream,
				preset: newStream.preset
			});
			toast.success('Stream created');
			showAddModal = false;
			newStream = { id: '', name: '', upstream: '', preset: 'legacy_ipad' };
			await loadData();
		} catch (e: any) {
			toast.error(e.message || 'Failed to create stream');
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

	async function probeSource(stream: Stream) {
		try {
			const info = await api.getSourceInfo(stream.id);
			toast.success(`Source: ${info.max_quality} (${info.variants.length} variants)`);
		} catch (e) {
			toast.error('Failed to probe source');
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
			<Button on:click={() => (showAddModal = true)}>
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
			<Button on:click={() => (showAddModal = true)}>
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
							<Button variant="ghost" size="icon" on:click={() => probeSource(stream)}>
								<Zap class="h-4 w-4" />
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

{#if showAddModal}
	<div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50" on:click={() => (showAddModal = false)}>
		<Card class="w-full max-w-md p-6 m-4" on:click={(e) => e.stopPropagation()}>
			<h3 class="text-lg font-semibold mb-4">Add New Stream</h3>

			<div class="space-y-4">
				<div>
					<label for="stream-id" class="text-sm font-medium mb-1 block">Stream ID</label>
					<Input id="stream-id" placeholder="my-stream" bind:value={newStream.id} />
					<p class="text-xs text-muted-foreground mt-1">Lowercase, no spaces</p>
				</div>

				<div>
					<label for="stream-name" class="text-sm font-medium mb-1 block">Display Name</label>
					<Input id="stream-name" placeholder="My Stream" bind:value={newStream.name} />
				</div>

				<div>
					<label for="stream-upstream" class="text-sm font-medium mb-1 block">Upstream URL</label>
					<Input id="stream-upstream" placeholder="https://example.com/stream.m3u8" bind:value={newStream.upstream} />
				</div>

				<div>
					<label for="stream-preset" class="text-sm font-medium mb-1 block">Initial Preset</label>
					<select
						id="stream-preset"
						bind:value={newStream.preset}
						class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
					>
						{#each presets.filter((p) => p.builtin) as preset}
							<option value={preset.id}>{preset.name}</option>
						{/each}
					</select>
				</div>
			</div>

			<div class="flex justify-end gap-2 mt-6">
				<Button variant="outline" on:click={() => (showAddModal = false)}>Cancel</Button>
				<Button on:click={createStream}>Create Stream</Button>
			</div>
		</Card>
	</div>
{/if}
