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

const empty = create(EmptySchema, {});

const transport = createGrpcWebTransport({
  baseUrl: "http://localhost:80", // l'URL de ton proxy gRPC-Web
  useBinaryFormat: true,             // recommandé pour gRPC-Web
  interceptors: [],                  // tu peux ajouter des middlewares si besoin
  fetch: globalThis.fetch,           // le fetch du navigateur
  jsonOptions: {},                   // options pour JSON, si tu veux
});

const gatherClient = createClient(GatherDataService, transport);

export async function getAllImages(): Promise<Image[]>{
    const results: Image[] = [];
    const stream = gatherClient.getAllImages(empty);
    for await (const img of stream) {
        results.push(img);
    }
    return results;
}

export async function getAllFlavors(): Promise<Flavor[]> {
    const results: Flavor[] = [];
    const stream = gatherClient.getAllFlavors(empty);
    for await (const flav of stream) {
        results.push(flav);
    }
    return results;
}

export async function getAllNetworks(): Promise<Network[]> {
    const results: Network[] = [];
    const stream = gatherClient.getAllNetworks(empty);
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