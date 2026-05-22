import { createClient } from "@connectrpc/connect";
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
    ExistData,
    UserRequest,
} from "../frontcontrol_pb"

import { authenticatedTransport } from "../transport";

const gatherClient = createClient(GatherDataService, authenticatedTransport);

export async function getAllImages(user: string): Promise<Image[]>{
    const results: Image[] = [];
    const req = create(UserRequestSchema, { user });
    const stream = gatherClient.getAllImages(req);
    for await (const img of stream) {
        if (!img.name) continue;
        results.push(img);
    }
    return results;
}

export async function getAllFlavors(user: string): Promise<Flavor[]> {
    const results: Flavor[] = [];
    const req = create(UserRequestSchema, { user });
    const stream = gatherClient.getAllFlavors(req);
    for await (const flav of stream) {
        if (!flav.name) continue;
        results.push(flav);
    }
    return results;
}

export async function getAllNetworks(user: string): Promise<Network[]> {
    const results: Network[] = [];
    const req = create(UserRequestSchema, { user });
    const stream = gatherClient.getAllNetworks(req);
    for await (const net of stream) {
        if (!net.name) continue;
        results.push(net);
    }
    return results;
}

export async function getAllServers(user: string): Promise<Server[]> {
    const results: Server[] = [];
    const req = create(UserRequestSchema, { user });
    const stream = gatherClient.getAllServers(req);
    for await (const srv of stream) {
        if (!srv.name) continue;
        results.push(srv);
    }
    return results;
}

export async function getAllServerPools(user : string): Promise<ServerPool[]> {
    const results: ServerPool[] = [];
    const req = create(UserRequestSchema, { user });
    const stream = gatherClient.getAllServerPools(req);
    for await (const pool of stream) {
        if (!pool.name) continue;
        results.push(pool);
    }
    return results;
}

export async function getAllConfigs(user: string): Promise<Config[]> {
    const results: Config[] = [];
    const req = create(UserRequestSchema, {user});
    const stream = gatherClient.getAllConfigs(req);
    for await (const conf of stream) {
        if (!conf.name) continue;
        results.push(conf);
    }
    return results;
}

export async function existServer(user: string): Promise<boolean> {
    const req : UserRequest = create(UserRequestSchema, {user});
    try {
        const res: ExistData = await gatherClient.existServer(req);
        return res.exist
    } catch (err) {
        console.error("Erreur existServer: ", err)
        throw err;
    }
}

export async function existServerPools(user: string): Promise<boolean> {
    const req : UserRequest = create(UserRequestSchema, {user});
    try {
        const res: ExistData = await gatherClient.existServerPools(req);
        return res.exist
    } catch (err) {
        console.error("Erreur existServerPools: ", err)
        throw err;
    }
}

export async function existConfigs(user: string): Promise<boolean> {
    const req : UserRequest = create(UserRequestSchema, {user});
    try {
        const res: ExistData = await gatherClient.existConfigs(req);
        return res.exist
    } catch (err) {
        console.error("Erreur existConfigs: ", err)
        throw err;
    }
}