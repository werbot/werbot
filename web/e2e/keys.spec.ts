import { expect, test } from '@playwright/test';
import en from '../messages/en.json' with { type: 'json' };
import { loginAsOwner } from './support/auth';

const PUBLIC_KEY = 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UJbT3aN1Nl4yHsGRDyAkCvMAEdMP e2e-key';

test('owner adds an SSH key and deletes it', async ({ page }) => {
	await loginAsOwner(page);
	await page.goto('/profile/keys');
	await expect(page.getByText(en.keys_empty)).toBeVisible();

	await page.getByRole('button', { name: en.keys_add }).click();
	const modal = page.getByRole('dialog', { name: en.keys_add });
	await modal.getByLabel(en.keys_name).fill('e2e-key');
	await modal.getByLabel(en.keys_public_key).fill(PUBLIC_KEY);
	await modal.getByRole('button', { name: en.keys_add }).click();
	await expect(modal).toHaveCount(0);
	const row = page.getByRole('row', { name: /e2e-key/ });
	await expect(row).toBeVisible();

	await row.getByRole('button', { name: en.common_more }).click();
	await page.getByRole('menuitem', { name: en.keys_delete }).click();
	const confirm = page.getByRole('dialog', { name: en.keys_delete });
	await expect(confirm.getByText(en.keys_delete_confirm)).toBeVisible();
	await confirm.getByRole('button', { name: en.keys_delete }).click();
	await expect(confirm).toHaveCount(0);
	await expect(page.getByText(en.keys_empty)).toBeVisible();
});
