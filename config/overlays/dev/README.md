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
