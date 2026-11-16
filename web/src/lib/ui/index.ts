// miso-ui — Miso-style design system for Svelte 5

// styles (import 'miso-ui/styles' side-effect in the consuming app)
import './styles/tokens.css';
import './styles/base.css';

// icons
export {
	icons,
	iconNames,
	iconAliases,
	resolveIcon,
	type IconName,
	type IconDef
} from './icons.js';

// primitives
export { default as Icon } from './components/Icon.svelte';
export {
	default as Button,
	type ButtonScheme,
	type ButtonVariant,
	type ButtonSize
} from './components/Button.svelte';
export { default as ButtonGroup } from './components/ButtonGroup.svelte';
export { default as Checkbox } from './components/Checkbox.svelte';
export { default as Radio } from './components/Radio.svelte';
export { default as Toggle } from './components/Toggle.svelte';
export { default as Spinner } from './components/Spinner.svelte';
export { default as Divider } from './components/Divider.svelte';
export { default as Kbd } from './components/Kbd.svelte';

// data display
export { default as Skeleton } from './components/Skeleton.svelte';
export { default as Avatar, type AvatarColor } from './components/Avatar.svelte';
export { default as AvatarStack } from './components/AvatarStack.svelte';
export { default as Chip, type ChipVariant } from './components/Chip.svelte';
export { default as Label, type LabelColor } from './components/Label.svelte';
export { default as Badge, type BadgeTone } from './components/Badge.svelte';
export { default as PriorityBadge } from './components/PriorityBadge.svelte';
export { default as ProgressBar, type ProgressTone } from './components/ProgressBar.svelte';
export { default as Table } from './components/Table.svelte';
export { default as PropertyRow } from './components/PropertyRow.svelte';
export { default as EmptyState } from './components/EmptyState.svelte';

// forms
export { default as TextField } from './components/TextField.svelte';
export { default as FilledField } from './components/FilledField.svelte';
export { default as Textarea } from './components/Textarea.svelte';
export { default as Select, type SelectOption } from './components/Select.svelte';
export { default as Calendar } from './components/Calendar.svelte';
export { default as InlineEdit } from './components/InlineEdit.svelte';

// interaction
export { default as SortableList } from './components/SortableList.svelte';
export { default as DropZone } from './components/DropZone.svelte';
export { default as FileItem } from './components/FileItem.svelte';
export { default as TimerButton } from './components/TimerButton.svelte';

// overlays
export { default as Dropdown } from './components/Dropdown.svelte';
export { default as Menu } from './components/Menu.svelte';
export { default as MenuItem } from './components/MenuItem.svelte';
export { default as MenuGroup } from './components/MenuGroup.svelte';
export { default as Tooltip } from './components/Tooltip.svelte';
export { default as Modal } from './components/Modal.svelte';
export { default as Drawer } from './components/Drawer.svelte';
export { default as Banner, type BannerTone } from './components/Banner.svelte';
export { default as InfoBlock, type InfoBlockTone } from './components/InfoBlock.svelte';
export { default as SettingsCard } from './components/SettingsCard.svelte';
export { default as Toast } from './components/Toast.svelte';
export { default as Toaster, type ToasterPosition } from './components/Toaster.svelte';
export {
	toast,
	dismissToast,
	toasts,
	type ToastTone,
	type ToastOptions,
	type ToastItem,
	type ToastAction
} from './toaster.svelte.js';

// navigation
export { default as Segmented, type SegmentedOption } from './components/Segmented.svelte';
export { default as Tabs, type TabOption } from './components/Tabs.svelte';
export { default as Breadcrumbs, type Crumb } from './components/Breadcrumbs.svelte';
export { default as Sidebar } from './components/Sidebar.svelte';
export { default as SidebarItem } from './components/SidebarItem.svelte';
