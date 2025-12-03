// updateHandlers.ts
import { servers, serverPools, configs } from "$lib/store";
import {
    Type,
    Status,
} from "../grpc/frontcontrol_pb";
import type { UpdateDataUserResponse } from "../grpc/frontcontrol_pb";
import { get, type Writable } from "svelte/store";

// ======================================================================
// Type → Store mapping
// ======================================================================

const storeMap: Record<Type, Writable<any[]> | undefined> = {
    [Type.TYPE_UNKNOWN]: undefined,
    [Type.SERVERPOOL]: serverPools,
    [Type.SERVER]: servers,
    [Type.CONFIG]: configs,
};

// ======================================================================
// map<string,string> → object JS
// ======================================================================

function mapToObject(map: Record<string, string>) {
    return { ...map };
}

// ======================================================================
// Vérifie si un objet possède la clé composite (user_id + name)
// ======================================================================

function hasRequiredKey(obj: any) {
    const ok = obj && obj.user_id && obj.name;
    if (!ok) {
        console.warn("❌ Objet ignoré (clé composite absente) :", obj);
    }
    return ok;
}

// ======================================================================
// Match clé composite : user_id + name
// ======================================================================

function isSameKey(a: any, b: any) {
    return a.user_id === b.user_id && a.name === b.name;
}

// ======================================================================
// Mutation CREATE - UPDATE - DELETE dans le Store
// ======================================================================

function applyStoreMutation(store: Writable<any[]>, status: Status, obj: any) {
    if (!hasRequiredKey(obj)) return;

    store.update(items => {
        if (!Array.isArray(items)) items = [];

        const idx = items.findIndex(i => isSameKey(i, obj));

        switch (status) {
            case Status.CREATE:
                if (idx === -1) {
                    console.log("🟢 CREATE :", obj);
                    items.push(obj);
                }
                break;

            case Status.UPDATE:
                if (idx !== -1) {
                    console.log("🟡 UPDATE :", obj);
                    items[idx] = { ...items[idx], ...obj };
                }
                break;

            case Status.DELETE:
                if (idx !== -1) {
                    console.log("🔴 DELETE :", obj);
                    items.splice(idx, 1);
                }
                break;

            default:
                console.warn("❓ Status inconnu :", status);
        }

        return [...items]; // force réactivité Svelte
    });
}

// ======================================================================
// Handler principal
// ======================================================================

export function handleUserUpdate(update: UpdateDataUserResponse) {
    console.log("📩 Update reçu :", update);

    const store = storeMap[update.type];
    if (!store) {
        console.warn("⚠ Type non géré :", update.type);
        return;
    }

    const obj = mapToObject(update.data);
    applyStoreMutation(store, update.status, obj);

    console.log("📦 Store mis à jour :", get(store));
}
