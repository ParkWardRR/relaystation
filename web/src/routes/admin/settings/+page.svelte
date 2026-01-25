<script lang="ts">
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import Card from '$lib/components/ui/card.svelte';
	import Button from '$lib/components/ui/button.svelte';
	import Input from '$lib/components/ui/input.svelte';
	import { api, type Defaults } from '$lib/api/client';
	import { Save, RefreshCw, Cog } from 'lucide-svelte';

	let defaults: Defaults = {
		segment_time: 2,
		playlist_size: 6,
		preset: 'ultrafast'
	};
	let loading = true;
	let saving = false;

	onMount(async () => {
		await loadDefaults();
	});

	async function loadDefaults() {
		loading = true;
		try {
			defaults = await api.getDefaults();
		} catch (e) {
			toast.error('Failed to load settings');
		}
		loading = false;
	}

	async function saveDefaults() {
		saving = true;
		try {
			await api.updateDefaults(defaults);
			toast.success('Settings saved');
		} catch (e) {
			toast.error('Failed to save settings');
		}
		saving = false;
	}

	async function reload() {
		try {
			await api.reload();
			toast.success('Configuration reloaded');
		} catch (e) {
			toast.error('Failed to reload');
		}
	}
</script>

<div class="space-y-6 max-w-2xl">
	<div>
		<h2 class="text-xl font-semibold">Settings</h2>
		<p class="text-sm text-muted-foreground">Global configuration and defaults</p>
	</div>

	{#if loading}
		<Card class="p-6 animate-pulse">
			<div class="space-y-4">
				<div class="h-10 bg-muted rounded"></div>
				<div class="h-10 bg-muted rounded"></div>
				<div class="h-10 bg-muted rounded"></div>
			</div>
		</Card>
	{:else}
		<!-- HLS Settings -->
		<Card class="p-6">
			<h3 class="font-semibold mb-4 flex items-center gap-2">
				<Cog class="h-5 w-5 text-muted-foreground" />
				HLS Settings
			</h3>

			<div class="space-y-4">
				<div>
					<label for="segment-time" class="text-sm font-medium mb-1 block">
						Segment Duration (seconds)
					</label>
					<Input
						id="segment-time"
						type="number"
						min="1"
						max="10"
						bind:value={defaults.segment_time}
					/>
					<p class="text-xs text-muted-foreground mt-1">
						Lower = less latency, higher = more stable. Recommended: 2
					</p>
				</div>

				<div>
					<label for="playlist-size" class="text-sm font-medium mb-1 block">
						Playlist Size (segments)
					</label>
					<Input
						id="playlist-size"
						type="number"
						min="3"
						max="20"
						bind:value={defaults.playlist_size}
					/>
					<p class="text-xs text-muted-foreground mt-1">
						Number of segments in the playlist. Recommended: 6
					</p>
				</div>

				<div>
					<label for="ffmpeg-preset" class="text-sm font-medium mb-1 block">FFmpeg Preset</label>
					<select
						id="ffmpeg-preset"
						bind:value={defaults.preset}
						class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
					>
						<option value="ultrafast">ultrafast (lowest latency)</option>
						<option value="superfast">superfast</option>
						<option value="veryfast">veryfast</option>
						<option value="faster">faster</option>
						<option value="fast">fast</option>
						<option value="medium">medium (balanced)</option>
						<option value="slow">slow (best quality)</option>
					</select>
					<p class="text-xs text-muted-foreground mt-1">
						Encoding speed vs quality tradeoff
					</p>
				</div>
			</div>

			<div class="flex justify-end mt-6">
				<Button on:click={saveDefaults} disabled={saving}>
					<Save class="h-4 w-4 mr-2" />
					{saving ? 'Saving...' : 'Save Settings'}
				</Button>
			</div>
		</Card>

		<!-- System Actions -->
		<Card class="p-6">
			<h3 class="font-semibold mb-4">System Actions</h3>

			<div class="space-y-4">
				<div class="flex items-center justify-between p-4 rounded-lg border bg-muted/30">
					<div>
						<p class="font-medium">Reload Configuration</p>
						<p class="text-sm text-muted-foreground">
							Reload streams.json and restart affected FFmpeg processes
						</p>
					</div>
					<Button variant="outline" on:click={reload}>
						<RefreshCw class="h-4 w-4 mr-2" />
						Reload
					</Button>
				</div>
			</div>
		</Card>
	{/if}
</div>
