#!/usr/bin/env bash
set -euo pipefail

VMID=9000
VMNAME="ubuntu-2204-template"
STORAGE_LVM="local-lvm"
BRIDGE="vmbr0"
CIUSER="ubuntu"
SSH_KEY_FILE="$HOME/.ssh/id_rsa.pub"
IMAGE_PATH="/var/lib/vz/template/iso/jammy-server-cloudimg-amd64.img"

echo "==> Create VM skeleton"
qm create ${VMID} \
  --name ${VMNAME} \
  --memory 1024 \
  --cores 1 \
  --cpu host \
  --net0 virtio,bridge=${BRIDGE} \
  --scsihw virtio-scsi-single \
  --machine q35 \
  --bios ovmf

echo "==> Import Ubuntu cloud image"
qm importdisk ${VMID} ${IMAGE_PATH} ${STORAGE_LVM}

echo "==> Attach imported disk as scsi0"
qm set ${VMID} \
  --scsi0 ${STORAGE_LVM}:vm-${VMID}-disk-0,discard=on,ssd=1

echo "==> Add Cloud-Init drive"
qm set ${VMID} --ide2 ${STORAGE_LVM}:cloudinit

echo "==> Set boot order"
qm set ${VMID} --boot order=scsi0

echo "==> Configure cloud-init (user / ssh / dhcp)"
qm set ${VMID} \
  --ciuser ${CIUSER} \
  --sshkeys ${SSH_KEY_FILE} \
  --ipconfig0 ip=dhcp

echo "==> Done. You can now start the VM:"
echo "    qm start ${VMID}"
echo "==> After verification, convert to template:"
echo "    qm template ${VMID}"
  \