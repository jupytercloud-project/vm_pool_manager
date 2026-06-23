-- Corrige les liens du formgrader nbgrader pour qu'ils respectent le base_url (proxy).
-- nbgrader (utils.js, fonction linkTo) code en dur "/notebooks/", "/edit/", "/tree/" à la
-- RACINE quand formgrader est ouvert en onglet → derrière /api/jupyter-proxy/{uuid}/ ça
-- tombe sur la racine du domaine = 404. On préfixe ces chemins par base_url (déjà injecté
-- dans la page). Le patch est appliqué dans le conteneur jupyter à chaque boot de la VM.
-- Idempotent : n'ajoute le bloc qu'aux configs system qui ne l'ont pas encore.
UPDATE config_pools SET data = data || $patch$

# --- Patch nbgrader formgrader : liens relatifs au base_url (proxy) ---
for i in $(seq 1 30); do sudo docker exec jupyter true 2>/dev/null && break; sleep 2; done
sudo docker exec -u root jupyter bash -lc 'f=$(find / -path "*formgrader/static/js/utils.js" 2>/dev/null | head -1); [ -n "$f" ] && sed -i "s#notebook: \"/notebooks/\"#notebook: base_url + \"/notebooks/\"#; s#file: \"/edit/\"#file: base_url + \"/edit/\"#; s#directory: \"/tree/\"#directory: base_url + \"/tree/\"#" "$f"' || true
$patch$
WHERE user_id='system' AND data LIKE '%--name jupyter%' AND data NOT LIKE '%formgrader/static/js/utils.js%';
