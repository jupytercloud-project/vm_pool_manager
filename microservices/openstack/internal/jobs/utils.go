package jobs

import (
	"fmt"
	"regexp"
	"strings"
)

func nfsCloudConfig(userID, serverpoolID string) string {
	nfsHost := sanitizeHostname(fmt.Sprintf("%s-%s-NFS", userID, serverpoolID))

	return fmt.Sprintf(`#cloud-config
hostname: %s
fqdn: %s.local
manage_etc_hosts: true

package_update: true
packages:
  - nfs-kernel-server

write_files:
  - path: /etc/exports
    owner: root:root
    permissions: '0644'
    content: |
      /srv/nfs *(rw,sync,no_root_squash,no_subtree_check)

runcmd:
  - mkdir -p /srv/nfs
  - chown nobody:nogroup /srv/nfs
  - exportfs -ra
  - systemctl enable nfs-server
  - systemctl restart nfs-server
`, nfsHost, nfsHost)
}

func baseUserConfig(sshKey string) string {
	return fmt.Sprintf(`#cloud-config
users:
  - name: vmuser
    shell: /bin/bash
    sudo: ALL=(ALL) NOPASSWD:ALL
    groups: sudo
    ssh_authorized_keys:
      - %s

package_update: true
package_upgrade: true
packages:
  - fuse3
  - unzip

runcmd:
  - curl https://rclone.org/install.sh | bash
  - echo "Installation de rclone terminee"
`, sshKey)
}

func computeNFSCloudConfig(nfsIP string) string {
	return fmt.Sprintf(`#cloud-config
package_update: true
packages:
  - nfs-common

write_files:
  - path: /usr/local/bin/mount-nfs.sh
    permissions: '0755'
    owner: root:root
    content: |
      #!/bin/bash
      set -e

      NFS_IP="%s"
      NFS_EXPORT="/srv/nfs"
      MOUNT_POINT="/mnt/pool"

      mkdir -p ${MOUNT_POINT}

      echo "[NFS] Waiting for NFS server ${NFS_IP}"
      until showmount -e ${NFS_IP} >/dev/null 2>&1; do
        sleep 5
      done

      if ! mountpoint -q ${MOUNT_POINT}; then
        mount -t nfs ${NFS_IP}:${NFS_EXPORT} ${MOUNT_POINT}
      fi

      if ! grep -q "${NFS_IP}:${NFS_EXPORT}" /etc/fstab; then
        echo "${NFS_IP}:${NFS_EXPORT} ${MOUNT_POINT} nfs defaults,_netdev,x-systemd.automount 0 0" >> /etc/fstab
      fi

runcmd:
  - /usr/local/bin/mount-nfs.sh
`, nfsIP)
}

func initRclone() string {
	return fmt.Sprintf(`#cloud-config
package_update: true
package_upgrade: true

packages:
  - fuse
  - fuse3
  - unzip

runcmd:
  - curl https://rclone.org/install.sh | bash
`)
}

func sanitizeHostname(s string) string {
	s = strings.ToLower(s)
	s = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(s, "-")
	return s
}
