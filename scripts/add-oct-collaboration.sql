-- Câble le code-server des VMs étudiantes au serveur de collaboration temps réel (OCT)
-- hébergé sur la VM infra dédiée colabVscodeInfra (157.136.249.81:8100).
-- Ajoute l'extension Open Collaboration Tools + fixe oct.serverUrl dans les settings
-- code-server, pour que les commandes « Open Collaboration: Share/Join » co-éditent en
-- temps réel (curseurs, CRDT) en passant par la VM centrale, pas par les VMs étudiantes.
-- Idempotent : ne touche que les configs system qui n'ont pas encore l'extension.
-- NB : si l'IP de colabVscodeInfra change, mettre à jour oct.serverUrl ici + sur les VMs.
UPDATE config_pools SET data = replace(
  data,
  $old$code-server --install-extension ms-python.python --install-extension ms-toolsai.jupyter || true; exec code-server --auth none --cert --bind-addr 0.0.0.0:8443 /home/coder/project$old$,
  $new$mkdir -p ~/.local/share/code-server/User; printf "{\"oct.serverUrl\":\"http://157.136.249.81:8100/\",\"oct.alwaysAskToOverrideServerUrl\":false}" > ~/.local/share/code-server/User/settings.json; code-server --install-extension ms-python.python --install-extension ms-toolsai.jupyter --install-extension typefox.open-collaboration-tools || true; exec code-server --auth none --cert --bind-addr 0.0.0.0:8443 /home/coder/project$new$
)
WHERE user_id = 'system' AND data LIKE '%--name codeserver %' AND data NOT LIKE '%open-collaboration%';
