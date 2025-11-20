import { createClient } from "@connectrpc/connect";
import { createGrpcWebTransport } from "@connectrpc/connect-web";
import {
  AuthService,
  CreateUserRequestSchema,
  CreateUserResponseSchema,
  AuthenticateUserRequestSchema,
  AuthenticateUserResponseSchema,
} from "../frontcontrol_pb";
import type {
  CreateUserRequest,
  CreateUserResponse,
  AuthenticateUserRequest,
  AuthenticateUserResponse,
} from "../frontcontrol_pb";
import { create } from "@bufbuild/protobuf"; // fonction utilitaire de protoc-gen-es


const transport = createGrpcWebTransport({
  baseUrl: "http://localhost:80", // l'URL de ton proxy gRPC-Web
  useBinaryFormat: true,             // recommandé pour gRPC-Web
  interceptors: [],                  // tu peux ajouter des middlewares si besoin
  fetch: globalThis.fetch,           // le fetch du navigateur
  jsonOptions: {},                   // options pour JSON, si tu veux
});

/**
 * Création d'un client Connect pour AuthService
 */
const authClient = createClient(AuthService, transport);


export async function createUser(username: string, email: string, password: string) {
  const req: CreateUserRequest = create(CreateUserRequestSchema, { username, email, password });

  try {
    // Appel RPC direct comme fonction
    const res: CreateUserResponse = await authClient.createUser(req);
    return res;
  } catch (err) {
    console.error("Erreur création utilisateur :", err);
    throw err;
  }
}

export async function authenticateUser(email: string, password: string) {
  const req: AuthenticateUserRequest = create(AuthenticateUserRequestSchema, { email, password });

  try {
    const res: AuthenticateUserResponse = await authClient.authenticateUser(req);
    return res;
  } catch (err) {
    console.error("Erreur authentification :", err);
    throw err;
  }
}


