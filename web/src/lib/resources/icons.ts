import type { ResourceType } from '#lib/gen/werbot/v1/resource_pb.js';
import { resolveIcon } from '#lib/ui/icons.js';
import type { IconName } from '#lib/ui/index.js';
import { m } from '#lib/paraglide/messages.js';

// Registry icons for now: swap the values here when the dedicated icons land.
const FALLBACK: IconName = 'powerplug';

const categories: Record<string, IconName> = {
	servers: 'desktop',
	databases: 'storage',
	applications: 'globe',
	desktops: 'mobile_code',
	containers: 'add_box',
	clouds: 'cloud'
};

const typeIcons: Record<string, IconName> = {
	ssh: 'editor/code_block',
	tcp: 'link',
	telnet: 'editor/code',
	udp: 'link_alt'
};

export const categoryIcon = (category: string): IconName => categories[category] ?? FALLBACK;
export const typeIcon = (type: string): IconName => typeIcons[type] ?? FALLBACK;

/** A preset names a registry icon; anything unknown shows its base type's icon. */
export const resourceTypeIcon = (t: Pick<ResourceType, 'name' | 'baseType' | 'icon'>): IconName => (t.icon && resolveIcon(t.icon) ? t.icon : typeIcon(t.baseType || t.name));

/** Types grouped by category, categories in first-seen order. */
export function groupTypes(types: ResourceType[]): { category: string; types: ResourceType[] }[] {
	const groups = new Map<string, ResourceType[]>();
	for (const t of types) groups.set(t.category, [...(groups.get(t.category) ?? []), t]);
	return [...groups].map(([category, types]) => ({ category, types }));
}

const categoryNames: Record<string, () => string> = {
	servers: m.resources_category_servers,
	databases: m.resources_category_databases,
	applications: m.resources_category_applications,
	desktops: m.resources_category_desktops,
	containers: m.resources_category_containers,
	clouds: m.resources_category_clouds
};

/** Display name of a category; an unknown one shows its raw name. */
export const categoryLabel = (category: string) => categoryNames[category]?.() ?? category;
