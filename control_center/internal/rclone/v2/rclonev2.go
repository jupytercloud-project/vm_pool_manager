package rclonev2

import (
	"bytes"
	"control_center/config"
	"control_center/internal/sshinject"
	"control_center/models"
	"fmt"
	"log"
	"os"
	"os/exec"

	"golang.org/x/crypto/ssh"
)

//LOCAL DEPOT VM

func createDepotUserCmdSecure(username string) string {
	return fmt.Sprintf(`
set -e

if ! id "%[1]s" >/dev/null 2>&1; then
    sudo useradd -m -s /bin/bash "%[1]s"
fi
`, username)
}

func authorizeDepotKey(username, pubKey string) error {
	cmd := fmt.Sprintf(`
set -e

sudo install -d -m 700 -o "%[1]s" -g "%[1]s" /home/%[1]s/.ssh
sudo touch /home/%[1]s/.ssh/authorized_keys
sudo chown "%[1]s":"%[1]s" /home/%[1]s/.ssh/authorized_keys
sudo chmod 600 /home/%[1]s/.ssh/authorized_keys

if ! sudo grep -qxF "%[2]s" /home/%[1]s/.ssh/authorized_keys; then
    printf "%%s\n" "%[2]s" | sudo tee -a /home/%[1]s/.ssh/authorized_keys > /dev/null
fi
`, username, pubKey)

	return runLocalCmd(cmd)
}

func ensureDepotPoolCmd(profname, poolname string) string {
	return fmt.Sprintf(`
sudo install -d -m 711 -o "%[1]s" -g "%[1]s" \
/home/%[1]s/depot/%[2]s
`, profname, poolname)
}

func ensureStudentFolderWithACLCmd(profname, poolname, student string) string {
	return fmt.Sprintf(`
set -e

# Création dossier étudiant
sudo install -d -m 700 -o "%[3]s" -g "%[3]s" \
/home/%[1]s/depot/%[2]s/%[3]s

# ACL : le prof peut tout faire
sudo setfacl -m u:%[1]s:rwx \
/home/%[1]s/depot/%[2]s/%[3]s

# ACL par défaut : nouveaux fichiers héritent
sudo setfacl -d -m u:%[1]s:rwx \
/home/%[1]s/depot/%[2]s/%[3]s
`, profname, poolname, student)
}

func linkPoolToStudentCmd(profname, poolname, student string) string {
	return fmt.Sprintf(`
sudo chmod 711 /home/%[3]s
sudo ln -sfn /home/%[1]s/depot/%[2]s \
/home/%[3]s/depot
`, profname, poolname, student)
}

//REMOTE VM

func ensureRemoteSSHKeyCmd(username string) string {
	return fmt.Sprintf(`
set -e
HOME=/home/%[1]s
SSH=$HOME/.ssh

sudo -u %[1]s mkdir -p "$SSH"
sudo -u %[1]s chmod 700 "$SSH"

if [ ! -f "$SSH/id_ed25519" ]; then
  sudo -u %[1]s ssh-keygen -t ed25519 -f "$SSH/id_ed25519" -N ""
fi

sudo -u %[1]s chmod 600 "$SSH/id_ed25519"
sudo -u %[1]s chmod 644 "$SSH/id_ed25519.pub"
`, username)
}

func readRemotePubKeyCmd(username string) string {
	return fmt.Sprintf(`sudo -u %s cat /home/%s/.ssh/id_ed25519.pub`, username, username)
}

func ensureRemoteMountPointCmd(username string) string {
	return fmt.Sprintf(`
sudo mkdir -p /home/%[1]s/depot
sudo chown %[1]s:%[1]s /home/%[1]s/depot
sudo chmod 700 /home/%[1]s/depot
`, username)
}

func ensureRemotePoolMountPointCmd(username, poolname string) string {
	return fmt.Sprintf(`
sudo mkdir -p /home/%[1]s/depot/%[2]s
sudo mkdir -p /home/%[1]s/depot/%[2]s/%[1]s

# ownership
sudo chown %[1]s:%[1]s /home/%[1]s/depot/%[2]s
sudo chown %[1]s:%[1]s /home/%[1]s/depot/%[2]s/%[1]s

# permissions
sudo chmod 755 /home/%[1]s/depot/%[2]s
sudo chmod 700 /home/%[1]s/depot/%[2]s/%[1]s
`, username, poolname)
}

// RCLONE CONFIG

func rcloneConfigCmd(username, profname string) string {
	return fmt.Sprintf(`
sudo -u %[1]s mkdir -p /home/%[1]s/.config/rclone

sudo -u %[1]s tee /home/%[1]s/.config/rclone/rclone.conf > /dev/null << EOF
[depot_%[1]s]
type = sftp
host = %[2]s
user = %[3]s
key_file = /home/%[1]s/.ssh/id_ed25519
shell_type = unix
EOF

sudo chown %[1]s:%[1]s /home/%[1]s/.config/rclone/rclone.conf
sudo chmod 600 /home/%[1]s/.config/rclone/rclone.conf
`, username, os.Getenv("IP_ADDRESS"), profname)
}

func rcloneSystemdCmd(username, poolname string) string {
	return fmt.Sprintf(`
set -e

SERVICE=/etc/systemd/system/rclone-depot-%[1]s.service

sudo tee "$SERVICE" > /dev/null << EOF
[Unit]
Description=Rclone depot mount for %[1]s
After=network-online.target
Wants=network-online.target

[Service]
User=%[1]s
ExecStart=/usr/bin/rclone mount depot_%[1]s:/home/%[1]s/depot/%[2]s /home/%[1]s/depot \
  --vfs-cache-mode writes \
  --log-file /home/%[1]s/.rclone_mount.log \
  --log-level INFO
ExecStop=/bin/fusermount -u /home/%[1]s/depot
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

sudo chmod 644 "$SERVICE"
sudo systemctl daemon-reload
sudo systemctl enable rclone-depot-%[1]s.service
sudo systemctl start rclone-depot-%[1]s.service
`, username, poolname)
}

// ENTRY POINT

func InstallRclone(server *models.Server, student *models.Student) error {
	username := sshinject.UsernameFromEmail(student.Name)

	var user models.User
	if err := config.Database.
		Where("email = ?", server.UserID).
		First(&user).Error; err != nil {
		return fmt.Errorf("fetch user failed: %w", err)
	}
	profname := sshinject.UsernameFromEmail(user.Email)

	signer, err := sshinject.LoadPrivateKey(os.Getenv("SSH_PRIVATE_KEY_PATH"))
	if err != nil {
		return err
	}

	config := sshinject.SshConfig("vmuser", signer)
	addr := fmt.Sprintf("%s:22", server.IP_Address)

	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return err
	}
	defer client.Close()

	// Create SSH key on remote VM
	cmd := ensureRemoteSSHKeyCmd(username)
	log.Println("ensureRemoteSSHKeyCmd")
	if err := sshinject.RunSSHcmd(client, cmd); err != nil {
		return fmt.Errorf("ensureRemoteSSHKeyCmd failed: %w", err)
	}

	// Read public key
	cmd = readRemotePubKeyCmd(username)
	log.Println("readRemotePubKeyCmd")
	pubkey, err := sshinject.RunSSHcmdWithOutput(client, cmd)
	if err != nil {
		return fmt.Errorf("readRemotePubKeyCmd failed: %w", err)
	}
	// log.Printf("Public key:\n%s", pubkey)

	// Create User on local
	runLocalCmd(createDepotUserCmdSecure(username))

	// Authorize public key on local
	if err := authorizeDepotKey(username, pubkey); err != nil {
		return fmt.Errorf("authorizeDepotKey failed: %w", err)
	}

	if err := runLocalCmd(createDepotUserCmdSecure(profname)); err != nil {
		return fmt.Errorf("createDepotUserCmdSecure failed: %w", err)
	}

	if err := runLocalCmd(ensureDepotPoolCmd(profname, server.ServerpoolID)); err != nil {
		return fmt.Errorf("ensureDepotPool failed: %w", err)
	}

	if err := runLocalCmd(ensureStudentFolderWithACLCmd(profname, server.ServerpoolID, username)); err != nil {
		return fmt.Errorf("ensureStudentFolderWithACLCmd failed: %w", err)
	}

	if err := runLocalCmd(linkPoolToStudentCmd(profname, server.ServerpoolID, username)); err != nil {
		return fmt.Errorf("linkPoolToStudentCmd failed: %w", err)
	}

	// Create mount point on remote VM
	cmd = ensureRemoteMountPointCmd(username)
	log.Println("ensureRemoteMountPointCmd")
	if err := sshinject.RunSSHcmd(client, cmd); err != nil {
		return fmt.Errorf("ensureRemoteMountPointCmd failed: %w", err)
	}

	// Create rclone config on remote VM
	cmd = rcloneConfigCmd(username, profname)
	log.Println("rcloneConfigCmd")
	if err := sshinject.RunSSHcmd(client, cmd); err != nil {
		return fmt.Errorf("rcloneConfigCmd failed: %w", err)
	}

	// Create systemd service on remote VM
	cmd = rcloneSystemdCmd(username, server.ServerpoolID)
	log.Println("rcloneSystemdCmd")
	if err := sshinject.RunSSHcmd(client, cmd); err != nil {
		return fmt.Errorf("rcloneSystemdCmd failed: %w", err)
	}

	return nil
}

func runLocalCmd(cmd string) error {
	log.Println("runLocalCmd: executing")

	c := exec.Command("bash", "-c", cmd)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr

	err := c.Run()

	log.Printf("runLocalCmd STDOUT:\n%s", stdout.String())
	log.Printf("runLocalCmd STDERR:\n%s", stderr.String())

	if err != nil {
		return fmt.Errorf("runLocalCmd failed: %w", err)
	}
	return nil
}
