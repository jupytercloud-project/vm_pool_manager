#!/bin/bash
# Creates an OpenStack snapshot for each Jupyter environment.
# Each snapshot has a custom Docker image pre-built that wraps the scientific
# base image and adds nbgrader (the assignment submission tool).
#
# Usage: ./scripts/make-jupyter-snapshots.sh [env_name]
#   env_name: optional, run only one env (e.g. "scipy")
#
# Requirements: openstack CLI, ssh access with SSH_PRIVATE_KEY_PATH

set -uo pipefail

OS_CLOUD="${OS_CLOUD:-ipp-idcs-vmpool}"
BASE_IMAGE="ubuntu-2204-docker"
FLAVOR="vd.24"
NETWORK="public-2"
KEYPAIR="maelan-mac"
SSH_KEY="${SSH_PRIVATE_KEY_PATH:-$HOME/.ssh/id_ed25519}"
SNAPSHOT_PREFIX="jupyter-snapshot"
POSTGRES_DSN="${POSTGRES_DSN:-postgres://admin:P00lManager_Secure_2026@localhost:5432/control_center?sslmode=disable}"

# Local Docker tag used on every snapshot VM for the nbgrader-enriched image.
# It is always the same so the startup script is universal.
NBGRADER_LOCAL_TAG="jupyter-nbgrader:latest"

# Each entry: "snapshot-suffix|docker-image|display-label"
ENVS=(
  "scipy|registry.virtualdata.cloud.idcs.polytechnique.fr/docker-hub-proxy/jupyter/scipy-notebook:latest|Python scientifique (scipy-notebook)"
  "scipy-plus|registry.virtualdata.cloud.idcs.polytechnique.fr/plmlab-hub-proxy/docker-images/scipy-notebook-plus:2023.01.24|Python scientifique+"
  "datascience|registry.virtualdata.cloud.idcs.polytechnique.fr/docker-hub-proxy/jupyter/datascience-notebook:2343e33dec46|Data Science (Python + R + Julia)"
  "julia|registry.virtualdata.cloud.idcs.polytechnique.fr/plmlab-hub-proxy/docker-images/julia:0.0.4|Julia"
  "bio583|registry.virtualdata.cloud.idcs.polytechnique.fr/plmlab-hub-proxy/ip-paris/idcs/docker/bio583:0.0.1|BIO583"
  "eco589|registry.virtualdata.cloud.idcs.polytechnique.fr/gitlab-in2p3-proxy/energy4climate/public/education/eco-589-tutorials:0.2|ECO589"
  "compeco|albop/computational_economics:latest|Computational Economics"
  "mec431|registry.virtualdata.cloud.idcs.polytechnique.fr/gitlab-hub-proxy/bleyerj/x_mec431:040520231145|MEC431"
  "mec558|registry.virtualdata.cloud.idcs.polytechnique.fr/gitlab-in2p3-proxy/ipsl/lmd/intro/jupyterlabimages:07-11-2023|MEC558"
  "map579|registry.virtualdata.cloud.idcs.polytechnique.fr/plmlab-hub-proxy/docker-images/xeus-cling:0.0.5|MAP579"
  "mec552a|registry.virtualdata.cloud.idcs.polytechnique.fr/gitlab-inria-proxy/mgenet/mec552a-repo2docker:latest|MEC552A"
  "mec552b|registry.virtualdata.cloud.idcs.polytechnique.fr/jupyter/mec552b-repo2docker:1d894fa3|MEC552B"
  "mec568|registry.virtualdata.cloud.idcs.polytechnique.fr/gitlab-inria-proxy/mgenet/mec568-repo2docker:latest|MEC568"
  "mec581|registry.virtualdata.cloud.idcs.polytechnique.fr/gitlab-inria-proxy/mgenet/mec-581-repo-2-docker:9b9d98b7|MEC581"
  "mec666|registry.virtualdata.cloud.idcs.polytechnique.fr/gitlab-in2p3-proxy/energy4climate/public/education/climate_change_and_energy_transition:0.2|MEC666"
)

FILTER="${1:-}"

log() { echo "[$(date +%H:%M:%S)] $*"; }
die() { echo "ERROR: $*" >&2; exit 1; }

wait_ssh() {
  local ip=$1
  log "Waiting for SSH on $ip..."
  for i in $(seq 1 60); do
    if ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 -o BatchMode=yes \
        -i "$SSH_KEY" "vmuser@$ip" "true" 2>/dev/null; then
      return 0
    fi
    sleep 5
  done
  die "SSH never became available on $ip"
}

# Write the generic startup script to PostgreSQL.
# All envs use the same NBGRADER_LOCAL_TAG image, built at snapshot time.
upsert_config() {
  local suffix=$1
  local config_name="jupyter-snapshot-${suffix}"
  # Use a temp var to avoid heredoc quoting issues with dollar signs in the SQL
  local nbgrader_tag="${NBGRADER_LOCAL_TAG}"
  local script
  script=$(cat <<SCRIPT
#!/bin/bash
# Start Jupyter (nbgrader-enriched image pre-built in snapshot).
# repo2docker course images have no start-notebook.sh, so we run \`jupyter lab\`
# directly (also works on jupyter/docker-stacks images), and upgrade \`packaging\`
# at boot (nbgrader can leave a version too old for JupyterLab 4 -> ImportError).
until sudo docker info >/dev/null 2>&1; do sleep 2; done
# nbgrader working dirs, owned by UID 1000 (= jovyan in the container) so it can
# write the gradebook DB and create assignments (else: root-owned -> EACCES).
mkdir -p /home/vmuser/nbgrader/source /home/vmuser/nbgrader/exchange /home/vmuser/nbgrader/submitted_copies
# nbgrader config (course + writable exchange), mounted over the image default.
cat > /home/vmuser/nbgrader_config.py <<'NBCFG'
c = get_config()
c.CourseDirectory.course_id = 'jupyter'
c.CourseDirectory.root = '/home/jovyan/nbgrader'
c.Exchange.root = '/home/jovyan/nbgrader/exchange'
NBCFG
chown -R 1000:1000 /home/vmuser/nbgrader /home/vmuser/nbgrader_config.py
# base_url Jupyter calé sur l'UUID de la VM → sert derrière /api/jupyter-proxy/{uuid}/
# (le control center proxifie sous ce préfixe ; sans base_url Jupyter répondrait 404).
# Récupéré sans jq (pas garanti sur l'hôte).
VM_UUID=$(curl -s http://169.254.169.254/openstack/latest/meta_data.json | tr ',' '\n' | grep -m1 '"uuid"' | sed -E 's/.*"uuid"[: ]+"([^"]+)".*/\1/')
JBASE="/api/jupyter-proxy/${VM_UUID}/"
sudo docker rm -f jupyter 2>/dev/null || true
sudo docker run -d --restart=always --name jupyter \
  -p 8888:8888 \
  -e JBASE="$JBASE" \
  -v /home/vmuser/nbgrader:/home/jovyan/nbgrader \
  -v /home/vmuser/nbgrader_config.py:/home/jovyan/nbgrader_config.py \
  -v /home/vmuser:/home/jovyan/work \
  ${nbgrader_tag} \
  bash -lc 'pip install -U packaging >/dev/null 2>&1 || true; exec jupyter lab --ip=0.0.0.0 --port=8888 --no-browser --ServerApp.token="" --ServerApp.password="" --ServerApp.allow_origin="*" --ServerApp.allow_remote_access=True --ServerApp.disable_check_xsrf=True --ServerApp.base_url="$JBASE"'
# VS Code (code-server) à côté de Jupyter, sur le port 8080, montant le même
# /home/vmuser -> mêmes fichiers que Jupyter (qui le voit dans /home/jovyan/work).
# --auth none : cohérent avec Jupyter lancé sans token (contrôle = frontière réseau).
# Image tirée via le proxy registry Polytechnique (pas de modif d'image VM).
sudo docker rm -f codeserver 2>/dev/null || true
# --network host : code-server peut joindre le serveur Jupyter (localhost:8888)
# pour exécuter les notebooks avec le MÊME environnement/libs que JupyterLab.
# Extensions Python+Jupyter installées au démarrage (depuis Open VSX).
sudo docker run -d --restart=always --name codeserver \
  --network host --entrypoint /bin/bash \
  -v /home/vmuser:/home/coder/project \
  registry.virtualdata.cloud.idcs.polytechnique.fr/docker-hub-proxy/codercom/code-server:latest \
  -lc 'code-server --install-extension ms-python.python --install-extension ms-toolsai.jupyter || true; exec code-server --auth none --cert --bind-addr 0.0.0.0:8443 /home/coder/project'
# 2ᵉ instance code-server EN LECTURE SEULE sur 8444, pour le partage « lecture » entre
# élèves (le proxy y route les invités en mode read). Le projet est monté ':ro' : c'est
# un verrou AU NIVEAU OS — ni l'éditeur ni le terminal ne peuvent modifier les fichiers,
# contrairement à un simple réglage VS Code contournable. files.readonlyInclude rend en
# plus l'éditeur explicitement en lecture seule (meilleure UX).
sudo docker rm -f codeserver-ro 2>/dev/null || true
sudo docker run -d --restart=always --name codeserver-ro \
  --network host --entrypoint /bin/bash \
  -v /home/vmuser:/home/coder/project:ro \
  registry.virtualdata.cloud.idcs.polytechnique.fr/docker-hub-proxy/codercom/code-server:latest \
  -lc 'mkdir -p ~/.local/share/code-server/User; printf "{\"files.readonlyInclude\":{\"**/*\":true}}" > ~/.local/share/code-server/User/settings.json; exec code-server --auth none --cert --bind-addr 0.0.0.0:8444 /home/coder/project'
SCRIPT
)
  if [ -n "$POSTGRES_DSN" ] && command -v psql &>/dev/null; then
    psql "$POSTGRES_DSN" -c "
      INSERT INTO config_pools (user_id, name, data)
      VALUES ('system', '${config_name}', \$\$${script}\$\$)
      ON CONFLICT (user_id, name) DO UPDATE SET data = EXCLUDED.data;
    " &>/dev/null && log "[$suffix] Config '$config_name' upserted in PostgreSQL." || true
  fi
}

# Build the nbgrader-enriched Docker image on the remote VM via scp + docker build.
build_nbgrader_image() {
  local ip=$1 base_image=$2

  log "  Writing Dockerfile for nbgrader layer (base: $base_image)..."

  # Write Dockerfile to a temp file locally then scp it
  local tmpdir
  tmpdir=$(mktemp -d)
  # Write Dockerfile with base_image expanded directly (no ARG hack, no sed)
  cat << EOF > "$tmpdir/Dockerfile"
FROM ${base_image}
USER root
# sqlite3 needed by nbgrader gradebook
RUN apt-get update -qq && apt-get install -y --no-install-recommends sqlite3 && rm -rf /var/lib/apt/lists/* || true
# Upgrade pip to avoid compatibility issues with older Python environments and then install nbgrader
RUN pip3 install --upgrade pip 2>/dev/null || pip install --upgrade pip 2>/dev/null || true
# -U: force the latest nbgrader. Without it, pip is a no-op when the base image
# ships an ancient nbgrader (e.g. 0.7.0.dev0) whose formgrader API crashes
# (AttributeError: 'CompoundSelect' object has no attribute 'mapper').
RUN pip3 install --quiet -U nbgrader 2>/dev/null || pip install --quiet -U nbgrader
# nbgrader can pin an old 'packaging'; JupyterLab 4 needs a newer one
# (else: ImportError: cannot import name 'InvalidName' from 'packaging.utils')
RUN pip3 install --quiet -U packaging 2>/dev/null || pip install --quiet -U packaging 2>/dev/null || true
# Enable notebook extensions
RUN jupyter nbextension install --sys-prefix --py nbgrader --overwrite --quiet 2>/dev/null || true
RUN jupyter nbextension enable  --sys-prefix --py nbgrader --quiet 2>/dev/null || true
RUN jupyter serverextension enable --sys-prefix --py nbgrader --quiet 2>/dev/null || true
RUN jupyter server extension enable --sys-prefix --py nbgrader --quiet 2>/dev/null || true
RUN mkdir -p /home/jovyan/nbgrader && chown 1000:100 /home/jovyan/nbgrader
RUN echo "c = get_config()" > /home/jovyan/nbgrader_config.py && \
    echo "c.CourseDirectory.course_id = 'jupyter'" >> /home/jovyan/nbgrader_config.py && \
    echo "c.CourseDirectory.root = '/home/jovyan/nbgrader'" >> /home/jovyan/nbgrader_config.py && \
    chown 1000:100 /home/jovyan/nbgrader_config.py
USER jovyan
EOF

  scp -o StrictHostKeyChecking=no -i "$SSH_KEY" "$tmpdir/Dockerfile" "vmuser@$ip:/home/vmuser/Dockerfile" || {
    rm -rf "$tmpdir"
    log "  WARNING: scp Dockerfile failed"
    return 1
  }
  rm -rf "$tmpdir"

  log "  Pulling base image then building ${NBGRADER_LOCAL_TAG} (may take a few minutes)..."
  ssh -o StrictHostKeyChecking=no -i "$SSH_KEY" "vmuser@$ip" bash -s << SSHEOF
set -e
sudo docker pull '${base_image}'
sudo docker build --no-cache -t '${NBGRADER_LOCAL_TAG}' /home/vmuser/
rm -f /home/vmuser/Dockerfile
echo "[build] Done."
SSHEOF
}

process_env() {
  local suffix=$1 docker_image=$2 label=$3
  local snapshot_name="${SNAPSHOT_PREFIX}-${suffix}"
  local vm_name="snapshot-builder-${suffix}-$$"

  upsert_config "$suffix"

  # Skip if snapshot already exists
  if openstack --os-cloud "$OS_CLOUD" image show "$snapshot_name" &>/dev/null; then
    log "[$suffix] Snapshot '$snapshot_name' already exists, skipping."
    return 0
  fi

  log "[$suffix] Starting VM '$vm_name'..."
  local vm_id
  vm_id=$(openstack --os-cloud "$OS_CLOUD" server create \
    --image "$BASE_IMAGE" \
    --flavor "$FLAVOR" \
    --network "$NETWORK" \
    --key-name "$KEYPAIR" \
    --wait \
    --format value -c id \
    "$vm_name")

  log "[$suffix] VM $vm_id created. Getting IP..."
  local ip=""
  for i in $(seq 1 30); do
    ip=$(openstack --os-cloud "$OS_CLOUD" server show "$vm_id" \
      --format value -c addresses 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+' | head -1 || true)
    [ -n "$ip" ] && break
    sleep 3
  done
  [ -z "$ip" ] && { openstack --os-cloud "$OS_CLOUD" server delete "$vm_id" --wait; die "[$suffix] No IP found"; }

  wait_ssh "$ip"

  # Build the nbgrader-enriched image. NO fallback to a plain pull+tag: a plain
  # course image has no formgrader (-> 404) and silently masks the failure.
  if ! build_nbgrader_image "$ip" "$docker_image"; then
    openstack --os-cloud "$OS_CLOUD" server delete "$vm_id" --wait 2>/dev/null || true
    die "[$suffix] ABANDON: build de l'image nbgrader échoué — snapshot non créé."
  fi

  # Verify formgrader is REALLY in the image before snapshotting, otherwise we
  # bake a broken snapshot (no image -> container never starts; plain image ->
  # 404 formgrader). Both failure modes are caught here.
  if ! ssh -o StrictHostKeyChecking=no -i "$SSH_KEY" "vmuser@$ip" \
        "sudo docker run --rm --entrypoint bash '${NBGRADER_LOCAL_TAG}' -lc 'jupyter server extension list 2>&1 | grep -qi formgrader'" 2>/dev/null; then
    openstack --os-cloud "$OS_CLOUD" server delete "$vm_id" --wait 2>/dev/null || true
    die "[$suffix] ABANDON: '${NBGRADER_LOCAL_TAG}' n'a pas l'extension formgrader — snapshot non créé."
  fi
  log "[$suffix] OK: formgrader présent dans l'image."

  log "[$suffix] Stopping VM for clean snapshot..."
  openstack --os-cloud "$OS_CLOUD" server stop "$vm_id"
  for i in $(seq 1 30); do
    status=$(openstack --os-cloud "$OS_CLOUD" server show "$vm_id" --format value -c status)
    [ "$status" = "SHUTOFF" ] && break
    sleep 3
  done

  log "[$suffix] Creating snapshot '$snapshot_name'..."
  snap_id=$(openstack --os-cloud "$OS_CLOUD" server image create \
    --name "$snapshot_name" \
    --format value -c id \
    "$vm_id" || true)
  if [ -z "$snap_id" ]; then
    log "[$suffix] WARNING: snapshot creation returned no ID, checking by name..."
    snap_id=$(openstack --os-cloud "$OS_CLOUD" image list --format value -c ID -c Name | grep "$snapshot_name" | awk '{print $1}' || true)
  fi
  if [ -n "$snap_id" ]; then
    log "[$suffix] Waiting for snapshot $snap_id to become active..."
    for i in $(seq 1 60); do
      snap_status=$(openstack --os-cloud "$OS_CLOUD" image show "$snap_id" --format value -c status 2>/dev/null || echo "error")
      if [ "$snap_status" = "active" ]; then
        log "[$suffix] Snapshot is active."
        break
      fi
      log "[$suffix] Snapshot status: $snap_status (attempt $i/60)..."
      sleep 15
    done
  fi

  log "[$suffix] Deleting build VM..."
  openstack --os-cloud "$OS_CLOUD" server delete "$vm_id" || true
  sleep 10
  log "[$suffix] Done. Snapshot '$snapshot_name' is ready."
}

log "Starting Jupyter snapshot builder (OS_CLOUD=$OS_CLOUD)"
log "Base image: $BASE_IMAGE | Flavor: $FLAVOR | Network: $NETWORK"
log "nbgrader enriched tag: $NBGRADER_LOCAL_TAG"
echo ""

for entry in "${ENVS[@]}"; do
  IFS='|' read -r suffix docker_image label <<< "$entry"
  if [ -n "$FILTER" ] && [ "$suffix" != "$FILTER" ]; then
    continue
  fi
  log "=== Processing: $label ($suffix) ==="
  process_env "$suffix" "$docker_image" "$label"
  echo ""
done

log "All done."
echo ""
log "Snapshots created (prefix: $SNAPSHOT_PREFIX-):"
openstack --os-cloud "$OS_CLOUD" image list --format value -c Name | grep "^${SNAPSHOT_PREFIX}-" | sort
