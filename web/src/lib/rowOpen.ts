// Controls in a row keep their own clicks: the "…" menu, selects, toggles, links, and the
// menus and dialogs they open (rendered inside the row).
const own = 'a, button, input, select, textarea, label, [role="menu"], [role="dialog"], .miso-dropdown';

// rowOpen makes a click anywhere on a list row act as a click on the row's name (.row-name).
// Selecting text in the row does not open it.
export function rowOpen(event: MouseEvent) {
	const target = event.target;
	if (!(target instanceof Element) || event.button !== 0 || target.closest(own)) return;
	if (getSelection()?.toString()) return;
	target.closest('tbody tr')?.querySelector<HTMLElement>('.row-name')?.click();
}
