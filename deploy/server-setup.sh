#!/usr/bin/env bash
# One-time bootstrap of a fresh Ubuntu 24.04+/Debian 12+ VPS. Run as root:
#   ssh root@IP 'bash -s' < deploy/server-setup.sh
set -euo pipefail

DEPLOY_USER=deploy

apt-get update -qq
DEBIAN_FRONTEND=noninteractive apt-get upgrade -y -qq
DEBIAN_FRONTEND=noninteractive apt-get install -y -qq ufw fail2ban unattended-upgrades rsync curl gnupg debian-keyring debian-archive-keyring apt-transport-https

# Caddy from the official repo.
if ! command -v caddy >/dev/null; then
  curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
  curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' > /etc/apt/sources.list.d/caddy-stable.list
  apt-get update -qq && apt-get install -y -qq caddy
fi

# Deploy user: owns /srv/landing, key-only login, no sudo.
id "$DEPLOY_USER" >/dev/null 2>&1 || adduser --disabled-password --gecos "" "$DEPLOY_USER"
install -d -m 700 -o "$DEPLOY_USER" -g "$DEPLOY_USER" /home/$DEPLOY_USER/.ssh
install -m 600 -o "$DEPLOY_USER" -g "$DEPLOY_USER" /root/.ssh/authorized_keys /home/$DEPLOY_USER/.ssh/authorized_keys
install -d -o "$DEPLOY_USER" -g "$DEPLOY_USER" /srv/landing
install -d -o caddy -g caddy /var/log/caddy

# SSH: keys only.
cat > /etc/ssh/sshd_config.d/10-hardening.conf <<'CONF'
PasswordAuthentication no
KbdInteractiveAuthentication no
PermitRootLogin prohibit-password
MaxAuthTries 3
CONF
sshd -t && systemctl reload ssh || systemctl reload sshd

# Firewall: SSH + HTTP/HTTPS (443/udp for HTTP/3).
ufw default deny incoming
ufw default allow outgoing
ufw allow OpenSSH
ufw allow 80/tcp
ufw allow 443
ufw --force enable

systemctl enable --now fail2ban
dpkg-reconfigure -f noninteractive unattended-upgrades

echo "OK: server ready. Next: copy deploy/Caddyfile to /etc/caddy/Caddyfile and run apps/landing/scripts/deploy.sh"
