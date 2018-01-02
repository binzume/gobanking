# Internet Banking fo Golang

以前からrubyで書いてたものをGolangに移植中.

- Pure Golang
- No JS Engine

## 実装状態

よく使う銀行から実装しています．

| 銀行     | 残高 | 履歴(Recent) | 履歴  | 送金 |
|----------|------|--------------|-------|------|
| みずほ   | ok   | ok           | ok    | ok   |
| 新生銀行 | ok   | ok           | ok    | ok   |
| 楽天銀行 | ok   | ok           | ok    | ok?  |
| 住信SBI  | ok   | TODO         |       |      |

### memo

- 二要素認証はできません
- 送金先は登録済み口座のみ
- みずほで取得可能な履歴は過去3ヶ月
- SBIは色々TODO

### Usage

T.B.D.

みずほダイレクトの場合.(楽天銀行もほぼ同じ．新生銀行はログイン時に全情報を渡す必要があります)

```golang
	words := map[string]string{
		"質問の部分文字列": "答え",
	}
	acc, _ := mizuho.Login("1234567890", "password", words)
	log.Println(acc.TotalBalance())
	_ = acc.Logout()
```

## TODO

- パッケージ名
- 証券口座の操作
- ドキュメント

# 注意

- このライブラリの挙動について，何の保証もできません
- 最悪，口座を凍結されたりお金がなくなっても，まぁいいか，と思える範囲で使ってください
- (実際，激しく使ってると，アカウントロックされる銀行があります...)
