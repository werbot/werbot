import { defineConfig, devices } from '@playwright/test';

// Smoke test against a running install (release.yml job `smoke`); no webServer.
export default defineConfig({
	testDir: 'smoke',
	testMatch: '*.smoke.ts',
	retries: 1,
	use: { baseURL: process.env.WERBOT_SMOKE_URL ?? 'http://localhost:8080' },
	projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }]
});
