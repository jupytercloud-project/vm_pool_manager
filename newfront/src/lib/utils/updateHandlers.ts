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

let storeMap: Record<Type, Writable<any[]> | undefined>;
function getStoreMap() {
    if (!storeMap) {
        storeMap = {
            [Type.TYPE_UNKNOWN]: undefined,
            [Type.SERVERPOOL]: serverPools,
            [Type.SERVER]: servers,
            [Type.CONFIG]: configs,
        };
    }
    return storeMap;
}

// ======================================================================
// map<string,string> → object JS
// ======================================================================

function mapToObject(map: Record<string, string>) {
    return { ...map };
}

// ======================================================================
// Vérifie si un objet possède la clé composite (user_id + name)
// ======================================================================

function getUserKey(obj: any): string | undefined {
    return obj?.user_id ?? obj?.userId ?? obj?.userid ?? obj?.userID;
}


function hasRequiredKey(obj: any) {
    const userKey = getUserKey(obj);
    const ok = obj && userKey && obj.name;
    if (!ok) {
        console.warn("❌ Objet ignoré (clé composite absente) :", obj);
    }
    return ok;
}

// ======================================================================
// Match clé composite : user_id + name
// ======================================================================

function isSameKey(a: any, b: any) {
    const userA = getUserKey(a);
    const userB = getUserKey(b);

    console.log("Comparing keys:", 
        { user_a: userA, name_a: a?.name },
        { user_b: userB, name_b: b?.name }
    );

    return userA === userB && a?.name === b?.name;
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
        return [...items];
    });
}

function normalizeKeys(obj: any, type: Type) {
    if (!obj) return obj;
    const normalized = { ...obj };
    for (const key of ["user_id", "userId", "userid", "userID"]) {
        if (normalized[key] !== undefined) {
            normalized.user_id = normalized[key];
            if (key !== "user_id") delete normalized[key];
        }
    }
    if (normalized.image_ref !== undefined) {
        normalized.image = normalized.image_ref;
        delete normalized.image_ref;
    }
    if (normalized.flavor_ref !== undefined) {
        normalized.flavor = normalized.flavor_ref;
        delete normalized.flavor_ref;
    }
    if (type === Type.SERVER) {
        if (normalized.networks !== undefined) {
            try {
                const arr = JSON.parse(normalized.networks);
                if (Array.isArray(arr) && arr.length > 0) {
                    const entry = arr[0];
                    if (typeof entry === "string") {
                        const [net, ip] = entry.split(":");               
                        if (net) normalized.network = net;
                        if (ip) normalized.ipAddress = ip;
                    }
                }
            } catch (e) {
                console.warn("❌ Erreur networks:", normalized.networks, e);
            }
            delete normalized.networks;
        }
    }
    if (type === Type.SERVERPOOL) {
        if (normalized.networks !== undefined) {
            try {
                let raw = normalized.networks;
                if (typeof raw === "string") {
                    const arr = JSON.parse(raw);
                    if (Array.isArray(arr) && arr.length > 0) {
                        raw = arr[0];
                    }
                }
                if (typeof raw === "string") {
                    if (raw.includes(":")) {
                        raw = raw.split(":")[0];
                    }
                    normalized.network = raw;
                }
            } catch (e) {
                console.warn("❌ Erreur networks:", normalized.networks, e);
                normalized.network = normalized.networks;
            }
            delete normalized.networks;
        }
    }

    for (const key of ["serverpool_id"]) {
        if (!normalized.name && normalized[key] !== undefined) {
            normalized.name = normalized[key];
        }
        if (key !== "name") delete normalized[key];
    }

    return normalized;
}


// ======================================================================
// Handler principal
// ======================================================================

export function handleUserUpdate(update: UpdateDataUserResponse) {
    const storeMap = getStoreMap();
    console.log("📩 Update reçu :", update);

    const store = storeMap[update.type];
    if (!store) {
        console.warn("⚠ Type non géré :", update.type);
        return;
    }

    const obj = normalizeKeys(mapToObject(update.data), update.type);
    applyStoreMutation(store, update.status, obj);

    console.log("📦 Store mis à jour :", get(store));
}
