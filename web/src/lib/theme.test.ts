import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

const css = readFileSync(new URL('./theme.css', import.meta.url), 'utf8');

// resolves --name: #hex or --name: var(--other)
function token(name: string): string {
	const v = new RegExp(`${name}:\\s*([^;]+);`).exec(css)?.[1].trim() ?? '';
	const ref = /^var\((--[\w-]+)\)$/.exec(v);
	return ref ? token(ref[1]) : v;
}

function luminance(hex: string): number {
	const [r, g, b] = [1, 3, 5].map((i) => {
		const c = parseInt(hex.slice(i, i + 2), 16) / 255;
		return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
	});
	return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contrast(a: string, b: string): number {
	const [x, y] = [luminance(a), luminance(b)].sort((p, q) => q - p);
	return (x + 0.05) / (y + 0.05);
}

describe('brand theme contrast', () => {
	it('base step is the logo gold', () => {
		expect(token('--wb-brand-700')).toBe('#f6c90c');
	});
	it('checked toggle track is the brand gold', () => {
		expect(token('--miso-btn-togg-on')).toBe(token('--wb-brand-700'));
	});
	it('link text on white >= 4.5', () => {
		expect(contrast(token('--miso-txt-link'), '#ffffff')).toBeGreaterThanOrEqual(4.5);
		expect(contrast(token('--miso-txt-link-major'), '#ffffff')).toBeGreaterThanOrEqual(4.5);
	});
	it('on-brand text on the brand base >= 4.5', () => {
		expect(contrast(token('--wb-on-brand'), token('--wb-brand-700'))).toBeGreaterThanOrEqual(4.5);
	});
	it('on-brand text on the hover and pill fills >= 4.5', () => {
		for (const t of ['--miso-hlt-invert', '--miso-hlt-invert-major', '--miso-sidebar-badge-upcoming'])
			expect(contrast(token('--wb-on-brand'), token(t))).toBeGreaterThanOrEqual(4.5);
	});
	it('white initials on the recolored avatar slots >= 3', () => {
		for (const t of ['--miso-av-2', '--miso-av-3']) expect(contrast('#ffffff', token(t))).toBeGreaterThanOrEqual(3);
	});
	it('focus ring vs white >= 3', () => {
		expect(contrast(token('--wb-focus'), '#ffffff')).toBeGreaterThanOrEqual(3);
	});
});
