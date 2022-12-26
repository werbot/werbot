import { RefreshTokenRequest } from "@proto/auth";
import { SignIn_Request } from "@proto/user";

import { http } from "@/api";

enum URL {
  auth = "auth",
}

const postSignIn = async (data: SignIn_Request) =>
  http("POST", URL.auth + "/signin", { data: data });

const postLogout = async () => http("POST", URL.auth + "/logout");

const postRefresh = async (data: RefreshTokenRequest) =>
  http("POST", URL.auth + "/refresh", { data: data });

// reset password - step 1
const postSendEmail = async (email: string) =>
  http("POST", URL.auth + "/password_reset", { data: { email: email } });

// reset password - step 2
const postCheckResetToken = async (token: string) =>
  http("POST", URL.auth + "/password_reset/" + token);

// reset password - step 3
const postResetPassword = async (token: string, password: string) =>
  http("POST", URL.auth + "/password_reset/" + token, { data: { password: password } });

const getProfile = async () => http("GET", URL.auth + "/profile");

export {
  postLogout,
  postSignIn,
  postRefresh,
  getProfile,
  postSendEmail,
  postCheckResetToken,
  postResetPassword,
};
