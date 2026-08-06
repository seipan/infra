# 外部PCから cloudflared 経由で kubectl を叩く

LAN外のPCから prd クラスタの kube-apiserver へ接続する手順。VPNやポート開放は不要で、既存の Cloudflare Tunnel を経由する。

## 仕組み

```
kubectl ──TLS──────────────────────────────────────────────┐
   │                                                        │
   └→ 127.0.0.1:6443 (cloudflared access tcp)               │
        └→ Cloudflare Edge (Access で認証)                  │
             └→ cloudflared Pod (k8s/cloudflared)           │
                  └→ kubernetes.default.svc:443 ────────────┘
```

- ingress ルールが `tcp://` なので cloudflared は生TCPをそのまま中継する。TLS は kubectl と apiserver の間で終端するため、**Cloudflare も cloudflared も認証情報を復号できない**
- オリジンは ClusterIP (`kubernetes.default.svc.cluster.local:443`) を指しているので、VIP `192.168.0.30:6443` へLAN経由で出る必要がない

## クラスタ側の設定

`k8s/cloudflared/config.yaml` の ingress に以下が入っている。変更は ArgoCD が自動 sync する。

```yaml
- hostname: k8s.yadon3141.com
  service: tcp://kubernetes.default.svc.cluster.local:443
```

DNS レコード（`k8s.yadon3141.com` → `<tunnel-id>.cfargotunnel.com` の CNAME）は cloudflared-dns-controller が configmap から生成する。作られていない場合は手動で:

```bash
cloudflared tunnel route dns 3bc95534-7c02-4257-af5a-6431b49728bd k8s.yadon3141.com
```

## クライアント側の設定

### 1. cloudflared のインストール

```bash
# macOS
brew install cloudflared
# Debian/Ubuntu
curl -fsSL https://pkg.cloudflare.com/cloudflare-main.gpg | sudo tee /usr/share/keyrings/cloudflare-main.gpg >/dev/null
echo 'deb [signed-by=/usr/share/keyrings/cloudflare-main.gpg] https://pkg.cloudflare.com/cloudflared any main' | sudo tee /etc/apt/sources.list.d/cloudflared.list
sudo apt update && sudo apt install cloudflared
```

### 2. トンネルの起動

```bash
cloudflared access tcp --hostname k8s.yadon3141.com --url 127.0.0.1:6443
```

初回はブラウザが開いて Access の認証（メールOTP等）が走り、トークンが `~/.cloudflared/` にキャッシュされる。kubectl を使っている間はこのプロセスを起動したままにする。

サービストークンを使う場合:

```bash
cloudflared access tcp --hostname k8s.yadon3141.com --url 127.0.0.1:6443 \
  --service-token-id "$CF_ACCESS_CLIENT_ID" \
  --service-token-secret "$CF_ACCESS_CLIENT_SECRET"
```

### 3. kubeconfig

`k8s/kubeadm/prd/cluster-config.yaml` で `certSANs` を指定していないため、apiserver 証明書の SAN は `kubernetes` / `kubernetes.default` / `kubernetes.default.svc` / `kubernetes.default.svc.cluster.local` / `192.168.0.30` / ノードIP のみ。`127.0.0.1` は含まれないので、`tls-server-name` で SNI/検証名を上書きする（証明書の作り直しは不要）。

```yaml
apiVersion: v1
kind: Config
clusters:
- name: prd-remote
  cluster:
    server: https://127.0.0.1:6443
    tls-server-name: kubernetes
    certificate-authority-data: <既存 kubeconfig の CA をそのままコピー>
contexts:
- name: prd-remote
  context:
    cluster: prd-remote
    user: prd-remote
current-context: prd-remote
users:
- name: prd-remote
  user:
    client-certificate-data: <後述>
    client-key-data: <後述>
```

### 4. 動作確認

```bash
kubectl --context prd-remote get nodes
```

## 認証情報について

admin の kubeconfig をそのまま持ち出すのは避け、外部PC用に権限を絞ったクレデンシャルを発行する。CSR を使う例:

```bash
openssl genrsa -out remote.key 2048
openssl req -new -key remote.key -out remote.csr -subj "/CN=remote-yamasaki/O=remote-users"
```

生成した CSR をクラスタ側で `CertificateSigningRequest` として approve し、`O=remote-users` に対して必要な範囲だけの ClusterRoleBinding を張る。

## トラブルシュート

| 症状 | 原因と対処 |
|---|---|
| `x509: certificate is valid for kubernetes, ... not 127.0.0.1` | kubeconfig の `tls-server-name: kubernetes` が抜けている |
| `websocket: bad handshake` / 401 | Access の認証が切れている。`cloudflared access login https://k8s.yadon3141.com` で取り直す |
| `connection refused` (127.0.0.1:6443) | `cloudflared access tcp` のプロセスが落ちている |
| DNS が引けない | CNAME レコードが未作成。`cloudflared tunnel route dns` を実行 |

## 関連

- 設定: `k8s/cloudflared/config.yaml`
- DNS コントローラ: `k8s/cloudflared-dns-controller/`
- クラスタ構成: [vm.md](./vm.md)
