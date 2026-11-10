# mdv

Markdown を mermaid ごと HTML に変換してブラウザで開く CLI。サーバは立てない。

```
mdv notes.md
```

- 生成物は `~/Library/Caches/mdv/<hash>-<name>.md.html`（同じファイルなら上書き）
- 7日より古いキャッシュは自動削除
- 相対パス画像は `<base href>` で md のディレクトリ基準に解決
- mermaid のパースエラーはそのブロックだけ赤枠で表示し、他は描画を続ける

## インストール

```
mise use -g github:yumazak/mdv
```

タグを打った直後は mise の `minimum_release_age`（既定24時間）で新リリースが隠れる。すぐ入れたいときは一度だけ:

```
mise use -g --minimum-release-age 0s github:yumazak/mdv
```

ソースから:

```
make build && make install
```

`vendor/` はバイナリと同じディレクトリに必要。移動する場合は `-vendor` で指定する。

## フラグ

| フラグ | 説明 |
|---|---|
| `-o PATH` | 出力先を指定（既定はキャッシュ） |
| `-no-open` | ブラウザを開かない（パスだけ出力） |
| `-theme auto\|dark\|light` | mermaid のテーマ |
| `-title TEXT` | タイトルを上書き |
| `-clean` | キャッシュを全削除 |
| `-keep-days N` | キャッシュの保持日数（既定7、0で無効） |
| `-vendor DIR` | vendor ディレクトリ |

## 自動更新

ブラウザの再読み込みで再生成後の内容を読める。ファイル監視をしたい場合だけ前景で回す。

```
fswatch -o notes.md | xargs -n1 mdv -no-open notes.md
```

## 仕組み

```
md --base64--> template.html に埋め込み
              marked で md -> HTML（ブラウザ側）
              ```mermaid を mermaid.render() で SVG 化
```

vendor は絶対 `file://` URL で参照するので、生成物はキャッシュに置いても動く。
HTML 自体は小さい（md の base64 のみ）。

## リリース

`v*` タグを push すると GitHub Actions が 4 プラットフォーム分の
`mdv-<version>-<os>-<arch>.tar.gz`（`mdv` + `vendor/`）を作り、
checksums.txt と attestation を付けてリリースする。

リポジトリが private の場合、mise から見えるトークンが必要:

```toml
# ~/.config/mise/config.toml
[settings.github]
credential_command = 'gh auth token --hostname "$MISE_CREDENTIAL_HOST"'
```

## 既知の制限

- Safari では `file://` からの vendor 読み込みがブロックされる可能性がある（未確認）。その場合は `-o ./preview.html` で md と同じディレクトリに出すか、vendor を HTML にインライン化する
- 目次・シンタックスハイライト・図の PNG 書き出しは未実装
