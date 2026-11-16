import { createClient, type Client, type Transport } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { SystemService } from '#lib/gen/werbot/v1/system_pb.js';
import { AuthService } from '#lib/gen/werbot/v1/auth_pb.js';
import { ProfileService } from '#lib/gen/werbot/v1/profile_pb.js';
import { SSHKeyService } from '#lib/gen/werbot/v1/ssh_key_pb.js';
import { MemberService } from '#lib/gen/werbot/v1/member_pb.js';
import { TeamService } from '#lib/gen/werbot/v1/team_pb.js';
import { ResourceService } from '#lib/gen/werbot/v1/resource_pb.js';
import { AccessService } from '#lib/gen/werbot/v1/access_pb.js';
import { PolicyService } from '#lib/gen/werbot/v1/policy_pb.js';
import { GatewayService } from '#lib/gen/werbot/v1/gateway_pb.js';

// Same origin in all-in-one mode and behind `werbot web`.
// Binary format keeps google.protobuf.Any connector configs opaque, so an
// unknown resource type still decodes without a type registry.
export const transport: Transport = createConnectTransport({ baseUrl: '/', useBinaryFormat: true });

export const system: Client<typeof SystemService> = createClient(SystemService, transport);
export const auth: Client<typeof AuthService> = createClient(AuthService, transport);
export const profile: Client<typeof ProfileService> = createClient(ProfileService, transport);
export const sshKeys: Client<typeof SSHKeyService> = createClient(SSHKeyService, transport);
export const members: Client<typeof MemberService> = createClient(MemberService, transport);
export const teams: Client<typeof TeamService> = createClient(TeamService, transport);
export const resources: Client<typeof ResourceService> = createClient(ResourceService, transport);
export const access: Client<typeof AccessService> = createClient(AccessService, transport);
export const policy: Client<typeof PolicyService> = createClient(PolicyService, transport);
export const gateways: Client<typeof GatewayService> = createClient(GatewayService, transport);
