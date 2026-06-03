#!/bin/bash
# Robust sequential snapshot worker. Run once; handles quota waits and failures.
# State is tracked via marker files so it can be safely restarted.
# Usage: nohup bash scripts/snapshot-worker.sh >> /tmp/snapshot-worker.log 2>&1 &

set -uo pipefail

OS_CLOUD="ipp-idcs-vmpool"
BASE_IMAGE="ubuntu-2204-docker"
FLAVOR="vd.24"
NETWORK="public-2"
KEYPAIR="maelan-mac"
SSH_KEY="${SSH_PRIVATE_KEY_PATH:-$HOME/.ssh/id_ed25519}"
STATE_DIR="/tmp/jupyter-snapshot-state"
mkdir -p "$STATE_DIR"

# Local Docker tag baked into every snapshot – the startup script always uses this name.
NBGRADER_LOCAL_TAG="jupyter-nbgrader:latest"

log() { echo "[$(date '+%H:%M:%S')] $*"; }

ENVS=(
  "scipy|registry.virtualdata.cloud.idcs.polytechnique.fr/docker-hub-proxy/jupyter/scipy-notebook:latest"
  "scipy-plus|registry.virtualdata.cloud.idcs.polytechnique.fr/plmlab-hub-proxy/docker-images/scipy-notebook-plus:2023.01.24"
  "datascience|registry.virtualdata.cloud.idcs.polytechnique.fr/docker-hub-proxy/jupyter/datascience-notebook:2343e33dec46"
  "julia|registry.virtualdata.cloud.idcs.polytechnique.fr/plmlab-hub-proxy/docker-images/julia:0.0.4"
  "bio583|registry.virtualdata.cloud.idcs.polytechnique.fr/plmlab-hub-proxy/ip-paris/idcs/docker/bio583:0.0.1"
  "eco589|registry.virtualdata.cloud.idcs.polytechnique.fr/gitlab-in2p3-proxy/energy4climate/public/education/eco-589-tutorials:0.2"
  "compeco|albop/computational_economics:latest"
  "mec431|registry.virtualdata.cloud.idcs.polytechnique.fr/gitlab-hub-proxy/bleyerj/x_mec431:040520231145"
  "mec558|registry.virtualdata.cloud.idcs.polytechnique.fr/gitlab-in2p3-proxy/ipsl/lmd/intro/jupyterlabimages:07-11-2023"
  "map579|registry.virtualdata.cloud.idcs.polytechnique.fr/plmlab-hub-proxy/docker-images/xeus-cling:0.0.5"
  "mec552a|registry.virtualdata.cloud.idcs.polytechnique.fr/gitlab-inria-proxy/mgenet/mec552a-repo2docker:latest"
  "mec552b|registry.virtualdata.cloud.idcs.polytechnique.fr/jupyter/mec552b-repo2docker:1d894fa3"
  "mec568|registry.virtualdata.cloud.idcs.polytechnique.fr/gitlab-inria-proxy/mgenet/mec568-repo2docker:latest"
  "mec581|registry.virtualdata.cloud.idcs.polytechnique.fr/gitlab-inria-proxy/mgenet/mec-581-repo-2-docker:9b9d98b7"
  "mec666|registry.virtualdata.cloud.idcs.polytechnique.fr/gitlab-in2p3-proxy/energy4climate/public/education/climate_change_and_energy_transition:0.2"
)

free_cores() {
  local out
  out=$(openstack --os-cloud "$OS_CLOUD" limits show --absolute --format value 2>/dev/null || true)
  local used max
  used=$(echo "$out" | awk '/total_cores_used/{print $2}')
  max=$(echo "$out" | awk '/max_total_cores/{print $2}')
  used=${used:-100}; max=${max:-100}
  echo $((max - used))
}

wait_for_cores() {
  local needed=24
  while true; do
    local free; free=$(free_cores)
    log "Cores free: $free (need $needed)"
    [ "$free" -ge "$needed" ] && return 0
    sleep 30
  done
}

snapshot_exists() { openstack --os-cloud "$OS_CLOUD" image show "jupyter-snapshot-$1" &>/dev/null; }

wait_ssh() {
  local ip=$1
  for i in $(seq 1 60); do
    ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 -o BatchMode=yes \
        -i "$SSH_KEY" "vmuser@$ip" "true" 2>/dev/null && return 0
    sleep 5
  done
  return 1
}

# Build nbgrader-enriched image on the remote VM via scp + docker build
build_nbgrader_image() {
  local ip=$1 base_image=$2

  log "  Building nbgrader image on top of: $base_image"

  local tmpdir
  tmpdir=$(mktemp -d)
  cat > "$tmpdir/Dockerfile" << DOCKERFILE
FROM ${base_image}
USER root
RUN apt-get update -qq && apt-get install -y --no-install-recommends sqlite3 && rm -rf /var/lib/apt/lists/* || true
RUN pip3 install --quiet nbgrader 2>/dev/null || pip install --quiet nbgrader
# JupyterLab 4 needs a newer 'packaging' than nbgrader may leave behind
RUN pip3 install --quiet -U packaging 2>/dev/null || pip install --quiet -U packaging 2>/dev/null || true
RUN jupyter nbextension install --sys-prefix --py nbgrader --overwrite --quiet 2>/dev/null || true && \
    jupyter nbextension enable  --sys-prefix --py nbgrader --quiet 2>/dev/null || true && \
    jupyter serverextension enable --sys-prefix --py nbgrader --quiet 2>/dev/null || true
RUN mkdir -p /home/jovyan/nbgrader
USER jovyan
DOCKERFILE

  scp -o StrictHostKeyChecking=no -i "$SSH_KEY" "$tmpdir/Dockerfile" "vmuser@$ip:/home/vmuser/Dockerfile"
  local scp_status=$?
  rm -rf "$tmpdir"
  [ "$scp_status" -ne 0 ] && { log "  WARNING: scp Dockerfile failed"; return 1; }

  ssh -o StrictHostKeyChecking=no -i "$SSH_KEY" "vmuser@$ip" bash -s << SSHEOF
set -e
sudo docker pull '${base_image}'
sudo docker build --no-cache -t '${NBGRADER_LOCAL_TAG}' /home/vmuser/
rm -f /home/vmuser/Dockerfile
echo "Build complete."
SSHEOF
}

process() {
  local suffix=$1 docker_image=$2
  local snap_name="jupyter-snapshot-$suffix"
  local state_file="$STATE_DIR/$suffix"
  local vm_id=""

  if snapshot_exists "$suffix"; then
    log "[$suffix] Already done, skipping."
    return 0
  fi

  # Resume from a previously started VM if state file exists
  if [ -f "$state_file" ]; then
    vm_id=$(cat "$state_file")
    if ! openstack --os-cloud "$OS_CLOUD" server show "$vm_id" &>/dev/null; then
      log "[$suffix] Stale state, starting fresh."
      vm_id=""
      rm -f "$state_file"
    else
      log "[$suffix] Resuming with existing VM $vm_id"
    fi
  fi

  # Create VM if needed
  if [ -z "$vm_id" ]; then
    wait_for_cores
    log "[$suffix] Creating VM..."
    vm_id=$(openstack --os-cloud "$OS_CLOUD" server create \
      --image "$BASE_IMAGE" --flavor "$FLAVOR" --network "$NETWORK" \
      --key-name "$KEYPAIR" --wait --format value -c id \
      "snap-builder-$suffix-$$") || { log "[$suffix] VM creation failed"; return 1; }
    echo "$vm_id" > "$state_file"
    log "[$suffix] VM $vm_id created."
  fi

  # Get IP
  local ip=""
  for i in $(seq 1 30); do
    ip=$(openstack --os-cloud "$OS_CLOUD" server show "$vm_id" \
      --format value -c addresses 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+' | head -1 || true)
    [ -n "$ip" ] && break
    sleep 3
  done
  if [ -z "$ip" ]; then
    log "[$suffix] No IP, deleting VM and aborting."
    openstack --os-cloud "$OS_CLOUD" server delete "$vm_id" || true
    rm -f "$state_file"
    return 1
  fi

  wait_ssh "$ip" || {
    log "[$suffix] SSH failed, cleaning up."
    openstack --os-cloud "$OS_CLOUD" server delete "$vm_id" || true
    rm -f "$state_file"
    return 1
  }

  local vm_status
  vm_status=$(openstack --os-cloud "$OS_CLOUD" server show "$vm_id" --format value -c status 2>/dev/null || echo "UNKNOWN")
  if [ "$vm_status" != "SHUTOFF" ]; then
    # Build nbgrader-enriched image; fallback to plain tag if build fails
    if ! build_nbgrader_image "$ip" "$docker_image"; then
      log "[$suffix] nbgrader build failed — falling back to plain pull + tag"
      ssh -o StrictHostKeyChecking=no -i "$SSH_KEY" "vmuser@$ip" \
        "sudo docker pull '${docker_image}' && sudo docker tag '${docker_image}' '${NBGRADER_LOCAL_TAG}'" || \
        log "[$suffix] WARNING: fallback pull also failed"
    fi

    log "[$suffix] Stopping VM..."
    openstack --os-cloud "$OS_CLOUD" server stop "$vm_id"
    for i in $(seq 1 30); do
      vm_status=$(openstack --os-cloud "$OS_CLOUD" server show "$vm_id" --format value -c status 2>/dev/null || echo "UNKNOWN")
      [ "$vm_status" = "SHUTOFF" ] && break
      sleep 3
    done
  else
    log "[$suffix] VM already SHUTOFF, creating snapshot directly."
  fi

  log "[$suffix] Creating snapshot '$snap_name'..."
  local snap_id
  snap_id=$(openstack --os-cloud "$OS_CLOUD" server image create \
    --name "$snap_name" --format value -c id "$vm_id" 2>/dev/null || true)

  if [ -z "$snap_id" ]; then
    snap_id=$(openstack --os-cloud "$OS_CLOUD" image list --format value -c ID -c Name 2>/dev/null | \
      awk -v n="$snap_name" '$2==n{print $1}' || true)
  fi

  if [ -n "$snap_id" ]; then
    log "[$suffix] Waiting for snapshot $snap_id..."
    for i in $(seq 1 80); do
      local s; s=$(openstack --os-cloud "$OS_CLOUD" image show "$snap_id" --format value -c status 2>/dev/null || echo "error")
      [ "$s" = "active" ] && { log "[$suffix] Snapshot active!"; break; }
      log "[$suffix] Snapshot status: $s ($i/80)"
      sleep 15
    done
  else
    log "[$suffix] WARNING: could not get snapshot ID"
  fi

  log "[$suffix] Deleting VM..."
  openstack --os-cloud "$OS_CLOUD" server delete "$vm_id" || true
  rm -f "$state_file"
  sleep 5
}

log "=== Snapshot worker started (PID $$) — nbgrader enriched images ==="
for entry in "${ENVS[@]}"; do
  IFS='|' read -r suffix docker_image <<< "$entry"
  process "$suffix" "$docker_image"
done

log "=== All done ==="
log "Snapshots available:"
openstack --os-cloud "$OS_CLOUD" image list --format value -c Name | grep "^jupyter-snapshot-" | sort
