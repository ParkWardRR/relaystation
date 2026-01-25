<script lang="ts">
	import { page } from '$app/stores';
	import Button from '$lib/components/ui/button.svelte';
	import { Radio, ArrowLeft, List, Sliders, Settings } from 'lucide-svelte';

	const tabs = [
		{ href: '/admin', label: 'Streams', icon: List },
		{ href: '/admin/presets', label: 'Presets', icon: Sliders },
		{ href: '/admin/settings', label: 'Settings', icon: Settings }
	];
</script>

<div class="container mx-auto px-4 py-8 max-w-6xl">
	<!-- Header -->
	<header class="mb-8">
		<div class="flex items-center gap-4 mb-6">
			<Button variant="ghost" size="icon" href="/">
				<ArrowLeft class="h-5 w-5" />
			</Button>
			<div class="flex items-center gap-3">
				<div class="p-2 rounded-lg bg-primary/10">
					<Radio class="h-6 w-6 text-primary" />
				</div>
				<div>
					<h1 class="text-2xl font-bold">Admin Panel</h1>
					<p class="text-sm text-muted-foreground">Manage streams and configuration</p>
				</div>
			</div>
		</div>

		<!-- Tabs -->
		<nav class="flex gap-1 p-1 bg-muted rounded-lg w-fit">
			{#each tabs as tab}
				{@const isActive = $page.url.pathname === tab.href}
				<a
					href={tab.href}
					class="flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-colors
						{isActive
						? 'bg-background text-foreground shadow-sm'
						: 'text-muted-foreground hover:text-foreground'}"
				>
					<svelte:component this={tab.icon} class="h-4 w-4" />
					{tab.label}
				</a>
			{/each}
		</nav>
	</header>

	<!-- Content -->
	<main>
		<slot />
	</main>
</div>
