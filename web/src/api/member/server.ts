import { http } from "@/api";
import {
  ServerMember_Request,
  ListServerMembers_Request,
  AddServerMember_Request,
  UpdateServerMember_Request,
  DeleteServerMember_Request,
  UpdateServerMemberStatus_Request,
  MembersWithoutServer_Request,
} from "@proto/member";

enum URL {
  server_members = "v1/members/server",
}

const getServerMembers = async (
  user_id: string,
  project_id: string,
  server_id: string,
  data?: ListServerMembers_Request
) =>
  http("GET", URL.server_members, {
    params: {
      limit: data.limit,
      offset: data.offset,
      owner_id: user_id,
      project_id: project_id,
      server_id: server_id,
    },
  });

const getServerMember = async (data: ServerMember_Request) =>
  http("GET", URL.server_members, { params: data });

const postServerMember = async (data: AddServerMember_Request) =>
  http("POST", URL.server_members, { data: data });

const updateServerMember = async (data: UpdateServerMember_Request) =>
  http("PATCH", URL.server_members, { data: data });

const deleteServerMember = async (data: DeleteServerMember_Request) =>
  http("DELETE", URL.server_members, { params: data });

const updateServerMemberStatus = async (data: UpdateServerMemberStatus_Request) =>
  http("PATCH", URL.server_members + "/active", { data: data });

  const getMembersWithoutServer = async (data: MembersWithoutServer_Request) =>
  http("GET", URL.server_members + "/search", { params: data });

export {
  getServerMembers,
  getServerMember,
  postServerMember,
  updateServerMember,
  deleteServerMember,
  updateServerMemberStatus,
  getMembersWithoutServer,
};
