import { createClient } from "@connectrpc/connect";
import { create } from "@bufbuild/protobuf";
import { authenticatedTransport } from "../transport";
import {
    UserService,
    UpdateDataUserRequestSchema,
    type AddPersonalSSHKeyRequest,
    type AddPersonnalSSHKeyResponse,
} from "../frontcontrol_pb"
import { handleUserUpdate } from "$lib/utils/updateHandlers";

const userclient = createClient(UserService, authenticatedTransport);

export async function subscribeUserUpdate(user: string, signal?: AbortSignal) {
    const req = create(UpdateDataUserRequestSchema, {user});
    const stream = userclient.updateDataUser(req, { signal });

    try {
        for await(const update of stream) { 
            handleUserUpdate(update);
        }
    } catch (err) {
        if ((err as any).name === 'AbortError') {
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

