<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Banner, Button } from '#lib/ui/index.js';
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

<h1>{m.profile_delete_confirm_title()}</h1>
{#if error}<Banner tone="alert">{error}</Banner>{/if}
<Button scheme="w" variant="solid" onclick={confirm}>{m.profile_delete_confirm_submit()}</Button>
