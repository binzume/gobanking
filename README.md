# Internet Banking fo Golang

銀行のサイトをスクレイピングして操作するためのライブラリです．

まだ実装途中です.以前からrubyで書いてたものをGolangに移植中. 

- Pure Golang
- No JS Engine


## 実装状態

よく使う銀行から実装しています．

| 銀行     | 残高 | 履歴(Recent) | 履歴  | 送金 |
|----------|------|--------------|-------|------|
| みずほ   | ok   | ok           | ok    | ok   |
| 新生銀行 | ok   | ok           | ok    | ok   |
| 楽天銀行 | ok   | ok           | ok    | ok?  |
| 住信SBI  | ok   | TODO         | TODO  |      |

### memo

- 二要素認証はできません
- 送金先は登録済み口座のみ
- SBIは色々TODO

## Usage

T.B.D.

とりあえず，共通の操作は common.Account のインターフェイスを見れば分かるかもしれません．

### ログイン

みずほダイレクトの場合.
ネットバンキングサイトによって引数が多少変わります．
(みずほ銀行と楽天銀行はほぼ同じ．新生銀行はログイン時に全情報を渡す必要があります)

```golang
	words := map[string]string{
		"質問の部分文字列": "答え",
	}
	acc, err := mizuho.Login("1234567890", "password", words)
	if err != nil {
		log.Fatal(err)
	}
	defer acc.Logout()
```

### 残高取得

```golang
	total, err := acc.TotalBalance()
```

### 取引履歴

直近の数件を返すものと，期間指定で取得する関数があります．件数や指定可能な期間は銀行によって異なります．

```golang
	recent, err := acc.Recent()
	history, err := History(time.Now().Add(-time.Hour*24*30), time.Now())
```

- みずほで取得可能な履歴は過去3ヶ月
- 楽天銀行は24ヶ月，3000件まで


### 送金

登録済みの口座に対してのみ振り込めます．
一応動きますが，インターフェイスとか色々整理中です．

```golang
	tr, err := acc.NewTransactionWithNick("登録済み振込先名", 10000)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(tr)

	recptNo, err := acc.Commit(tr, "暗証番号等")
	log.Println(recptNo, err)
```

- 楽天銀行は 振込先名 ではなく 登録順のインデックス(後で直します)
- 新生銀行は 振込先名のところに口座番号． 暗証番号は空で良い．

## TODO

- パッケージ名
- 証券口座の操作
- ドキュメント

# 注意

- このライブラリの挙動について，何の保証もできません
- 最悪，口座を凍結されたりお金がなくなっても，まぁいいか，と思える範囲で使ってください
- (実際，激しく使ってると，アカウントロックされる銀行があります...)
