#!/bin/bash
# make-nbgrader-snapshot.sh
# Creates an OpenStack snapshot with JupyterLab + nbgrader for the instructor VM.
# Usage: ./scripts/make-nbgrader-snapshot.sh <base-vm-ip> [snapshot-name]
#
# Prerequisites:
#   - SSH access to the base VM (vmuser@<ip>) using SSH_PRIVATE_KEY_PATH
#   - openstack CLI configured (clouds.yaml)
#   - The base VM must be running Ubuntu 22.04+

set -euo pipefail

BASE_IP="${1:?Usage: $0 <base-vm-ip> [snapshot-name]}"
SNAPSHOT_NAME="${2:-jupyter-nbgrader-instructor}"
SSH_KEY="${SSH_PRIVATE_KEY_PATH:-$HOME/.ssh/id_ed25519}"

echo "=== Installing JupyterLab + nbgrader on $BASE_IP ==="

ssh -i "$SSH_KEY" -o StrictHostKeyChecking=no "vmuser@$BASE_IP" bash <<'EOF'
set -euo pipefail

# Install Python + pip if not present
sudo apt-get update -qq
sudo apt-get install -y python3-pip python3-venv git sqlite3

# Install JupyterLab + nbgrader in a virtualenv
python3 -m venv /home/vmuser/jupyter-env
source /home/vmuser/jupyter-env/bin/activate

pip install --quiet --upgrade pip
pip install --quiet jupyterlab nbgrader "notebook<7"

# Enable nbgrader extensions
jupyter nbextension install --sys-prefix --py nbgrader
jupyter nbextension enable --sys-prefix --py nbgrader
jupyter serverextension enable --sys-prefix --py nbgrader

# Create nbgrader course structure
mkdir -p /home/vmuser/nbgrader/{source,release,submitted,feedback}
mkdir -p /home/vmuser/nbgrader/autograded

# Write nbgrader_config.py for instructor
cat > /home/vmuser/nbgrader/nbgrader_config.py << 'NBCFG'
c = get_config()
c.CourseDirectory.root = '/home/vmuser/nbgrader'
c.CourseDirectory.course_id = 'course'
# Students submit to: submitted/{student}/{assignment}/
c.CourseDirectory.submitted_directory = 'submitted'
c.CourseDirectory.autograded_directory = 'autograded'
c.CourseDirectory.feedback_directory = 'feedback'
c.Exchange.root = '/home/vmuser/nbgrader/exchange'
NBCFG

# Install NFS client
sudo apt-get install -y nfs-common jq curl

# Create a wrapper script to fetch metadata and start Jupyter
sudo tee /usr/local/bin/start-jupyterlab.sh > /dev/null << 'WRAPPER'
#!/bin/bash
set -a
# Mount NFS if NFS_SERVER_IP is provided in metadata or hardcoded
# In a real environment, you might fetch this from metadata as well.
NFS_SERVER_IP="157.136.249.198" # Default placeholder, will be updated by deploy script or user
mkdir -p /home/vmuser/nbgrader/exchange
sudo mount -t nfs ${NFS_SERVER_IP}:/srv/nbgrader/exchange /home/vmuser/nbgrader/exchange || true

# Get metadata for pool_id and user_id to correctly set the Jupyter base_url
POOL_ID=$(curl -s http://169.254.169.254/openstack/latest/meta_data.json | jq -r .meta.serverpool_id || echo "pool")
USER_ID=$(curl -s http://169.254.169.254/openstack/latest/meta_data.json | jq -r .meta.user_id || echo "user")
JUPYTER_BASE_URL="/api/jupyter-proxy/${POOL_ID}/${USER_ID}/"

exec /home/vmuser/jupyter-env/bin/jupyter lab \
  --no-browser --ip=0.0.0.0 --port=8888 \
  --ServerApp.token='' --ServerApp.password='' \
  --ServerApp.allow_origin=* --ServerApp.allow_remote_access=True \
  --ServerApp.base_url=${JUPYTER_BASE_URL}
WRAPPER
sudo chmod +x /usr/local/bin/start-jupyterlab.sh

# Create a systemd service to auto-start JupyterLab on boot
sudo tee /etc/systemd/system/jupyterlab.service > /dev/null << 'SVC'
[Unit]
Description=JupyterLab for instructor
After=network.target

[Service]
Type=simple
User=vmuser
WorkingDirectory=/home/vmuser/nbgrader
ExecStart=/usr/local/bin/start-jupyterlab.sh
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
SVC

sudo systemctl daemon-reload
sudo systemctl enable jupyterlab
sudo systemctl start jupyterlab

echo "✅ JupyterLab + nbgrader installed and started on port 8888"
EOF

echo "=== Creating OpenStack snapshot: $SNAPSHOT_NAME ==="

# Find the instance ID from IP
INSTANCE_ID=$(openstack server list --format json | python3 -c "
import json, sys
data = json.load(sys.stdin)
for s in data:
    if '$BASE_IP' in str(s.get('Networks', '')):
        print(s['ID'])
        break
")

if [ -z "$INSTANCE_ID" ]; then
    echo "❌ Could not find instance with IP $BASE_IP"
    exit 1
fi

echo "Instance ID: $INSTANCE_ID"
openstack server image create --name "$SNAPSHOT_NAME" "$INSTANCE_ID"
echo "✅ Snapshot '$SNAPSHOT_NAME' created"
echo ""
echo "Next steps:"
echo "  1. In CloudPoolManager, create a pool with:"
echo "     - Image: $SNAPSHOT_NAME"
echo "     - MaxVM: 1 (one instructor VM)"
echo "     - AppPort: 8888"
echo "  2. Start the pool to provision the instructor VM"
echo "  3. In the Notation page, select this pool to grade assignments"
