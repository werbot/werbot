import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { defineConfig, devices } from '@playwright/test';

const dbURL =
	process.env.WERBOT_E2E_DATABASE_URL ?? 'postgres://postgres:password@localhost:5433/werbot_e2e?sslmode=disable';

export default defineConfig({
	testDir: 'e2e',
	workers: 1,
	retries: 0,
	use: { baseURL: 'http://127.0.0.1:18080', trace: 'retain-on-failure' },
	projects: [
		{ name: 'identity', testMatch: 'identity.spec.ts', use: { ...devices['Desktop Chrome'] } },
		{ name: 'chromium', testIgnore: 'identity.spec.ts', dependencies: ['identity'], use: { ...devices['Desktop Chrome'] } }
	],
	webServer: [
		{
			command: '../bin/werbot server --with-gateway > e2e/.server.log 2>&1',
			url: 'http://127.0.0.1:18080/healthz',
			reuseExistingServer: false,
			timeout: 60_000,
			env: {
				WERBOT_ENV: 'development',
				WERBOT_GATEWAY_ADDR: '127.0.0.1:18443',
				WERBOT_GATEWAY_SSH_ADDR: '127.0.0.1:2222',
				WERBOT_HTTP_ADDR: '127.0.0.1:18080',
				WERBOT_PUBLIC_URL: 'http://127.0.0.1:18080',
				WERBOT_DATABASE_URL: dbURL,
				// the e2e database is recreated per run, so the embedded gateway needs a fresh identity
				WERBOT_DATA_DIR: mkdtempSync(join(tmpdir(), 'werbot-e2e-')),
				WERBOT_SECRET_KEY: 'AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=',
				HTTPS_PROXY: 'http://127.0.0.1:9', // feed jobs fail fast; loopback is never proxied
				WERBOT_LOG_FORMAT: 'json' // SMTP unset: LogMailer prints emails to the log
			}
		},
		{
			command: 'go run ./internal/e2e/sshtarget -addr 127.0.0.1:2022 -password e2e-secret',
			cwd: '..',
			port: 2022,
			reuseExistingServer: false,
			timeout: 120_000
		}
	]
});
