import { http } from "@/api";
import {
  ListServers_Request,
  Server_Request,
  AddServer_Request,
  UpdateServer_Request,
  DeleteServer_Request,
  UpdateServerActiveStatus_Request,
  ServerNameByID_Request,
} from "@proto/server";

enum URL {
  servers = "v1/servers",
}

const getServers = async (user_id: string, project_id: string, data?: ListServers_Request) =>
  http("GET", URL.servers, {
    params: {
      limit: data.limit,
      offset: data.offset,
      user_id: user_id,
      project_id: project_id,
    },
  });

const getServer = async (data: Server_Request) => http("GET", URL.servers, { params: data });

const postServer = async (data: AddServer_Request) => http("POST", URL.servers, { data: data });

const updateServer = async (data: UpdateServer_Request) =>
  http("PATCH", URL.servers, { data: data });

const deleteServer = async (data: DeleteServer_Request) =>
  http("DELETE", URL.servers, { params: data });

const updateServerStatus = async (data: UpdateServerActiveStatus_Request) =>
  http("PATCH", URL.servers + "/active", { data: data });

const serverNameByID = async (data: ServerNameByID_Request) =>
  http("GET", URL.servers + "/name", { params: data });

export {
  getServers,
  getServer,
  postServer,
  updateServer,
  deleteServer,
  updateServerStatus,
  serverNameByID,
};
