/** Fills {host} and {port} in a preset launch hint; the result is shown as text only. */
export const launchHint = (hint: string, host: string, port: number) => hint.replaceAll('{host}', host).replaceAll('{port}', String(port));

/** The first port of a tcp or udp resource config JSON; 0 when it has none. */
export function firstPort(configJson: string): number {
	try {
		return Number(JSON.parse(configJson)?.ports?.[0]?.port) || 0;
	} catch {
		return 0;
	}
}
