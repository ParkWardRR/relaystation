<script lang="ts">
	import { cn } from '$lib/utils';

	export let type: 'text' | 'number' | 'password' | 'email' | 'url' = 'text';
	export let value: string | number = '';
	export let placeholder = '';
	export let disabled = false;
	let className = '';
	export { className as class };
	export let id: string | undefined = undefined;
	export let name: string | undefined = undefined;
	export let min: string | number | undefined = undefined;
	export let max: string | number | undefined = undefined;

	const inputClass = cn(
		'flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50',
		className
	);
</script>

{#if type === 'number'}
	<input
		type="number"
		bind:value
		{placeholder}
		{disabled}
		{id}
		{name}
		{min}
		{max}
		class={inputClass}
		on:input
	/>
{:else if type === 'password'}
	<input
		type="password"
		bind:value
		{placeholder}
		{disabled}
		{id}
		{name}
		class={inputClass}
		on:input
	/>
{:else if type === 'email'}
	<input
		type="email"
		bind:value
		{placeholder}
		{disabled}
		{id}
		{name}
		class={inputClass}
		on:input
	/>
{:else if type === 'url'}
	<input
		type="url"
		bind:value
		{placeholder}
		{disabled}
		{id}
		{name}
		class={inputClass}
		on:input
	/>
{:else}
	<input
		type="text"
		bind:value
		{placeholder}
		{disabled}
		{id}
		{name}
		class={inputClass}
		on:input
	/>
{/if}
