import { http } from "@/api";
import {
  ListUsers_Request,
  User_Request,
  AddUser_Request,
  UpdateUser_Request,
  UpdatePassword_Request,
  DeleteUser_Request,
} from "@proto/user";

enum URL {
  users = "v1/users",
}

const getUsers = async (data?: ListUsers_Request, user_id?: string) =>
  http("GET", URL.users, {
    params: {
      limit: data.limit,
      offset: data.offset,
      user_id: user_id,
    },
  });

const getUser = async (data: User_Request) =>
  http("GET", URL.users, {params: data});

const postUser = async (data: AddUser_Request) => http("POST", URL.users, { data: data });

const updateUser = async (data: UpdateUser_Request) =>
  http("PATCH", URL.users, {data: data});

const updatePassword = async (data: UpdatePassword_Request) =>
  http("PATCH", URL.users + "/password", {data: data});

const deleteUserStep1 = async (data: DeleteUser_Request) =>
  http("DELETE", URL.users, {
    data: {
      user_id: data.user_id,
      password: data.request["password"],
    },
  });

const deleteUserStep2 = async (data: DeleteUser_Request) =>
  http("DELETE", URL.users, {
    data: {
      user_id: data.user_id,
      token: data.request["token"],
    },
  });

export {
  getUsers,
  getUser,
  postUser,
  updateUser,
  updatePassword,
  deleteUserStep1,
  deleteUserStep2,
};
