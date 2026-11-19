import { getContext, setContext } from 'svelte';

export const buildVersion: string = import.meta.env.VITE_WERBOT_VERSION ?? 'dev';

export interface ServerInfo {
	version: string;
	geoipAttribution: boolean;
	failed: boolean;
}

const serverInfoKey = Symbol('serverInfo');

/** root layout: one GetVersion call shared with every page below it */
export const provideServerInfo = (info: ServerInfo) => setContext(serverInfoKey, info);
export const serverInfo = () => getContext<ServerInfo>(serverInfoKey);
