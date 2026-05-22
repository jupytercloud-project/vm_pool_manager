import { createGrpcWebTransport } from "@connectrpc/connect-web";
import type { Interceptor } from "@connectrpc/connect";
import { get } from "svelte/store";
import { authStore } from "$lib/store/authStore";

const authInterceptor: Interceptor = (next) => (req) => {
  const auth = get(authStore);
  if (auth?.accessToken) {
    req.header.set("authorization", `Bearer ${auth.accessToken}`);
  }
  return next(req);
};

export const authenticatedTransport = createGrpcWebTransport({
  baseUrl: "/rpc/",
  useBinaryFormat: true,
  interceptors: [authInterceptor],
  fetch: globalThis.fetch,
  jsonOptions: {},
});

export const publicTransport = createGrpcWebTransport({
  baseUrl: "/rpc/",
  useBinaryFormat: true,
  interceptors: [],
  fetch: globalThis.fetch,
  jsonOptions: {},
});
