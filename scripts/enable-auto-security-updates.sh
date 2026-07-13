#!/usr/bin/env bash
# enable-auto-security-updates.sh
# Active les mises a jour de SECURITE automatiques sur une VM Ubuntu (unattended-upgrades).
# Securite uniquement (pas les montees de version), idempotent, ASCII seulement.
#
# Usage :
#   sudo ./enable-auto-security-updates.sh                 # reboot auto a 04:00 (VMs de service)
#   sudo ./enable-auto-security-updates.sh --no-reboot     # sans reboot auto (ex: noeud Postgres)
#   sudo ./enable-auto-security-updates.sh --time=03:30    # fenetre de reboot personnalisee
#
# Ne remplace PAS le durcissement reseau (security group) : c'est une couche d'hygiene
# complementaire, pas le correctif du vecteur d'intrusion (service expose sans auth).

set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then
  echo "A lancer en root (sudo)." >&2
  exit 1
fi

REBOOT="true"
TIME="04:00"
for arg in "$@"; do
  case "$arg" in
    --no-reboot) REBOOT="false" ;;
    --time=*)    TIME="${arg#--time=}" ;;
    *) echo "argument inconnu: $arg" >&2; exit 1 ;;
  esac
done

export DEBIAN_FRONTEND=noninteractive
echo "[1/4] Installation d'unattended-upgrades..."
apt-get update -qq
apt-get install -y -qq unattended-upgrades apt-listchanges

echo "[2/4] Configuration (securite uniquement, reboot=$REBOOT a $TIME)..."
# Drop-in dedie : n'ecrase pas le fichier distributeur. Par defaut, unattended-upgrades
# n'autorise deja QUE l'origine -security ; on active le periodique + la politique de reboot.
cat > /etc/apt/apt.conf.d/52security-autoupdate <<CONF
// Gere par scripts/enable-auto-security-updates.sh -- MAJ de securite uniquement.
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Download-Upgradeable-Packages "1";
APT::Periodic::Unattended-Upgrade "1";
APT::Periodic::AutocleanInterval "7";
Unattended-Upgrade::Automatic-Reboot "$REBOOT";
Unattended-Upgrade::Automatic-Reboot-Time "$TIME";
Unattended-Upgrade::Remove-Unused-Kernel-Packages "true";
Unattended-Upgrade::Remove-Unused-Dependencies "true";
CONF

echo "[3/4] Activation du service..."
systemctl enable --now unattended-upgrades >/dev/null 2>&1 || true
systemctl enable --now apt-daily.timer apt-daily-upgrade.timer >/dev/null 2>&1 || true

echo "[4/4] Verification a blanc (dry-run) :"
unattended-upgrades --dry-run --debug 2>&1 | grep -iE "Allowed origins|Checking|packages that|upgraded|No packages" | head -12 || true

echo
echo "[ok] Mises a jour de securite automatiques activees."
echo "     reboot auto: $REBOOT (a $TIME)  |  origine: ...-security uniquement"
echo "     journaux: /var/log/unattended-upgrades/"
