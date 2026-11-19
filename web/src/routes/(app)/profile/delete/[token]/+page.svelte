<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Button, InfoBlock, SettingsCard } from '#lib/ui/index.js';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import { profile } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';

	async function confirm() {
		try {
			await profile.confirmAccountDeletion({ token: page.params.token });
			await goto('/login');
		} catch (e) {
			failToast(e);
		}
	}
</script>

<PageHeader title={m.profile_delete_confirm_title()} />
<PageBody width={680}>
	<SettingsCard icon="trash_can" title={m.profile_delete_confirm_title()}>
		<div class="body">
			<InfoBlock tone="warning" icon="exclam_triangle-line" title={m.common_warning()}>{m.profile_delete_confirm_warning()}</InfoBlock>
			<div><Button size={40} scheme="w" variant="solid" onclick={confirm}>{m.profile_delete_confirm_submit()}</Button></div>
		</div>
	</SettingsCard>
</PageBody>

<style>
	.body {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
	}
</style>
