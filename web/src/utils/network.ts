type AddressType = "IPv4" | "IPv6" | "Hostname" | "Error";

import * as ipaddr from "ipaddr.js";

export function getAddressType(addr: string): AddressType {
  //const hostnameRegex = "(?=.{1,255}$)[0-9A-Za-z](?:(?:[0-9A-Za-z]|\b-){0,61}[0-9A-Za-z])?(?:\.[0-9A-Za-z](?:(?:[0-9A-Za-z]|\b-){0,61}[0-9A-Za-z])?)*\.?"

  try {
    const parse_addr = ipaddr.parse(addr);
    const kind = parse_addr.kind();

    if (kind === "ipv4") {
      return "IPv4";
    } else if (kind === "ipv6") {
      return "IPv6";
    } else {
      throw new Error("unexpected return value");
    }
  } catch (err) {
    //return "Error";
    return "Hostname";
  }
}
