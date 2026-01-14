#!/bin/bash
set -euo pipefail

# バージョン設定
CONTAINERD_VERSION="2.2.1"
CRICTL_VERSION="v1.34.0"
ARCH="amd64"

log() { echo -e "\033[1;32m[INFO]\033[0m $1"; }
warn() { echo -e "\033[1;33m[WARN]\033[0m $1"; }
err() { echo -e "\033[1;31m[ERROR]\033[0m $1" >&2; exit 1; }

# root権限チェック
[[ $EUID -eq 0 ]] || err "このスクリプトはroot権限で実行してください"

# 一時ディレクトリ
WORKDIR=$(mktemp -d)
trap "rm -rf $WORKDIR" EXIT
cd "$WORKDIR"

log "=== Kubernetes Master Node セットアップ開始 ==="

## swap off
log "Swapを無効化..."
swapoff -a
sed -i '/ swap / s/^/#/' /etc/fstab

## カーネルモジュール設定
log "カーネルモジュールを設定..."
cat <<EOF | tee /etc/modules-load.d/k8s.conf
overlay
br_netfilter
EOF

modprobe overlay
modprobe br_netfilter

cat <<EOF | tee /etc/sysctl.d/k8s.conf
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
EOF

sysctl --system

## runc
if ! command -v runc &>/dev/null; then
    log "runcをビルド・インストール..."
    apt-get update && apt-get install -y make gcc libseccomp-dev
    git clone --depth 1 https://github.com/opencontainers/runc
    cd runc
    make
    make install
    cd ..
else
    warn "runcは既にインストール済み: $(runc --version | head -1)"
fi

## containerd
if ! command -v containerd &>/dev/null; then
    log "containerd v${CONTAINERD_VERSION} をインストール..."
    wget -q "https://github.com/containerd/containerd/releases/download/v${CONTAINERD_VERSION}/containerd-${CONTAINERD_VERSION}-linux-${ARCH}.tar.gz"
    tar Cxzf /usr/local "containerd-${CONTAINERD_VERSION}-linux-${ARCH}.tar.gz"
else
    warn "containerdは既にインストール済み: $(containerd --version)"
fi

# containerd設定
mkdir -p /etc/containerd
containerd config default | tee /etc/containerd/config.toml > /dev/null
sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml

# systemdサービス設定
if [[ ! -f /etc/systemd/system/containerd.service ]]; then
    log "containerd systemdサービスを設定..."
    cat <<EOF | tee /etc/systemd/system/containerd.service
[Unit]
Description=containerd container runtime
Documentation=https://containerd.io
After=network.target local-fs.target

[Service]
ExecStart=/usr/local/bin/containerd
Restart=always
RestartSec=5
Delegate=yes
KillMode=process
OOMScoreAdjust=-999
LimitNOFILE=1048576
LimitNPROC=infinity
LimitCORE=infinity

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl enable --now containerd
fi

## cri-tools
if ! command -v crictl &>/dev/null; then
    log "crictl ${CRICTL_VERSION} をインストール..."
    wget -q "https://github.com/kubernetes-sigs/cri-tools/releases/download/${CRICTL_VERSION}/crictl-${CRICTL_VERSION}-linux-${ARCH}.tar.gz"
    tar zxf "crictl-${CRICTL_VERSION}-linux-${ARCH}.tar.gz" -C /usr/local/bin
else
    warn "crictlは既にインストール済み: $(crictl --version)"
fi

# crictl設定
cat <<EOF | tee /etc/crictl.yaml
runtime-endpoint: unix:///run/containerd/containerd.sock
image-endpoint: unix:///run/containerd/containerd.sock
timeout: 10
EOF

## etcd disk mount
if [[ -b /dev/sdb ]] && ! grep -q '/var/lib/etcd' /etc/fstab; then
    log "etcd用ディスク /dev/sdb をマウント..."
    mkfs.ext4 -F /dev/sdb
    mkdir -p /var/lib/etcd
    echo "/dev/sdb /var/lib/etcd ext4 defaults 0 2" >> /etc/fstab
    mount -a
elif [[ ! -b /dev/sdb ]]; then
    warn "/dev/sdb が見つかりません。etcdディスクのマウントをスキップ"
else
    warn "etcdディスクは既にfstabに設定済み"
fi

log "=== セットアップ完了 ==="
log "次のステップ: kubeadm init を実行してください"
