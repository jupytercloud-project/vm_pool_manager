#!/bin/bash
# Runs all Jupyter snapshot builds sequentially (quota constraint: only 1 VM at a time).
# Waits for quota to free up before each env.
# Logs go to /tmp/snap-{env}.log

cd "$(dirname "$0")/.."

ENVS=(scipy scipy-plus datascience julia bio583 eco589 compeco mec431 mec558 map579 mec552a mec552b mec568 mec581 mec666)

wait_for_quota() {
  local needed=24
  echo "[$(date +%H:%M:%S)] Waiting for $needed cores to be available..."
  while true; do
    limits=$(openstack --os-cloud ipp-idcs-vmpool limits show --absolute --format value 2>/dev/null || true)
    used=$(echo "$limits" | grep "total_cores_used" | awk '{print $2}' || echo 100)
    max=$(echo "$limits" | grep "max_total_cores" | awk '{print $2}' || echo 100)
    used=${used:-100}; max=${max:-100}
    free=$((max - used))
    echo "[$(date +%H:%M:%S)] Cores: $used/$max used, $free free (need $needed)"
    if [ "$free" -ge "$needed" ]; then
      echo "[$(date +%H:%M:%S)] Quota available, proceeding."
      return 0
    fi
    sleep 30
  done
}

for env in "${ENVS[@]}"; do
  # Skip if snapshot already exists
  if openstack --os-cloud ipp-idcs-vmpool image show "jupyter-snapshot-${env}" &>/dev/null; then
    echo "[$(date +%H:%M:%S)] [${env}] Snapshot already exists, skipping."
    continue
  fi
  wait_for_quota
  echo "[$(date +%H:%M:%S)] === Starting $env ==="
  bash scripts/make-jupyter-snapshots.sh "$env" 2>&1 | tee "/tmp/snap-${env}.log"
  echo "[$(date +%H:%M:%S)] === Done $env ==="
  echo ""
  sleep 5
done

echo "All snapshots done."
openstack --os-cloud ipp-idcs-vmpool image list --format value -c Name | grep "^jupyter-snapshot-" | sort
