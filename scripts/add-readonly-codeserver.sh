#!/bin/bash
# Ajoute (idempotent) une 2ᵉ instance code-server EN LECTURE SEULE sur le port 8444 aux
# configs cloud-init 'system' Docker qui ont déjà un conteneur 'codeserver' mais pas
# encore 'codeserver-ro'. Sert le mode « partage en lecture » entre élèves : le projet
# est monté ':ro' (verrou OS — ni l'éditeur ni le terminal ne peuvent écrire).
#
# Usage : POSTGRES_* lus depuis .env du repo.
set -euo pipefail
cd "$(dirname "$0")/.."

export PGPASSWORD=$(grep '^POSTGRES_PASSWORD=' .env | cut -d= -f2-)
PGU=$(grep '^POSTGRES_USER=' .env | cut -d= -f2-)
PGDB=$(grep '^POSTGRES_DB=' .env | cut -d= -f2-)
PGH=$(grep '^POSTGRES_HOST=' .env | cut -d= -f2-); PGH=${PGH:-localhost}
PSQL=(psql -h "$PGH" -U "$PGU" -d "$PGDB" -tA)

# Bloc à ajouter (dollar-quoté côté SQL pour gérer les apostrophes du -lc '...').
read -r -d '' RO_BLOCK <<'BLOCK' || true

# --- code-server LECTURE SEULE (8444) : ajouté par add-readonly-codeserver.sh ---
sudo docker rm -f codeserver-ro 2>/dev/null || true
sudo docker run -d --restart=always --name codeserver-ro \
  --network host --entrypoint /bin/bash \
  -v /home/vmuser:/home/coder/project:ro \
  registry.virtualdata.cloud.idcs.polytechnique.fr/docker-hub-proxy/codercom/code-server:latest \
  -lc 'mkdir -p ~/.local/share/code-server/User; printf "{\"files.readonlyInclude\":{\"**/*\":true}}" > ~/.local/share/code-server/User/settings.json; exec code-server --auth none --cert --bind-addr 0.0.0.0:8444 /home/coder/project'
BLOCK

# Cibles : configs system avec un conteneur codeserver mais sans codeserver-ro.
names=$("${PSQL[@]}" -c "SELECT name FROM config_pools WHERE user_id='system' AND data LIKE '%--name codeserver %' AND data NOT LIKE '%codeserver-ro%';")

if [ -z "$names" ]; then
  echo "Rien à patcher (toutes les configs system ont déjà codeserver-ro)."
  exit 0
fi

while IFS= read -r name; do
  [ -z "$name" ] && continue
  # Append en SQL via littéral dollar-quoté (insensible aux apostrophes du bloc).
  "${PSQL[@]}" -c "UPDATE config_pools SET data = data || \$ro\$${RO_BLOCK}\$ro\$ WHERE user_id='system' AND name='${name}';" >/dev/null
  echo "patché: $name"
done <<< "$names"

echo "Terminé."
