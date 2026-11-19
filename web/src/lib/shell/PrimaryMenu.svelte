<script lang="ts">
	import { Avatar, Icon, Tooltip, resolveIcon } from '#lib/ui/index.js';
	import type { IconName } from '#lib/ui/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import type { NavSection, SectionId } from './nav.js';
	import ProfileMenu from './ProfileMenu.svelte';

	interface Props {
		sections: NavSection[];
		active: SectionId;
		user: { name: string; email: string };
		onlogout: () => void;
	}

	let { sections, active, user, onlogout }: Props = $props();

	let profileOpen = $state(false);
	let avatar = $state<HTMLButtonElement>();
	let rail = $derived(sections.filter((s) => s.id !== 'profile'));

	// the solid variant marks the active item, when the icon set has one
	const activeIcon = (name: IconName): IconName => (resolveIcon(`${name}-solid`) ? `${name}-solid` : name);
</script>

<aside class="pm">
	<a class="pm__head" href="/" aria-label="Werbot"><img src="/logo.svg" alt="" width="28" height="15" /></a>

	<nav class="pm__content">
		{#each rail as section (section.id)}
			<Tooltip text={section.label()} placement="right">
				<a
					class={['pm__item', active === section.id && 'pm__item--active']}
					href={section.href}
					aria-label={section.label()}
					aria-current={active === section.id ? 'page' : undefined}
				>
					<Icon name={active === section.id ? activeIcon(section.icon) : section.icon} size={20} />
				</a>
			</Tooltip>
		{/each}
	</nav>

	<div class="pm__foot">
		<Tooltip text={user.name} placement="right">
			<button
				bind:this={avatar}
				type="button"
				class={['pm__profile', (profileOpen || active === 'profile') && 'pm__profile--active']}
				aria-label={m.nav_profile()}
				aria-haspopup="menu"
				aria-expanded={profileOpen}
				onclick={() => (profileOpen = !profileOpen)}
			>
				<Avatar name={user.name} size={24} />
			</button>
		</Tooltip>
	</div>
</aside>

<ProfileMenu bind:open={profileOpen} {user} {onlogout} trigger={avatar} />

<style>
	/* .primaryMenu__wrapper{width:64px;background:sidebar-cont-base} */
	.pm {
		display: flex;
		flex-direction: column;
		justify-content: space-between;
		align-items: center;
		width: 64px;
		height: 100%;
		background: var(--miso-sidebar-base);
	}

	.pm__content {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 4px;
		flex: 1;
	}

	/* .primaryMenu__head{40px;padding:12px;margin-bottom:10px} */
	.pm__head {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 40px;
		height: 40px;
		margin: 12px 0 10px;
		border-radius: 12px;
	}

	/* .primaryMenu__item{20px;padding:6px;border-radius:8px} — 34px total incl. border */
	.pm__item {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 34px;
		height: 34px;
		padding: 6px;
		border: 1px solid transparent;
		border-radius: 8px;
		box-sizing: border-box;
		color: var(--miso-sidebar-ico-base);
		transition: all 0.3s ease-in;
	}

	.pm__item:hover:not(.pm__item--active) {
		background: var(--miso-sidebar-hover);
		color: var(--miso-sidebar-ico-hover);
	}

	.pm__head:focus-visible,
	.pm__item:focus-visible,
	.pm__profile:focus-visible {
		outline: 2px solid var(--miso-sidebar-ico-active);
		outline-offset: 2px;
	}

	.pm__item--active {
		background: var(--miso-sidebar-active);
		box-shadow: 0 0 3px 0 var(--miso-dark-a20) inset;
		color: var(--miso-sidebar-ico-active);
	}

	/* .primaryMenu__foot{width:48px;margin:16px auto} */
	.pm__foot {
		display: flex;
		flex-direction: column;
		align-items: center;
		width: 48px;
		margin: 16px auto;
	}

	.pm__profile {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 36px;
		height: 36px;
		border: 0;
		border-radius: 8px;
		background: none;
		cursor: pointer;
		transition: all 0.1s ease-in;
	}

	.pm__profile:hover,
	.pm__profile--active {
		background: var(--wb-brand-600);
	}
</style>
