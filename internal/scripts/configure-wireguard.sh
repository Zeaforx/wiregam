#!/usr/bin/env bash
set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "This configuration step must run as root. SSM Run Command normally runs it as root."
  exit 1
fi

PRIVATE_KEY_FILE="/etc/wireguard/server_private.key"
CONFIG_FILE="/etc/wireguard/wg0.conf"

if [[ ! -s "${PRIVATE_KEY_FILE}" ]]; then
  echo "The WireGuard server private key is missing. Run the install step first."
  exit 1
fi

INTERFACE="$(ip -4 route show default | awk '/default/ {print $5; exit}')"
if [[ -z "${INTERFACE}" ]]; then
  echo "Could not determine the internet-facing network interface."
  exit 1
fi

PRIVATE_KEY="$(<"${PRIVATE_KEY_FILE}")"
if [[ -z "${PRIVATE_KEY}" ]]; then
  echo "The WireGuard server private key is empty."
  exit 1
fi

umask 077
TEMP_CONFIG="$(mktemp /etc/wireguard/wg0.conf.XXXXXX)"
trap 'rm -f "${TEMP_CONFIG}"' EXIT

cat > "${TEMP_CONFIG}" <<EOF
[Interface]
Address = 10.8.0.1/24
ListenPort = 51820
PrivateKey = ${PRIVATE_KEY}

PostUp = iptables -A FORWARD -i wg0 -j ACCEPT; iptables -A FORWARD -o wg0 -j ACCEPT; iptables -t nat -A POSTROUTING -o ${INTERFACE} -j MASQUERADE
PostDown = iptables -D FORWARD -i wg0 -j ACCEPT; iptables -D FORWARD -o wg0 -j ACCEPT; iptables -t nat -D POSTROUTING -o ${INTERFACE} -j MASQUERADE

[Peer]
PublicKey = {{CLIENT_PUBLIC_KEY}}
AllowedIPs = 10.8.0.2/32
EOF

chmod 600 "${TEMP_CONFIG}"
mv "${TEMP_CONFIG}" "${CONFIG_FILE}"
trap - EXIT

wg-quick strip "${CONFIG_FILE}" > /dev/null

echo "Created ${CONFIG_FILE} using outbound interface ${INTERFACE}."
