import { http } from "@/api";

enum URL {
  version = "v1/version",
}

const getVersion = async () => http("GET", URL.version);

export { getVersion };
