#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# install-kube-vip-staticpod-local.sh
#
# Run this ON EACH prd control-plane node (prd-cp1 / prd-cp2 / prd-cp3).
# It generates and installs kube-vip static Pod manifest locally.
#
# Kubernetes v1.29+ (incl. v1.35) bootstrap-safe:
#   - Mounts /etc/kubernetes/super-admin.conf from host
#   - Keeps container mountPath as /etc/kubernetes/admin.conf
#
# Usage:
#   sudo ./install-kube-vip-staticpod-local.sh
#
# Notes:
#   - Run BEFORE kubeadm init/join.
#   - Adjust VIP and INTERFACE below.
# ============================================================

VIP="192.168.0.30"
PORT="6443"
INTERFACE="eth0"          # change if your NIC name differs (check: ip -br a)
KVVERSION="v0.8.2"        # pin kube-vip version
MANIFEST_DIR="/etc/kubernetes/manifests"
MANIFEST_PATH="${MANIFEST_DIR}/kube-vip.yaml"

log()  { echo -e "\033[1;32m[INFO]\033[0m $*"; }
warn() { echo -e "\033[1;33m[WARN]\033[0m $*"; }
err()  { echo -e "\033[1;31m[ERROR]\033[0m $*\033[0m" >&2; exit 1; }

[[ $EUID -eq 0 ]] || err "Run as root (sudo)."

command -v ctr >/dev/null 2>&1 || err "ctr not found. Install/enable containerd first."
command -v sed >/dev/null 2>&1 || err "sed not found (unexpected)."

log "=== kube-vip Static Pod install (local) ==="
log "VIP=${VIP}:${PORT}"
log "INTERFACE=${INTERFACE}"
log "KVVERSION=${KVVERSION}"

log "Creating manifest dir: ${MANIFEST_DIR}"
mkdir -p "${MANIFEST_DIR}"

log "Pulling kube-vip image..."
ctr image pull "ghcr.io/kube-vip/kube-vip:${KVVERSION}" >/dev/null

log "Generating kube-vip manifest (ARP + leaderElection + services)..."
ctr run --rm --net-host "ghcr.io/kube-vip/kube-vip:${KVVERSION}" vip /kube-vip manifest pod \
  --interface "${INTERFACE}" \
  --address "${VIP}" \
  --port "${PORT}" \
  --controlplane \
  --services \
  --arp \
  --leaderElection \
  | tee "${MANIFEST_PATH}" >/dev/null

log "Patching kubeconfig hostPath to use super-admin.conf (v1.29+ bootstrap safe)"
# Replace hostPath:
#   path: /etc/kubernetes/admin.conf
# -> path: /etc/kubernetes/super-admin.conf
sed -i 's|path: /etc/kubernetes/admin\.conf|path: /etc/kubernetes/super-admin.conf|g' "${MANIFEST_PATH}"

log "Setting ownership/permissions"
chown root:root "${MANIFEST_PATH}"
chmod 644 "${MANIFEST_PATH}"

log "Installed: ${MANIFEST_PATH}"
log "Sanity check (key lines):"
grep -nE 'vip_interface|address|port|super-admin\.conf|hostNetwork' "${MANIFEST_PATH}" || true

warn "No VIP will appear until kubeadm init/join starts the control-plane."
log "Done."
