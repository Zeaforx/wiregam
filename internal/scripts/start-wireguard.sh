#!/usr/bin/env bash
set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "This service step must run as root. SSM Run Command normally runs it as root."
  exit 1
fi

CONFIG_FILE="/etc/wireguard/wg0.conf"

if [[ ! -s "${CONFIG_FILE}" ]]; then
  echo "The WireGuard configuration is missing. Run the configuration step first."
  exit 1
fi

chmod 600 "${CONFIG_FILE}"
systemctl enable wg-quick@wg0

if systemctl is-active --quiet wg-quick@wg0; then
  systemctl restart wg-quick@wg0
else
  systemctl start wg-quick@wg0
fi

systemctl is-active --quiet wg-quick@wg0
wg show wg0
ss -H -uln | awk '$4 ~ /:51820$/ { found = 1 } END { exit !found }'

echo "WireGuard is active and listening on UDP port 51820."
printf 'WIREGAM_SERVER_PUBLIC_KEY=%s\n' "$(wg show wg0 public-key)"
