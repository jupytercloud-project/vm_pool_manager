#!/usr/bin/env bash
# Bootstrap du Moodle local de dev : active les Web Services, crée un service + token,
# et crée des cours/élèves/inscriptions de démo. Idempotent.
# Reporte MOODLE_URL / MOODLE_TOKEN dans le .env racine.
#
#   moodle/docker compose up -d   (Moodle doit répondre sur :8081)
#   scripts/moodle-bootstrap.sh
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MOODLE_DIR="$ROOT/moodle"
MOODLE_URL="${MOODLE_URL:-http://localhost:8081}"

cd "$MOODLE_DIR"
echo "→ Vérification que Moodle répond sur $MOODLE_URL …"
if ! curl -s -o /dev/null --max-time 8 "$MOODLE_URL/login/index.php"; then
  echo "✗ Moodle ne répond pas. Lancer d'abord : (cd moodle && docker compose up -d) puis attendre l'install."
  exit 1
fi

echo "→ Exécution du bootstrap dans le conteneur (API Moodle)…"
docker compose cp bootstrap.php moodle:/tmp/cpm-bootstrap.php >/dev/null
OUT="$(docker compose exec -T moodle php /tmp/cpm-bootstrap.php)"
echo "$OUT"

# moodledata doit rester inscriptible par le process web (sinon les WS qui créent des
# dossiers temporaires échouent en invaliddatarootpermissions). À refaire APRÈS le
# bootstrap car purge_all_caches/création d'activités recrée des dossiers (dev only).
echo "→ Permissions moodledata (dev)…"
docker compose exec -T -u root moodle chmod -R a+rwX /bitnami/moodledata >/dev/null 2>&1 || true

if ! grep -q '^OK$' <<<"$OUT"; then
  echo "✗ Bootstrap incomplet (pas de ligne OK). Voir la sortie ci-dessus."
  exit 1
fi
TOKEN="$(grep '^TOKEN=' <<<"$OUT" | head -1 | cut -d= -f2)"
if [ -z "$TOKEN" ]; then echo "✗ Token introuvable dans la sortie."; exit 1; fi

# ── Reporter MOODLE_URL / MOODLE_TOKEN dans les .env (idempotent) ──
# Le control center charge control_center/.env en priorité (et ne lit ../.env que si absent),
# donc on écrit dans les DEUX : racine (commodité) + control_center/.env (réellement chargé).
upsert() { # fichier clé valeur
  local f="$1" k="$2" v="$3"
  touch "$f"
  if grep -qE "^${k}=" "$f"; then
    sed -i '' -E "s|^${k}=.*|${k}=${v}|" "$f" 2>/dev/null || sed -i -E "s|^${k}=.*|${k}=${v}|" "$f"
  else
    printf '\n%s=%s\n' "$k" "$v" >> "$f"
  fi
}
for ENV in "$ROOT/.env" "$ROOT/control_center/.env"; do
  upsert "$ENV" "MOODLE_URL" "$MOODLE_URL"
  upsert "$ENV" "MOODLE_TOKEN" "$TOKEN"
done

echo ""
echo "✓ Moodle prêt. MOODLE_URL et MOODLE_TOKEN écrits dans .env"
echo "  UI Moodle  : $MOODLE_URL  (admin / voir moodle/.env)"
echo "  Token (WS) : ${TOKEN:0:8}… (longueur ${#TOKEN})"
