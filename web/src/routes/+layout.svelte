<script lang="ts">
	import '../app.css';
	import { onMount, onDestroy } from 'svelte';
	import { Toaster } from 'svelte-sonner';
	import { connectWebSocket, disconnectWebSocket } from '$lib/stores/status';

	onMount(() => {
		connectWebSocket();
	});

	onDestroy(() => {
		disconnectWebSocket();
	});
</script>

<svelte:head>
	<title>RelayStation</title>
</svelte:head>

<div class="min-h-screen bg-gradient-to-br from-background via-background to-muted/20">
	<slot />
</div>

<Toaster richColors position="bottom-right" />
