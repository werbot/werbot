// A drawer's Escape listener runs before the select popup's own, so the drawer asks first.
export const selectOpen = () => !!document.querySelector('.miso-select__control--open');
