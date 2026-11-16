import { createClient, type Client, type Transport } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { SystemService } from '#lib/gen/werbot/v1/system_pb.js';

// Same origin in all-in-one mode and behind `werbot web`.
export const transport: Transport = createConnectTransport({ baseUrl: '/' });

export const system: Client<typeof SystemService> = createClient(SystemService, transport);
