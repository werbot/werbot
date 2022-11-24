import { http } from "@/api";
import {
  GetServerMember_Request,
  ListServerMembers_Request,
  CreateServerMember_Request,
  UpdateServerMember_Request,
  DeleteServerMember_Request,
} from "@proto/member/member";

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

const getServerMember = async (data: GetServerMember_Request) =>
  http("GET", URL.server_members, { params: data });

const postServerMember = async (data: CreateServerMember_Request) =>
  http("POST", URL.server_members, { data: data });

const updateServerMember = async (data: UpdateServerMember_Request) =>
  http("PATCH", URL.server_members, { data: data });

const deleteServerMember = async (data: DeleteServerMember_Request) =>
  http("DELETE", URL.server_members, { params: data });

const updateServerMemberStatus = async (data: UpdateServerMemberStatus_Request) =>
  http("PATCH", URL.server_members + "/active", { data: data });

export {
  getServerMembers,
  getServerMember,
  postServerMember,
  updateServerMember,
  deleteServerMember,
  updateServerMemberStatus,
};
