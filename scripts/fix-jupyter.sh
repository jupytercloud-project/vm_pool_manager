#!/bin/bash
set -euo pipefail

POOL_ID=$(curl -s http://169.254.169.254/openstack/latest/meta_data.json | jq -r .meta.serverpool_id)
USER_ID=$(curl -s http://169.254.169.254/openstack/latest/meta_data.json | jq -r .meta.user_id)
JUPYTER_BASE_URL="/api/jupyter-proxy/${POOL_ID}/${USER_ID}/"

mkdir -p /home/vmuser/nbgrader/{source,release,submitted,autograded,feedback,exchange}

chown -R vmuser:vmuser /home/vmuser/nbgrader /home/vmuser/jupyter-env || true

cat > /home/vmuser/nbgrader/nbgrader_config.py << 'NBCFG'
c = get_config()
c.CourseDirectory.root = '/home/vmuser/nbgrader'
c.CourseDirectory.course_id = 'course'
c.Exchange.root = '/home/vmuser/nbgrader/exchange'
NBCFG
chown vmuser:vmuser /home/vmuser/nbgrader/nbgrader_config.py

cat > /etc/systemd/system/jupyterlab.service << SVC
[Unit]
Description=JupyterLab
After=network.target

[Service]
Type=simple
User=vmuser
WorkingDirectory=/home/vmuser/nbgrader
ExecStart=/home/vmuser/jupyter-env/bin/jupyter lab \
  --no-browser --ip=0.0.0.0 --port=8888 \
  --ServerApp.token='' \
  --ServerApp.password='' \
  --ServerApp.allow_origin='*' \
  --ServerApp.allow_remote_access=True \
  --ServerApp.base_url=${JUPYTER_BASE_URL} \
  --ServerApp.disable_check_xsrf=True
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
SVC

systemctl daemon-reload
systemctl unmask jupyterlab || true
systemctl enable jupyterlab

# Enable nbgrader extensions
sudo -u vmuser /home/vmuser/jupyter-env/bin/jupyter nbextension install --sys-prefix --py nbgrader --overwrite
sudo -u vmuser /home/vmuser/jupyter-env/bin/jupyter nbextension enable --sys-prefix --py nbgrader
sudo -u vmuser /home/vmuser/jupyter-env/bin/jupyter serverextension enable --sys-prefix --py nbgrader

systemctl restart jupyterlab
