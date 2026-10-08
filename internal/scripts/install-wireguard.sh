#!/usr/bin/env bash
set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "This installer must run as root. SSM Run Command normally runs it as root."
  exit 1
fi

dnf install -y wireguard-tools iptables

install -d -m 700 /etc/wireguard

if [[ ! -f /etc/wireguard/server_private.key ]]; then
  umask 077
  wg genkey | tee /etc/wireguard/server_private.key | wg pubkey > /etc/wireguard/server_public.key
  echo "WireGuard server keys created."
else
  echo "Existing server key found; leaving it unchanged."
fi

sysctl -w net.ipv4.ip_forward=1
install -d -m 755 /etc/sysctl.d
printf '%s\n' 'net.ipv4.ip_forward = 1' > /etc/sysctl.d/99-wireguard.conf
sysctl --system

INTERFACE="$(ip -4 route show default | awk '/default/ {print $5; exit}')"
if [[ -z "${INTERFACE}" ]]; then
  echo "Could not determine the internet-facing network interface."
  exit 1
fi

echo "Internet-facing network interface: ${INTERFACE}"
echo
echo "Next AWS step: disable Source/Destination Check for this EC2 instance."
