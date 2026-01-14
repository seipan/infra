#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# kubeadm / kubelet / kubectl install script (Ubuntu 22.04+)
# - Adds Kubernetes apt repo (pkgs.k8s.io)
# - Installs kubelet/kubeadm/kubectl
# - Pins (hold) versions
# - Optionally sets kubelet --node-ip
#
# Usage:
#   sudo ./install-k8s.sh --minor 1.35 --node-ip 192.168.0.31
#
# Examples (prd):
#   cp1: sudo ./install-k8s.sh --minor 1.35 --node-ip 192.168.0.31
#   cp2: sudo ./install-k8s.sh --minor 1.35 --node-ip 192.168.0.32
#   cp3: sudo ./install-k8s.sh --minor 1.35 --node-ip 192.168.0.33
#   w1 : sudo ./install-k8s.sh --minor 1.35 --node-ip 192.168.0.41
#   w2 : sudo ./install-k8s.sh --minor 1.35 --node-ip 192.168.0.44
# ============================================================

log()  { echo -e "\033[1;32m[INFO]\033[0m $*"; }
warn() { echo -e "\033[1;33m[WARN]\033[0m $*"; }
err()  { echo -e "\033[1;31m[ERROR]\033[0m $*\033[0m" >&2; exit 1; }

MINOR=""
NODE_IP=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --minor)
      MINOR="${2:-}"; shift 2;;
    --node-ip)
      NODE_IP="${2:-}"; shift 2;;
    -h|--help)
      sed -n '1,120p' "$0"; exit 0;;
    *)
      err "Unknown arg: $1 (use --help)";;
  esac
done

[[ $EUID -eq 0 ]] || err "Run as root (sudo)."
[[ -n "$MINOR" ]] || err "--minor is required (e.g. 1.35)"
[[ "$MINOR" =~ ^1\.[0-9]+$ ]] || err "--minor must look like 1.xx (e.g. 1.29, 1.35)"
if [[ -n "$NODE_IP" ]]; then
  [[ "$NODE_IP" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]] || err "--node-ip must be an IPv4 address"
fi

log "Installing Kubernetes packages for minor version: v${MINOR}"
log "Node IP: ${NODE_IP:-<not set>} (kubelet extra args)"

# --- prerequisites
log "Installing prerequisites..."
apt-get update -y
apt-get install -y apt-transport-https ca-certificates curl gpg

# --- keyring
log "Setting up apt keyring..."
mkdir -p -m 755 /etc/apt/keyrings
curl -fsSL "https://pkgs.k8s.io/core:/stable:/v${MINOR}/deb/Release.key" \
  | gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg

# --- repo
log "Adding Kubernetes apt repo (pkgs.k8s.io)..."
cat > /etc/apt/sources.list.d/kubernetes.list <<EOF
deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v${MINOR}/deb/ /
EOF

# --- install
log "Installing kubelet, kubeadm, kubectl..."
apt-get update -y
apt-get install -y kubelet kubeadm kubectl

log "Holding kubelet/kubeadm/kubectl (prevent unintended upgrades)..."
apt-mark hold kubelet kubeadm kubectl >/dev/null

# --- kubelet node-ip (recommended for fixed-IP LAN)
if [[ -n "$NODE_IP" ]]; then
  log "Configuring kubelet to use --node-ip=${NODE_IP} ..."
  cat > /etc/default/kubelet <<EOF
KUBELET_EXTRA_ARGS=--node-ip=${NODE_IP}
EOF
else
  warn "Skipping kubelet --node-ip (set with --node-ip if you want to pin InternalIP)."
fi

log "Enabling kubelet service..."
systemctl daemon-reload
systemctl enable --now kubelet

log "Done."
log "Versions:"
kubeadm version || true
kubelet --version || true
kubectl version --client || true

log "Note: kubelet may show errors until you run kubeadm init/join. That's normal."
