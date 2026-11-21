<script lang="ts">
	import { afterNavigate } from '$app/navigation';
	import { scale } from 'svelte/transition';
	import { cubicOut } from 'svelte/easing';
	import { Avatar, Icon, MenuItem } from '#lib/ui/index.js';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		open: boolean;
		user: { name: string; email: string };
		onlogout: () => void;
		/** gets focus back when the menu is dismissed (Escape, outside click) */
		trigger?: HTMLElement;
	}

	let { open = $bindable(), user, onlogout, trigger }: Props = $props();

	const close = () => (open = false);

	// the backdrop leaves the rail clickable: a rail click navigates, so the menu must not stay open
	afterNavigate(close);

	function dismiss() {
		close();
		trigger?.focus();
	}

	// focus lands on the first item when the menu opens
	const focusFirst = (node: HTMLElement) => node.querySelector<HTMLElement>('[role="menuitem"]')?.focus();
</script>

<svelte:window
	onkeydown={(e) => {
		if (open && e.key === 'Escape') dismiss();
	}}
/>

{#if open}
	<div class="pfm__backdrop" onclick={dismiss} role="presentation"></div>

	<div {@attach focusFirst} class="pfm" transition:scale={{ duration: 150, start: 0.95, easing: cubicOut }}>
		<!-- ── head (.menu__head-profile) ─────────── -->
		<header class="pfm__head">
			<span class="pfm__avatar wb-me"><Avatar name={user.name} size={48} /></span>
			<div class="pfm__who">
				<span class="pfm__name">{user.name}</span>
				<span class="pfm__email">{user.email}</span>
			</div>
		</header>

		<div class="pfm__menu" role="menu">
			<div class="pfm__group">
				<MenuItem icon="person" iconSize={20} href="/profile" onclick={close}>{m.nav_profile()}</MenuItem>
				<MenuItem icon="key" iconSize={20} href="/profile/keys" onclick={close}>{m.nav_keys()}</MenuItem>
				<MenuItem icon="laptop" iconSize={20} href="/profile/cli-sessions" onclick={close}>{m.cli_sessions_title()}</MenuItem>
			</div>

			<!-- ── footer (.menu__foot-profile) ───────── -->
			<footer class="pfm__foot">
				<button
					type="button"
					role="menuitem"
					class="pfm__logout"
					onclick={() => {
						close();
						onlogout();
					}}
				>
					{m.auth_logout()}
					<Icon name="logout" size={16} />
				</button>
			</footer>
		</div>
	</div>
{/if}

<style>
	.pfm__backdrop {
		position: fixed;
		inset: 0;
		left: 64px;
		z-index: var(--miso-z-dropdown);
	}

	/* #sidebarProfileMenu — white rounded card anchored to the rail avatar */
	.pfm {
		position: fixed;
		left: 50px;
		bottom: 26px;
		z-index: var(--miso-z-dropdown);
		display: flex;
		flex-direction: column;
		width: 280px;
		border-radius: 16px;
		background: var(--miso-menu-cont);
		box-shadow: var(--miso-shadow-mod-minor);
		transform-origin: left bottom;
	}

	/* live profile menu rows: gap 8, pad 4/8 */
	.pfm :global(.miso-menu-item) {
		gap: 8px;
		padding: 4px 8px;
	}

	/* .menu__head-profile{padding:8px 20px 12px;gap:12px} */
	.pfm__head {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 12px 20px;
	}

	/* .av_nm{border-radius:8px} */
	.pfm__avatar :global(.miso-avatar) {
		border-radius: 8px;
	}

	.pfm__who {
		display: flex;
		flex: 1;
		flex-direction: column;
		gap: 2px;
		min-width: 0;
	}

	/* .menu__head-profile .name{font:ui-18-title-2-semi;color:txt-accent} */
	.pfm__name,
	.pfm__email {
		padding: 0 4px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.pfm__name {
		font: var(--miso-font-ui-18-semi);
		color: var(--miso-txt-accent);
	}

	.pfm__email {
		font: var(--miso-font-ui-12-reg);
		color: var(--miso-txt-secondary);
	}

	.pfm__group {
		display: flex;
		flex-direction: column;
		padding: 0 20px 16px;
	}

	/* ── footer (.menu__foot-profile{padding:20px;border-top:1px}) ── */
	.pfm__foot {
		display: flex;
		justify-content: flex-end;
		padding: 12px 20px 16px;
		border-top: 1px solid var(--miso-line-primary);
	}

	.pfm__logout {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		height: 40px;
		padding: 0 var(--miso-sp-12);
		border: 0;
		border-radius: var(--miso-radius-08);
		background: none;
		font: var(--miso-font-ui-14-reg);
		color: var(--miso-txt-secondary);
		cursor: pointer;
	}

	.pfm__logout :global(.miso-icon) {
		color: var(--miso-txt-alert);
	}

	.pfm__logout:hover {
		background: var(--miso-menu-item-hover);
	}
</style>
