import { http } from "@/api";
import {
  ListMembers_Request,
  GetMember_Request,
  CreateMember_Request,
  UpdateMember_Request,
  DeleteMember_Request,
  UpdateMemberActiveStatus_Request,
  GetUsersWithoutProject_Request,
} from "@proto/member/member";

enum URL {
  members = "v1/members",
}

const getMembers = async (user_id: string, project_id: string, data?: ListMembers_Request) =>
  http("GET", URL.members, {
    params: {
      limit: data.limit,
      offset: data.offset,
      owner_id: user_id,
      project_id: project_id,
    },
  });

const getMember = async (data: GetMember_Request) => http("GET", URL.members, { params: data });

const postMember = async (data: CreateMember_Request) => http("POST", URL.members, { data: data });

const updateMember = async (data: UpdateMember_Request) =>
  http("PATCH", URL.members, { data: data });

const deleteMember = async (data: DeleteMember_Request) =>
  http("DELETE", URL.members, { params: data });

const updateMemberStatus = async (data: UpdateMemberActiveStatus_Request) =>
  http("PATCH", URL.members + "/active", { data: data });

const getUsersWithoutProject = async (data: GetUsersWithoutProject_Request) =>
  http("GET", URL.members + "/search", { params: data });

export {
  getMembers,
  getMember,
  postMember,
  updateMember,
  deleteMember,
  updateMemberStatus,
  getUsersWithoutProject,
};
