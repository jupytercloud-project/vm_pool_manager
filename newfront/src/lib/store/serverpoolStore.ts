import { writable } from "svelte/store";

import {
    getAllImages,
    getAllFlavors,
    getAllNetworks,
    getAllServers,
    getAllServerPools,
    getAllConfigs,
    existServer,
    existServerPools,
    existConfigs,
} from "$lib/index";

import type {
    Image,
    Flavor,
    Network,
    Server,
    ServerPool,
    Config,
} from "../grpc/frontcontrol_pb";


// ==========================================================================
// Stores
// ==========================================================================
export const images = writable<Image[]>([]);
export const flavors = writable<Flavor[]>([]);
export const networks = writable<Network[]>([]);
export const servers = writable<Server[]>([]);
export const serverPools = writable<ServerPool[]>([]);
export const configs = writable<Config[]>([]);


// ==========================================================================
// Loaders (chargent les données et mettent à jour les stores)
// ==========================================================================

export async function loadImages(user: string) {
    console.log("getAllImages start")
    const data = await getAllImages(user);
    images.set(data);
    console.log("getAllImages end")
}

export async function loadFlavors(user: string) {
    console.log("getAllFlavors start")
    const data = await getAllFlavors(user);
    flavors.set(data);
    console.log("getAllFlavors end")
}

export async function loadNetworks(user: string) {
    console.log("getAllNetworks start")
    const data = await getAllNetworks(user);
    networks.set(data);
    console.log("getAllNetworks end")
}

export async function loadServers(user: string) {
    console.log("getAllServers start")
    const exist = await existServer(user);
    if (!exist) {
        console.log("no server for user ", user);
        servers.set([]);
    }
    else {
        const data = await getAllServers(user);
        servers.set(data);
    }
    console.log("getAllServers end")
}

export async function loadServerPools(user: string) {
    console.log("getAllServerPools start")
    const exist = await existServerPools(user);
    if (!exist) {
        console.log("no serverpool for user ", user);
        servers.set([]);
    }
    else {
        const data = await getAllServerPools(user);
        serverPools.set(data);
    }
    console.log("getAllServerPools end")
}

export async function loadConfigs(user: string) {
    console.log("getAllConfigs start")
    const exist = await existConfigs(user);
    if (!exist) {
        console.log("no config for user ", user);
        configs.set([]);
    }
    else {
        const data = await getAllConfigs(user);
        configs.set(data);
    }
    console.log("getAllConfigs end")
}


// ==========================================================================
// Helper pour tout charger d'un coup (infrastructure générale)
// ==========================================================================
export async function loadAll(user: string) {
    await loadImages(user);
    await loadFlavors(user);
    await loadNetworks(user);
    await loadServers(user);
    await loadServerPools(user);
    await loadConfigs(user);
}

export function resetAll() {
    images.set([]);
    flavors.set([]);
    networks.set([]);
    servers.set([]);
    serverPools.set([]);
    configs.set([]);
}