// src/lib/poolClient.ts
import { createClient } from "@connectrpc/connect";
import { createGrpcWebTransport } from "@connectrpc/connect-web";
import { create } from "@bufbuild/protobuf";

import {
  PoolWithKeyRequestSchema,
  AttribVMinPoolRequestSchema,
  AttribVMService,
} from "../frontcontrol_pb";

import type {
  PoolWithKeyRequest,
  PoolWithKeyResponse,
  AttribVMinPoolRequest,
  AttribVMinPoolResponse,
} from "../frontcontrol_pb";


const transport = createGrpcWebTransport({
  baseUrl: "http://localhost:80", //a modifier !
  // baseUrl: "/rpc/", // Version VM
  useBinaryFormat: true,
  interceptors: [],
  fetch: globalThis.fetch,
  jsonOptions: {},
});

const attribClient = createClient(AttribVMService, transport);

/**
 * Recherche les pools disponibles pour une clé SSH donnée
 * @param pubkey Clé SSH publique
 * @returns Liste des pools { pool_id, user_id }
 */
export async function returnPoolsWithKey(
  pubkey: string
): Promise<{ pool_id: string; user_id: string }[]> {
  const req: PoolWithKeyRequest = create(PoolWithKeyRequestSchema, { pubkey });
  const pools: { pool_id: string; user_id: string }[] = [];
  console.log("Recherche des pools pour la clé: ", pubkey);

  try {
    const stream = attribClient.returnPoolWithKey(req);
    for await (const pool of stream) {
      pools.push({ pool_id: pool.poolId, user_id: pool.userId });
    }
    return pools;
  } catch (err) {
    console.error("Erreur récupération pools: ", err);
    throw err;
  }
}

/**
 * Attribue une VM dans le pool choisi pour la clé SSH
 * @param serverpool_id ID du pool
 * @param user_id ID de l'utilisateur
 * @param pubkey Clé SSH publique
 * @returns IP de la VM attribuée
 */
export async function attribVMinPool(
  serverpool_id: string,
  user_id: string,
  pubkey: string
): Promise<string> {
  const req: AttribVMinPoolRequest = create(AttribVMinPoolRequestSchema, {
    serverpoolId: serverpool_id,
    userId: user_id,
    pubkey: pubkey,
  });

  try {
    const res: AttribVMinPoolResponse = await attribClient.attribVMinPool(req);
    if (!res.success) {
      throw new Error("Aucune VM disponible ou erreur backend");
    }
    return res.addressedIp;
  } catch (err) {
    console.error("Erreur attribution VM: ", err);
    throw err;
  }
}
