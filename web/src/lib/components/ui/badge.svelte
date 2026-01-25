<script lang="ts">
	import { cn } from '$lib/utils';

	type Variant = 'default' | 'secondary' | 'destructive' | 'outline' | 'success';

	export let variant: Variant = 'default';
	let className = '';
	export { className as class };

	const variants: Record<Variant, string> = {
		default: 'border-transparent bg-primary text-primary-foreground',
		secondary: 'border-transparent bg-secondary text-secondary-foreground',
		destructive: 'border-transparent bg-destructive text-destructive-foreground',
		outline: 'text-foreground',
		success: 'border-transparent bg-emerald-500/20 text-emerald-400'
	};
</script>

<div
	class={cn(
		'inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold transition-colors',
		variants[variant],
		className
	)}
>
	<slot />
</div>
