# ---- BUILD CADDY WITH XCADDY ----
FROM caddy:builder AS builder

RUN xcaddy build \
    --with github.com/mholt/caddy-grpc-web

# ---- FINAL IMAGE ----
FROM caddy:latest

COPY --from=builder /usr/bin/caddy /usr/bin/caddy

# Ajoute ton Caddyfile
COPY Caddyfile /etc/caddy/Caddyfile

EXPOSE 80
EXPOSE 443

CMD ["caddy", "run", "--config", "/etc/caddy/Caddyfile", "--adapter", "caddyfile"]
