# Static Server

MinIOベースの静的ファイル配信サーバー

## 機能

- ホスト名からバケット名を自動抽出 (例: blog.yadon3141.com → blogバケット)
- index.htmlの自動配信
- .html拡張子の自動補完
- 各種静的ファイルのContent-Type対応
- インメモリキャッシュ (存在しないファイルの結果も記憶する)
- `/healthz` ヘルスチェックエンドポイント

## ヘルスチェック

`/healthz` は MinIO に一切アクセスせず、プロセスが生きていることだけを返します。
liveness プローブはここを向けてください。MinIO を触るプローブは、バケットが遅く
なっただけで再起動ループを招き、しかも全レプリカが同時に落ちます。

readiness プローブは `/` を向けたままで構いません。実際の配信経路を検証でき、
キャッシュが効いている間は MinIO へのアクセスも発生しません。

## ビルドとデプロイ

### 方法1: sudoを使用する場合
```bash
cd apps/static-server
./build-and-push-sudo.sh [TAG]
```

### 方法2: Rootless Containerdを使用する場合
```bash
# 初回のみ: Rootless Containerdのセットアップ
./setup-rootless-containerd.sh
source ~/.bashrc  # または ~/.zshrc

# ビルドとプッシュ
./build-and-push.sh [TAG]
```

### Kubernetesへのデプロイ
```bash
kubectl apply -f k8s/default/secret/minio-static-server.yaml
kubectl apply -k k8s/application/static-server/
```

## 環境変数

- `MINIO_ENDPOINT`: MinIOエンドポイント (デフォルト: minio.minio.svc.cluster.local:9000)
- `MINIO_ACCESS_KEY`: アクセスキー
- `MINIO_SECRET_KEY`: シークレットキー
- `MINIO_USE_SSL`: SSL使用フラグ (true/false)
- `PORT`: サーバーポート (デフォルト: 8080)
- `CACHE_TTL`: キャッシュしたファイルの保持時間 (デフォルト: 1m、`0` でキャッシュ無効)
- `CACHE_NEGATIVE_TTL`: 存在しないファイルを記憶しておく時間 (デフォルト: 10s、`0` で無効)
- `CACHE_MAX_BYTES`: キャッシュ全体の上限バイト数 (デフォルト: 16MiB)
- `CACHE_MAX_OBJECT_BYTES`: キャッシュ対象とするファイルの上限バイト数 (デフォルト: 1MiB、これを超えるものは都度ストリーミング)

## 必要なツール

- nerdctl: containerdを使用したコンテナビルド用
- kubectl: Kubernetesへのデプロイ用