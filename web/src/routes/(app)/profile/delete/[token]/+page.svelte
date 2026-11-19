<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Banner, Button, InfoBlock, SettingsCard } from '#lib/ui/index.js';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import { profile } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';

	let error = $state('');

	async function confirm() {
		error = '';
		try {
			await profile.confirmAccountDeletion({ token: page.params.token });
			await goto('/login');
		} catch (e) {
			error = errorMessage(e);
		}
	}
</script>

<PageHeader title={m.profile_delete_confirm_title()} />
{#if error}<Banner tone="alert">{error}</Banner>{/if}
<div class="card">
	<SettingsCard icon="trash_can" title={m.profile_delete_confirm_title()}>
		<div class="body">
			<InfoBlock tone="warning">{m.profile_delete_confirm_warning()}</InfoBlock>
			<div><Button scheme="w" variant="solid" onclick={confirm}>{m.profile_delete_confirm_submit()}</Button></div>
		</div>
	</SettingsCard>
</div>

<style>
	.card {
		max-width: 480px;
	}

	.body {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}
</style>
