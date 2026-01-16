import { createClient } from "@connectrpc/connect";
import { createGrpcWebTransport } from "@connectrpc/connect-web";
import { create } from "@bufbuild/protobuf";
import {
    UserService,
    UpdateDataUserRequestSchema,
    type AddPersonalSSHKeyRequest,
    type AddPersonnalSSHKeyResponse,
} from "../frontcontrol_pb"
import { handleUserUpdate } from "$lib/utils/updateHandlers";

const transport = createGrpcWebTransport({
  baseUrl: "http://localhost:80", //a modifier !
  // baseUrl: "/rpc/", // Version VM
  useBinaryFormat: true,
  interceptors: [],
  fetch: globalThis.fetch,
  jsonOptions: {},
});

const userclient = createClient(UserService, transport);

export async function subscribeUserUpdate(user: string, signal?: AbortSignal) {
    const req = create(UpdateDataUserRequestSchema, {user});
    console.log("Envoi request stream :", req);
    const stream = userclient.updateDataUser(req, { signal });

    try {
        for await(const update of stream) { 
            handleUserUpdate(update);
        }
    } catch (err) {
        if ((err as any).name === 'AbortError') {
            console.log("Stream UserService arrêté");
        } else {
            console.error("Erreur stream UserService:", err);
        }
    }
}

export async function addSSHPersonalKey(req: AddPersonalSSHKeyRequest): Promise<AddPersonnalSSHKeyResponse> {
    try {
        const res: AddPersonnalSSHKeyResponse = await userclient.addPersonalSSHKey(req);
        return res;
    }
    catch (err) { 
        console.error("Erreur ajout clé SSH perso :", err);
        throw err;
    }
}

