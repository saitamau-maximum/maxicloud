# MaxiCloud

サークル会員向けのPaaSです。

## Setup

[mise](https://mise.jdx.dev/) を使用して開発環境を構築します。

```bash
mise trust
mise run setup
```

[環境変数](./config/overlays/dev/README.md) を設定してからデプロイします。

```bash
mise run release
```

## Deployment

### バックエンド

変更を反映させるコマンドは `mise run release` です。個別に実行する場合は以下を参照してください。

```bash
# protoを変更したとき
mise run buf:generate

# CRDの定義やKubebuilderのマーカを変更したとき
mise run generate
mise run manifests

# Goのコードを変更したとき
mise run docker:build docker:push
mise run rollout
```

### ダッシュボード

ダッシュボードは`/dashboard`にあります。以下のコマンドで起動できます。

```bash
cd dashboard
pnpm install
pnpm dev
```

ダッシュボードは `http://localtest.me:5173` でアクセスしてください。

## GitHub Webhookの受け取り方

smee-client を使用してローカルに Webhook を転送します。

```bash
npm install --global smee-client
```

[smee.io](https://smee.io) にアクセスして Webhook を受け取る URL を作成し、以下を実行します。

```bash
smee --url <作成したURL> --target http://localtest.me:8080/github/webhook
```
