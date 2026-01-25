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

<div class="min-h-screen">
	<slot />
</div>

<Toaster richColors position="bottom-right" />
