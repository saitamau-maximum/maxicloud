# 環境変数の設定

## GitHub Appの作成

作成時には以下を権限を付与してください。

Permissions:
- Contents: read only
- Checks: read and write
- PullRequests: read and write

Subscribe to events:
- PullRequest
- Push

また、Callback URLには`http://localtest.me:8080/github/callback`を設定してください。

GitHub Appを作成したら、秘密鍵をこのディレクトリに`private-key.pem`という名前で保存してください。

また、環境変数の設定も行ってください。
```bash
cp config.example.env config.env
cp secret.example.env secret.env
```

`secret.env` の `PACK_VOLUME_KEY` には、Buildpacks の volume cache を安定して利用するための秘密値を設定してください。
例えば以下のコマンドで生成できます。
```bash
PACK_VOLUME_KEY=$(openssl rand -hex 32)
```

生成した値を `secret.env` に書き込みます。
```env
PACK_VOLUME_KEY=<生成した値>
```
