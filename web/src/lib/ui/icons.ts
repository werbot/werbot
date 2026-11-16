/**
 * miso-ui icon registry — the full original Miso icon set, generated
 * 1:1 from the product SVG sprites (791 icons across 11 sprites).
 *
 * Naming follows the original sprites:
 * - core icons have plain names: `done`, `close`, `star`, `tune`, …
 * - namespaced sprites use a prefix: `arrows/chevron-down`, `editor/bold`,
 *   `props/date`, `roles/owner`, `color/…`, `social/…`, `tools/…`
 *
 * Compact legacy aliases (`x` → `close`, `trash` → `trash_can`, …) are kept
 * for backwards compatibility.
 */
import { coreIcons } from './icons/core.js';
import { arrowIcons } from './icons/arrows.js';
import { editorIcons } from './icons/editor.js';
import { propIcons } from './icons/props.js';
import { roleIcons } from './icons/roles.js';
import { colorIcons } from './icons/color.js';
import { socialIcons } from './icons/social.js';
import { toolIcons } from './icons/tools.js';
import { helperIcons } from './icons/helper.js';
import { taskWorkflowIcons } from './icons/task_workflow.js';
import { prjWorkflowIcons } from './icons/prj_workflow.js';

export interface IconDef {
	/** inner svg markup copied verbatim from the sprite */
	body: string;
	/** defaults to '0 0 20 20' */
	viewBox?: string;
	/** icon carries its own fill colors and must not be recolored */
	colored?: boolean;
}

export const icons: Record<string, IconDef> = {
	...coreIcons,
	...arrowIcons,
	...editorIcons,
	...propIcons,
	...roleIcons,
	...colorIcons,
	...socialIcons,
	...toolIcons,
	...helperIcons,
	...taskWorkflowIcons,
	...prjWorkflowIcons
};

/** Legacy miso-ui names mapped onto original sprite ids. */
export const iconAliases: Record<string, string> = {
	x: 'close',
	'chevron-down': 'arrows/chevron-down',
	'chevron-up': 'arrows/chevron-up',
	'chevron-left': 'arrows/chevron-left',
	'chevron-right': 'arrows/chevron-right',
	'arrow-right': 'arrows/arrow-right',
	'arrow-left': 'arrows/arrow-left',
	clock: 'time',
	timer: 'stopwatch',
	user: 'person',
	users: 'people',
	flag: 'priority',
	'more-h': 'more-horiz',
	'more-v': 'more-vert',
	trash: 'trash_can',
	attach: 'attach-vert',
	comment: 'comment_bubble',
	folder: 'folder-close',
	play: 'timer-play',
	pause: 'timer-pause',
	warning: 'exclam_triangle-line',
	info: 'info_circle-line',
	question: 'help',
	bell: 'notifications',
	eye: 'eye-open',
	board: 'kanban',
	inbox: 'input',
	refresh: 'autorenew',
	'circle-check': 'check_circle',
	'circle-alert': 'exclam_triangle-line',
	circle: 'input-circle-blank',
	dot: 'shape-circle-6',
	grip: 'drag_indicator',
	copy: 'content_copy',
	external: 'open_in_new',
	document: 'file',
	gantt: 'gantt_chart',
	dashboard: 'grid_view',
	lightning: 'bolt',
	pin: 'location',
	// original product aliases (from the components.js alias table)
	hint: 'info_circle-line',
	'hint-exclam': 'exclam_triangle-solid',
	checkbox: 'input-box-blank',
	radio: 'input-circle-blank',
	meet: 'alarm_clock',
	'arrows/chevron-forward': 'arrows/chevron-right',
	'social/apple': 'social/apple_for_day'
};

/** Icon name: any original sprite id or a legacy alias. */
export type IconName = string;

export function resolveIcon(name: IconName): IconDef | undefined {
	return icons[name] ?? icons[iconAliases[name]];
}

/** All canonical icon names (without aliases), sprite order preserved. */
export const iconNames: IconName[] = Object.keys(icons);
