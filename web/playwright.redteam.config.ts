import { defineConfig } from '@playwright/test';
import base from './playwright.config';

// The red-team browser scenarios: the base server on :18080 (database from
// WERBOT_E2E_DATABASE_URL), no sshtarget or telnettarget, one project.
export default defineConfig({
	...base,
	testDir: 'redteam',
	testMatch: '*.redteam.ts',
	projects: [{ name: 'chromium', use: { ...base.projects![0].use } }],
	webServer: [[base.webServer].flat()[0]!]
});
