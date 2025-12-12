import { createClient } from "@connectrpc/connect";
import { createGrpcWebTransport } from "@connectrpc/connect-web";
import { create } from "@bufbuild/protobuf";

import {
    CreateConfigRequestSchema,
    UpdateConfigRequestSchema,
    DeleteConfigRequestSchema,
    GetConfigRequestSchema,
    ConfigService,
} from "../frontcontrol_pb"

import type {
    CreateConfigResponse,
    UpdateConfigResponse,
    DeleteConfigResponse,
    GetConfigResponse,
    CreateConfigRequest,
    UpdateConfigRequest,
    DeleteConfigRequest,
    GetConfigRequest,
} from "../frontcontrol_pb"

const transport = createGrpcWebTransport({
    baseUrl: "http://localhost:80",
    useBinaryFormat: true,
    interceptors: [],
    fetch: globalThis.fetch,
    jsonOptions: {},
});

const configClient = createClient(ConfigService, transport);

export async function createConfig(
    user: string, 
    key: string, 
    value: string,
): Promise<boolean> {
    const req : CreateConfigRequest = create(CreateConfigRequestSchema, {
        user, key, value});
    try {
        const res: CreateConfigResponse = await configClient.createConfig(req);
        return res.success;
    } catch (err) {
        console.error("Erreur creation config: ", err);
        throw err;
    }
}

export async function updateConfig(
    user: string, 
    key: string, 
    value: string,
): Promise<boolean> {
    const req : UpdateConfigRequest = create(UpdateConfigRequestSchema, {
        user, key, value});
    try {
        const res: UpdateConfigResponse = await configClient.updateConfig(req);
        return res.success;
    } catch (err) {
        console.error("Erreur update config: ", err);
        throw err;
    }
}

export async function deleteConfig(
    user: string, 
    key: string,
): Promise<boolean> {
    const req : DeleteConfigRequest= create(DeleteConfigRequestSchema, {
        user, key});
        try {
            const res: DeleteConfigResponse = 
                await configClient.deleteConfig(req);
        return res.success;
    } catch (err) {
        console.error("Erreur delete config: ", err);
        throw err;
    }
}

export async function getConfig(
    user: string, 
    key: string
): Promise<GetConfigResponse> {
    const req : GetConfigRequest = create(GetConfigRequestSchema, {user, key});
    try {
        const res: GetConfigResponse = await configClient.getConfig(req);
        return res
    } catch (err) {
        console.error("Error fetching config: ", err);
        throw err;
    }
}