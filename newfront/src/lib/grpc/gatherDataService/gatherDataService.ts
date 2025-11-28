import { createClient } from "@connectrpc/connect";
import { createGrpcWebTransport } from "@connectrpc/connect-web";
import { EmptySchema } from "@bufbuild/protobuf/wkt";
import { create } from "@bufbuild/protobuf";
import {
    GatherDataService,
    UserRequestSchema,
} from "../frontcontrol_pb"
import type {
    Image,
    Flavor,
    Network,
    Server,
    ServerPool,
    Config,
} from "../frontcontrol_pb"

const transport = createGrpcWebTransport({
  baseUrl: "http://localhost:80", // l'URL de ton proxy gRPC-Web
  useBinaryFormat: true,             // recommandé pour gRPC-Web
  interceptors: [],                  // tu peux ajouter des middlewares si besoin
  fetch: globalThis.fetch,           // le fetch du navigateur
  jsonOptions: {},                   // options pour JSON, si tu veux
});

const gatherClient = createClient(GatherDataService, transport);

export async function getAllImages(user: string): Promise<Image[]>{
    const results: Image[] = [];
    const req = create(UserRequestSchema, { user });
    const stream = gatherClient.getAllImages(req);
    for await (const img of stream) {
        results.push(img);
    }
    return results;
}

export async function getAllFlavors(user: string): Promise<Flavor[]> {
    const results: Flavor[] = [];
    const req = create(UserRequestSchema, { user });
    const stream = gatherClient.getAllFlavors(req);
    for await (const flav of stream) {
        results.push(flav);
    }
    return results;
}

export async function getAllNetworks(user: string): Promise<Network[]> {
    const results: Network[] = [];
    const req = create(UserRequestSchema, { user });
    const stream = gatherClient.getAllNetworks(req);
    for await (const net of stream) {
        results.push(net);
    }
    return results;
}

export async function getAllServers(user: string): Promise<Server[]> {
    const results: Server[] = [];
    const req = create(UserRequestSchema, { user });
    const stream = gatherClient.getAllServers(req);
    for await (const srv of stream) {
        results.push(srv);
    }
    return results;
}

export async function getAllServerPools(user : string): Promise<ServerPool[]> {
    const results: ServerPool[] = [];
    const req = create(UserRequestSchema, { user });
    const stream = gatherClient.getAllServerPools(req);
    for await (const pool of stream) {
        results.push(pool);
    }
    return results;
}

export async function getAllConfigs(user: string): Promise<Config[]> {
    const results: Config[] = [];
    const req = create(UserRequestSchema, {user});
    const stream = gatherClient.getAllConfigs(req);
    for await (const conf of stream) {
        results.push(conf);
    }
    return results;
}