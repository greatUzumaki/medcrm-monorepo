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
# cloud-init drops a PasswordAuthentication=yes override; remove it so nothing contradicts the hardening.
grep -l "PasswordAuthentication yes" /etc/ssh/sshd_config.d/*.conf 2>/dev/null | xargs -r rm -f
# Ubuntu 24.04+ runs ssh via socket activation: reload may find no active service, so don't let it abort.
sshd -t && systemctl try-reload-or-restart ssh

# Firewall: SSH (rate-limited) + HTTP/HTTPS (443/udp for HTTP/3).
ufw default deny incoming
ufw default allow outgoing
ufw limit OpenSSH
ufw allow 80/tcp
ufw allow 443
ufw --force enable

printf "[sshd]\nenabled = true\nbantime = 1h\nfindtime = 10m\nmaxretry = 5\n" > /etc/fail2ban/jail.d/sshd.local
systemctl enable --now fail2ban
systemctl restart fail2ban
printf 'APT::Periodic::Update-Package-Lists "1";\nAPT::Periodic::Unattended-Upgrade "1";\n' > /etc/apt/apt.conf.d/20auto-upgrades

echo "OK: server ready. Next: copy deploy/Caddyfile to /etc/caddy/Caddyfile and run apps/landing/scripts/deploy.sh"
