-- Ajoute (idempotent) le base_url Jupyter calé sur l'UUID de la VM aux configs cloud-init
-- 'system' Docker, pour servir JupyterLab derrière /api/jupyter-proxy/{uuid}/.
-- Sans base_url, Jupyter (servi à /) renvoie 404 sous le préfixe du proxy.
-- Idempotent : ne touche que les configs qui n'ont pas encore base_url.

-- 1) Définir VM_UUID + JBASE avant le lancement du conteneur jupyter.
UPDATE config_pools SET data = replace(
  data,
  $p$sudo docker rm -f jupyter 2>/dev/null || true$p$,
  $p$VM_UUID=$(curl -s http://169.254.169.254/openstack/latest/meta_data.json | tr ',' '\n' | grep -m1 '"uuid"' | sed -E 's/.*"uuid"[: ]+"([^"]+)".*/\1/')
JBASE="/api/jupyter-proxy/${VM_UUID}/"
sudo docker rm -f jupyter 2>/dev/null || true$p$
)
WHERE user_id = 'system' AND data LIKE '%docker run -d --restart=always --name jupyter%' AND data NOT LIKE '%base_url%';

-- 2) Passer JBASE en variable d'environnement au conteneur jupyter.
UPDATE config_pools SET data = replace(
  data,
  $p$  -p 8888:8888 \
  -v /home/vmuser/nbgrader:/home/jovyan/nbgrader \$p$,
  $p$  -p 8888:8888 \
  -e JBASE="$JBASE" \
  -v /home/vmuser/nbgrader:/home/jovyan/nbgrader \$p$
)
WHERE user_id = 'system' AND data LIKE '%docker run -d --restart=always --name jupyter%' AND data NOT LIKE '%-e JBASE=%';

-- 3) Ajouter les flags base_url / remote_access / xsrf à la commande jupyter lab.
UPDATE config_pools SET data = replace(
  data,
  $p$--ServerApp.allow_origin="*"'$p$,
  $p$--ServerApp.allow_origin="*" --ServerApp.allow_remote_access=True --ServerApp.disable_check_xsrf=True --ServerApp.base_url="$JBASE"'$p$
)
WHERE user_id = 'system' AND data LIKE '%docker run -d --restart=always --name jupyter%' AND data NOT LIKE '%base_url%';
