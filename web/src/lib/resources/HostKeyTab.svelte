<script lang="ts">
	import { timestampDate } from '@bufbuild/protobuf/wkt';
	import { Button, InfoBlock, PropertyRow, toast } from '#lib/ui/index.js';
	import { resources } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import type { Resource } from '#lib/gen/werbot/v1/resource_pb.js';
	import { m } from '#lib/paraglide/messages.js';

	let { resource = $bindable() }: { resource: Resource } = $props();

	async function clear() {
		try {
			await resources.clearHostKey({ resourceId: resource.id });
			const res = await resources.getResource({ id: resource.id });
			if (res.resource) resource = res.resource;
			toast({ tone: 'success', text: m.common_saved() });
		} catch (e) {
			failToast(e);
		}
	}
</script>

{#if resource.hostKey}
	<PropertyRow label={m.host_key_type()}>{resource.hostKey.keyType}</PropertyRow>
	<PropertyRow label={m.host_key_fingerprint()}>{resource.hostKey.fingerprintSha256}</PropertyRow>
	{#if resource.hostKey.createdAt}
		<PropertyRow label={m.host_key_pinned()}>{timestampDate(resource.hostKey.createdAt).toLocaleString()}</PropertyRow>
	{/if}
	<p><Button size={40} scheme="w" variant="plain" onclick={clear}>{m.host_key_clear()}</Button></p>
{:else}
	<InfoBlock title={m.host_key_pending()} />
{/if}
