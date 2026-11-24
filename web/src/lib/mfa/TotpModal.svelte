<script lang="ts">
	import { Button, FilledField, Modal } from '#lib/ui/index.js';
	import { mfa } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import type { EnrollTOTPResponse } from '#lib/gen/werbot/v1/mfa_pb.js';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		/** the pending enrollment; null closes the dialog */
		enrollment: EnrollTOTPResponse | null;
		onclose: () => void;
		onconfirmed: () => void;
	}

	let { enrollment, onclose, onconfirmed }: Props = $props();

	let code = $state('');

	async function confirm(e: SubmitEvent) {
		e.preventDefault();
		try {
			await mfa.confirmTOTP({ code: code.trim() });
			code = '';
			onconfirmed();
		} catch (err) {
			failToast(err);
		}
	}
</script>

<Modal open={enrollment !== null} {onclose} title={m.mfa_totp_title()} width={440}>
	{#if enrollment}
		<form id="confirm-totp" onsubmit={confirm}>
			<p>{m.mfa_totp_scan()}</p>
			<!-- dark modules on a white ground with a quiet zone, whatever the theme -->
			<svg class="qr" viewBox="-2 -2 {enrollment.qrSize + 4} {enrollment.qrSize + 4}" shape-rendering="crispEdges" role="img" aria-label={m.mfa_totp_qr()}>
				<rect x="-2" y="-2" width={enrollment.qrSize + 4} height={enrollment.qrSize + 4} fill="#fff" />
				<path d={enrollment.qrPath} fill="#000" />
			</svg>
			<p class="muted">{m.mfa_totp_manual()}</p>
			<code class="secret">{enrollment.secret}</code>
			<FilledField label={m.auth_mfa_totp_code()} bind:value={code} required inputmode="numeric" pattern={'[0-9]{6}'} maxlength={6} autocomplete="one-time-code" />
		</form>
	{/if}
	{#snippet footer()}
		<Button size={40} scheme="g" variant="plain" onclick={onclose}>{m.common_cancel()}</Button>
		<Button size={40} type="submit" form="confirm-totp" variant="solid">{m.mfa_totp_confirm()}</Button>
	{/snippet}
</Modal>

<style>
	form {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
	}

	p {
		margin: 0;
	}

	.muted {
		font: var(--miso-font-ui-12-reg);
		color: var(--miso-txt-tertiary);
	}

	.qr {
		align-self: center;
		width: 176px;
		height: 176px;
		border-radius: var(--miso-radius-08);
	}

	.secret {
		align-self: center;
		font-size: 14px;
		letter-spacing: 0.08em;
		overflow-wrap: anywhere;
		user-select: all;
	}
</style>
