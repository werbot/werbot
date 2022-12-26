// @generated by protobuf-ts 2.8.2 with parameter use_proto_field_name,ts_nocheck,long_type_string,force_optimize_code_size,force_client_none
// @generated from protobuf file "utility.proto" (package "utility", syntax proto3)
// tslint:disable
// @ts-nocheck
import { ServiceType } from "@protobuf-ts/runtime-rpc";
import { MessageType } from "@protobuf-ts/runtime";
/**
 * rpc GetInfo
 *
 * @generated from protobuf message utility.ListCountries
 */
export interface ListCountries {
}
/**
 * @generated from protobuf message utility.ListCountries.Request
 */
export interface ListCountries_Request {
    /**
     * @generated from protobuf field: string name = 1;
     */
    name: string;
}
/**
 * @generated from protobuf message utility.ListCountries.Response
 */
export interface ListCountries_Response {
    /**
     * @generated from protobuf field: repeated utility.ListCountries.Response.Country countries = 1;
     */
    countries: ListCountries_Response_Country[];
}
/**
 * @generated from protobuf message utility.ListCountries.Response.Country
 */
export interface ListCountries_Response_Country {
    /**
     * @generated from protobuf field: string code = 1;
     */
    code: string;
    /**
     * @generated from protobuf field: string name = 2;
     */
    name: string;
}
// @generated message type with reflection information, may provide speed optimized methods
class ListCountries$Type extends MessageType<ListCountries> {
    constructor() {
        super("utility.ListCountries", []);
    }
}
/**
 * @generated MessageType for protobuf message utility.ListCountries
 */
export const ListCountries = new ListCountries$Type();
// @generated message type with reflection information, may provide speed optimized methods
class ListCountries_Request$Type extends MessageType<ListCountries_Request> {
    constructor() {
        super("utility.ListCountries.Request", [
            { no: 1, name: "name", kind: "scalar", T: 9 /*ScalarType.STRING*/, options: { "validate.rules": { string: { minLen: "2" } } } }
        ]);
    }
}
/**
 * @generated MessageType for protobuf message utility.ListCountries.Request
 */
export const ListCountries_Request = new ListCountries_Request$Type();
// @generated message type with reflection information, may provide speed optimized methods
class ListCountries_Response$Type extends MessageType<ListCountries_Response> {
    constructor() {
        super("utility.ListCountries.Response", [
            { no: 1, name: "countries", kind: "message", repeat: 1 /*RepeatType.PACKED*/, T: () => ListCountries_Response_Country }
        ]);
    }
}
/**
 * @generated MessageType for protobuf message utility.ListCountries.Response
 */
export const ListCountries_Response = new ListCountries_Response$Type();
// @generated message type with reflection information, may provide speed optimized methods
class ListCountries_Response_Country$Type extends MessageType<ListCountries_Response_Country> {
    constructor() {
        super("utility.ListCountries.Response.Country", [
            { no: 1, name: "code", kind: "scalar", T: 9 /*ScalarType.STRING*/ },
            { no: 2, name: "name", kind: "scalar", T: 9 /*ScalarType.STRING*/ }
        ]);
    }
}
/**
 * @generated MessageType for protobuf message utility.ListCountries.Response.Country
 */
export const ListCountries_Response_Country = new ListCountries_Response_Country$Type();
/**
 * @generated ServiceType for protobuf service utility.UtilityHandlers
 */
export const UtilityHandlers = new ServiceType("utility.UtilityHandlers", [
    { name: "ListCountries", options: {}, I: ListCountries_Request, O: ListCountries_Response }
]);
