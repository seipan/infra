# VM構成
## 1. 物理ホスト一覧（Proxmox）

| ホスト名 | IP | RAM | 備考 |
|---|---|---:|---|
| PC1 (prox1) | 192.168.0.11 | 16GB | 追加SSDあり（local-lvm-2 512GB） |
| PC2 (prox2) | 192.168.0.12 | 16GB | local-lvm 370GB |
| PC3 (prox3) | 192.168.0.13 | 16GB | local-lvm 370GB |

---

## 2. VM 全体構成

- **mgmt クラスタ**：2 VM  
- **prd クラスタ**  
  - control-plane：3 VM  
  - worker（PC）：2 VM  
  - worker（Raspberry Pi）：2 台  
- **stg クラスタ**：1 VM  

---

## 3. mgmt クラスタ（管理系）

| VM名 | 配置 | IP | vCPU | RAM | ディスク構成 |
|---|---|---|---:|---:|---|
| mgmt-1 | PC1 | 192.168.0.21 | 2 | 3GB | OS 40GB（local-lvm-2）<br>MinIO 350GB（local-lvm-2） |
| mgmt-2 | PC2 | 192.168.0.22 | 2 | 4GB | OS 40GB（local-lvm） |


---

## 4. prd クラスタ（本番）

### 4.1 control-plane

| VM名 | 配置 | IP | vCPU | RAM | ディスク構成 |
|---|---|---|---:|---:|---|
| prd-cp1 | PC1 | 192.168.0.31 | 2 | 3GB | OS 30GB（local-lvm-2）<br>etcd 40GB（local-lvm） |
| prd-cp2 | PC2 | 192.168.0.32 | 2 | 4GB | OS 30GB（local-lvm）<br>etcd 40GB（local-lvm） |
| prd-cp3 | PC3 | 192.168.0.33 | 2 | 4GB | OS 30GB（local-lvm）<br>etcd 40GB（local-lvm） |
 

---

### 4.2 worker ノード

| Node名 | 実体 | 配置 | IP | vCPU | RAM | ディスク構成 | Longhorn |
|---|---|---|---|---:|---:|---|---|
| prd-w1 | VM | PC1 | 192.168.0.41 | 4 | 8GB | OS 30GB（local-lvm-2）<br>data 250GB（local-lvm） | 有効 |
| prd-w2 | VM | PC3 | 192.168.0.44 | 4 | 8GB | OS 30GB（local-lvm）<br>data 200GB（local-lvm） | 有効 |
| prd-pi1 | RasPi5 | 物理 | 192.168.0.42 | 4 | 8GB | SSD 256GB | 原則無効 |
| prd-pi2 | RasPi5 | 物理 | 192.168.0.43 | 4 | 8GB | SSD 256GB | 原則無効 |

---

## 5. stg クラスタ（検証）

| VM名 | 配置 | IP | vCPU | RAM | ディスク構成 |
|---|---|---|---:|---:|---|
| stg-1 | PC2 | 192.168.0.51 | 2 | 4GB | OS 60GB（local-lvm） |

---