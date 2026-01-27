package jobs

import (
	"fmt"
	"regexp"
	"strings"
)

func mountNFSScript(userID, serverpoolID string) string {
	nfsHost := sanitizeHostname(fmt.Sprintf("%s-%s-NFS", userID, serverpoolID))

	return fmt.Sprintf(`#!/bin/bash
set -e

apt-get update
apt-get install -y nfs-common

mkdir -p /mnt/pool

echo "Waiting for DNS (%s)..."
until getent hosts %s >/dev/null 2>&1; do
  sleep 5
done

echo "Waiting for NFS server..."
until showmount -e %s >/dev/null 2>&1; do
  sleep 5
done

mount %s:/srv/nfs /mnt/pool

echo "%s:/srv/nfs /mnt/pool nfs defaults,_netdev,x-systemd.automount 0 0" >> /etc/fstab
`, nfsHost, nfsHost, nfsHost, nfsHost, nfsHost)
}

func installNFSClient() string {
	return `#!/bin/bash

# Installer le client NFS sur la VM (nfs-common)

# Met à jour la liste des paquets
apt-get update

# Installe nfs-common s'il n'est pas déjà présent
if ! dpkg -l | grep -qw nfs-common; then
    apt-get install -y nfs-common
else
    echo "nfs-common déjà installé"
fi

echo "Installation de nfs-common terminée"
`
}

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

package-update: true
packages:
  - nfs-common

runcmd:
  - echo "Installation de nfs-common terminee"
`, sshKey)
}

func sanitizeHostname(s string) string {
	s = strings.ToLower(s)
	s = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(s, "-")
	return s
}
