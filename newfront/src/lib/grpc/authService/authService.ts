import { createClient } from "@connectrpc/connect";
import { createGrpcWebTransport } from "@connectrpc/connect-web";
import {
  AuthService,
  CreateUserRequestSchema,
  AuthenticateUserRequestSchema,
} from "../frontcontrol_pb";
import type {
  CreateUserRequest,
  CreateUserResponse,
  AuthenticateUserRequest,
  AuthenticateUserResponse,
} from "../frontcontrol_pb";
import { create } from "@bufbuild/protobuf";


const transport = createGrpcWebTransport({
  baseUrl: "http://localhost:80", //a modifier ! 
  // baseUrl: "/rpc/", // Version VM
  useBinaryFormat: true,
  interceptors: [],
  fetch: globalThis.fetch,
  jsonOptions: {},
});

/**
 * Création d'un client Connect pour AuthService
 */
const authClient = createClient(AuthService, transport);


export async function createUser(
  username: string, 
  email: string, 
  password: string,
) {
  const req: CreateUserRequest = create(CreateUserRequestSchema, {
     username, email, password });
  try {
    const res: CreateUserResponse = await authClient.createUser(req);
    return res;
  } catch (err) {
    console.error("Erreur création utilisateur :", err);
    throw err;
  }
}

export async function authenticateUser(
  email: string, 
  password: string,
) {
  const req: AuthenticateUserRequest = create(AuthenticateUserRequestSchema, {
     email, password });
  try {
    const res: AuthenticateUserResponse
      = await authClient.authenticateUser(req);
    return res;
  } catch (err) {
    console.error("Erreur authentification :", err);
    throw err;
  }
}


