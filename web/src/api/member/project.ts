import { http } from "@/api";
import {
  ListProjectMembers_Request,
  GetProjectMember_Request,
  CreateProjectMember_Request,
  UpdateProjectMember_Request,
  DeleteProjectMember_Request,
  UpdateProjectMemberStatus_Request,
  GetUsersWithoutProject_Request,
  ListProjectMembersInvite_Request,
  CreateProjectMemberInvite_Request,
  DeleteProjectMemberInvite_Request,
} from "@proto/member/member";

enum URL {
  project_members = "v1/members",
}

const getProjectMembers = async (
  user_id: string,
  project_id: string,
  data?: ListProjectMembers_Request
) =>
  http("GET", URL.project_members, {
    params: {
      limit: data.limit,
      offset: data.offset,
      owner_id: user_id,
      project_id: project_id,
    },
  });

const getProjectMember = async (data: GetProjectMember_Request) =>
  http("GET", URL.project_members, { params: data });

const postProjectMember = async (data: CreateProjectMember_Request) =>
  http("POST", URL.project_members, { data: data });

const updateProjectMember = async (data: UpdateProjectMember_Request) =>
  http("PATCH", URL.project_members, { data: data });

const deleteProjectMember = async (data: DeleteProjectMember_Request) =>
  http("DELETE", URL.project_members, { params: data });

const updateProjectMemberStatus = async (data: UpdateProjectMemberStatus_Request) =>
  http("PATCH", URL.project_members + "/active", { data: data });

const getUsersWithoutProject = async (data: GetUsersWithoutProject_Request) =>
  http("GET", URL.project_members + "/search", { params: data });

const getProjectMembersInvite = async (data: ListProjectMembersInvite_Request) =>
  http("GET", URL.project_members + "/invite", { params: data });

const postProjectMemberInvite = async (data: CreateProjectMemberInvite_Request) =>
  http("POST", URL.project_members + "/invite", { data: data });

const deleteProjectMemberInvite = async (data: DeleteProjectMemberInvite_Request) =>
  http("DELETE", URL.project_members + "/invite", { params: data });

export {
  getProjectMembers,
  getProjectMember,
  postProjectMember,
  updateProjectMember,
  deleteProjectMember,
  updateProjectMemberStatus,
  getUsersWithoutProject,
  getProjectMembersInvite,
  postProjectMemberInvite,
  deleteProjectMemberInvite,
};
