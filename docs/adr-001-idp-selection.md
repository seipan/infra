# ADR-001: 中央 IdP の選定

## Status

Proposed

## Context

現在、各アプリケーション (ArgoCD, MinIO, Grafana 等) に対して個別に認証設定を行う必要がある。Cloudflare Access による認証ゲートは存在するが、アプリケーション内でのユーザー識別 (OIDC/SAML) には対応できない。GitHub Login を統一的に使いたい。

## 候補

### Authelia

| 項目 | 内容 |
|------|------|
| リソース消費 | 極めて軽量。コンテナ ~20 MB、メモリ ~30 MB。Go 製 |
| 対応プロトコル | OIDC (OpenID Certified)、ForwardAuth |
| GitHub 連携 | 外部 OIDC プロバイダ経由で可能 |
| ストレージ | ファイルベース / SQLite / PostgreSQL / MySQL |
| 運用 | YAML 設定。Traefik ForwardAuth とネイティブ統合。Passkeys 対応 |
| コミュニティ | GitHub 22k+ stars。活発に開発中 |
| 制限事項 | SAML プロバイダ機能なし。GitHub 直接連携には外部 IdP が必要。ユーザー管理は YAML/LDAP ベース |

### DEX

| 項目 | 内容 |
|------|------|
| リソース消費 | 極めて軽量。~20-50 MB RAM。Go 製シングルバイナリ |
| 対応プロトコル | OIDC のみ |
| GitHub 連携 | First-class connector あり |
| ストレージ | Kubernetes CRDs / SQLite / PostgreSQL (refresh token 用) |
| 運用 | YAML 設定のみ。UI なし |
| コミュニティ | CNCF Sandbox。ArgoCD にバンドル済み |
| セキュリティ | 過去に Critical 脆弱性あり: 認可コード傍受 ([GHSA-vh7g-p26c-j2cw](https://github.com/dexidp/dex/security/advisories/GHSA-vh7g-p26c-j2cw))、SAML 署名検証バイパス ([GHSA-m9hp-7r99-94h5](https://github.com/dexidp/dex/security/advisories/GHSA-m9hp-7r99-94h5))、TLS フォールバック ([GHSA-gr79-9v6v-gc9r](https://github.com/dexidp/dex/security/advisories/GHSA-gr79-9v6v-gc9r))。いずれも修正済みだが、認証基盤として Critical が複数出ている点は留意 |
| 制限事項 | SAML プロバイダ機能なし。LDAP サーバ機能なし。ユーザー管理 UI なし。ID 連携ブローカーに特化 |

### Authentik

| 項目 | 内容 |
|------|------|
| リソース消費 | 重い。PostgreSQL + Redis 必須。合計 ~1.5-2 GB RAM |
| 対応プロトコル | OIDC, SAML, LDAP (outbound proxy), SCIM |
| GitHub 連携 | Built-in social source あり |
| ストレージ | PostgreSQL 必須 |
| 運用 | Web UI で設定可能。Helm chart あり |
| コミュニティ | セルフホスト界隈で人気。Python/Django 製 |
| 制限事項 | 個人インフラにはリソース過多。PostgreSQL + Redis の運用コスト |

### Keycloak

| 項目 | 内容 |
|------|------|
| リソース消費 | 重い。Java (Quarkus) 製。~500 MB-1.5 GB RAM + PostgreSQL |
| 対応プロトコル | OIDC, SAML 2.0, LDAP (federation), SCIM (extension) |
| GitHub 連携 | Built-in identity provider あり |
| ストレージ | PostgreSQL 必須 |
| 運用 | 強力な Admin Console。ただし学習コストが高い |
| コミュニティ | CNCF Incubating。Red Hat 支援。エンタープライズ実績多数 |
| 制限事項 | 個人用途にはオーバースペック。Java のメモリオーバーヘッド。設定が複雑 |

### Kanidm

| 項目 | 内容 |
|------|------|
| リソース消費 | 軽量。Rust 製。~50-100 MB RAM。組み込み DB |
| 対応プロトコル | OIDC, LDAP (read-only), RADIUS |
| GitHub 連携 | なし (2025年時点。OAuth2 upstream は実験的) |
| ストレージ | 組み込み DB (外部 DB 不要) |
| 運用 | 比較的新しくドキュメント少なめ |
| コミュニティ | 小規模だが成長中。Rust エコシステム |
| 制限事項 | SAML 非対応。GitHub ソーシャルログイン未対応。エコシステムが未成熟 |

### Zitadel

| 項目 | 内容 |
|------|------|
| リソース消費 | 中程度。Go 製。~200-400 MB RAM + PostgreSQL |
| 対応プロトコル | OIDC, SAML, LDAP (identity brokering 経由) |
| GitHub 連携 | Built-in social identity provider あり |
| ストレージ | CockroachDB (default) or PostgreSQL |
| 運用 | モダンな Web UI。API ファースト設計。Keycloak より簡潔 |
| コミュニティ | 企業支援あり。Keycloak より新しいが採用拡大中 |
| 制限事項 | DB 依存は避けられない。コミュニティリソースはまだ少なめ |

### Cloudflare Access JWT + Traefik ForwardAuth (既存インフラ活用)

| 項目 | 内容 |
|------|------|
| リソース消費 | ゼロ (クラスタ内に IdP 不要) |
| 対応プロトコル | 独自 JWT のみ (OIDC/SAML プロバイダではない) |
| GitHub 連携 | Cloudflare Access 側で設定済み |
| ストレージ | なし |
| 運用 | Traefik ForwardAuth middleware の設定のみ |
| コミュニティ | Cloudflare 公式 |
| 制限事項 | OIDC トークンエンドポイントを持たない。ArgoCD や Grafana 等 OIDC フローが必要なアプリには使えない。認証ゲートとしてのみ機能 |

## 比較サマリ

| | Authelia | DEX | Authentik | Keycloak | Kanidm | Zitadel | CF Access |
|---|---|---|---|---|---|---|---|
| RAM | ~30 MB | ~30 MB | ~1.5 GB | ~1 GB | ~80 MB | ~300 MB | 0 |
| 外部 DB | 不要 (SQLite) | 不要 (CRDs) | PostgreSQL + Redis | PostgreSQL | 不要 | PostgreSQL | 不要 |
| OIDC | o (Certified) | o | o | o | o | o | x |
| SAML | x | x | o | o | x | o | x |
| GitHub Login | △ (外部IdP経由) | o | o | o | x | o | o |
| 管理 UI | x (YAML) | x (YAML) | o | o | o | o | o (Cloudflare) |
| Traefik 統合 | ◎ (ForwardAuth) | o | o | o | o | o | o |
| セキュリティ実績 | 良好 | Critical CVE 複数 | 良好 | 良好 | 良好 | 良好 | 良好 |
| 個人インフラ向き | ◎ | o | △ | x | △ | o | ◎ (認証ゲートのみ) |

## Decision

**Authelia** を採用する。

理由:
- 極めて軽量 (~30 MB RAM) で個人インフラに適している
- OpenID Certified な OIDC プロバイダとして ArgoCD, MinIO, Grafana に対応可能
- Traefik ForwardAuth とネイティブ統合でき、既存構成との親和性が高い
- Passkeys / TOTP による MFA に対応
- セキュリティ実績が良好 (Critical CVE なし)
- GitHub Login は不要と判断。個人インフラのため、ユーザー名 + パスワード + MFA で十分

## Consequences

- ArgoCD, Grafana は ForwardAuth + プロキシ認証ヘッダー (`X-Forwarded-User`) で認証
- MinIO は Authelia を OIDC プロバイダとして設定
- ユーザー管理は YAML ファイルベース (個人利用のため)
- Traefik に ForwardAuth middleware の追加が必要
- Cloudflare Access との併用は可能 (外部からのアクセスゲート + 内部認証の二重防御)
