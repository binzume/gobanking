# Internet Banking fo Golang

[![Build Status](https://travis-ci.org/binzume/gobanking.svg)](https://travis-ci.org/binzume/gobanking)
[![codecov](https://codecov.io/gh/binzume/gobanking/branch/master/graph/badge.svg)](https://codecov.io/gh/binzume/gobanking)
[![GoDoc](https://godoc.org/github.com/binzume/gobanking?status.svg)](https://godoc.org/github.com/binzume/gobanking)

Golangで銀行のサイトをスクレイピングして操作するためのライブラリです．
昔Rubyで書いたものをGolangに移植したもの．

- Pure Golang
- No JS Engine


## 実装状態

よく使う銀行から実装しています．

| 銀行     | 残高 | 履歴(Recent) | 履歴  | 送金 |
|----------|------|--------------|-------|------|
| みずほ   | ok   | ok           | ok    | ok   |
| 新生銀行 | ok   | ok           | ok    | ok   |
| 楽天銀行 | ok   | ok           | ok    | ok   |
| 住信SBI  | ok   | TODO         | TODO  |      |

### memo

- 二要素認証はできません
- 送金先は登録済み口座のみ
- SBIは色々TODO
- [stub](stub) はそれっぽい値を返すダミー実装(テスト用)

## Usage

T.B.D.

とりあえず，共通の操作は [common.Account](common/common.go) のインターフェイスを見れば分かるかもしれません．

### ログイン

[jsonファイル](examples/README.md)に書かれたアカウント情報を使う場合．

```go
import "github.com/binzume/gobanking"

func main() {
	acc, err := banking.LoginWithJsonFile("account/mizuho.json")
	if err != nil {
		log.Fatal(err)
	}
	defer acc.Logout()
	// ...
```

または，個々のパッケージのLoginを直接呼んでも問題ありません．(みずほダイレクトの場合).
ネットバンキングサイトによってログイン時の引数やインターフェイスが多少変わります．
(みずほ銀行と楽天銀行はほぼ同じ．新生銀行はログイン時に全情報を渡す必要があります)

```go
import "github.com/binzume/gobanking/mizuho"

func main() {
	words := map[string]interface{}{
		"質問の部分文字列": "答え",
	}
	acc, err := mizuho.Login("1234567890", "password", words)
	if err != nil {
		log.Fatal(err)
	}
	defer acc.Logout()
	// ...
```


### 残高取得

```golang
	total, err := acc.TotalBalance()
```

### 取引履歴

直近の数件を返すものと，期間指定で取得する関数があります．取得可能な件数や期間は銀行によって異なります．

```golang
	recent, err := acc.Recent()
	history, err := History(time.Now().Add(-time.Hour*24*30), time.Now())
```

- みずほ：取得可能な履歴は過去3ヶ月
- 楽天銀行：24ヶ月，3000件まで
- Recent() は日時の古いものがスライスの先頭です

### 送金

振込先として登録済みの口座に対してのみ振り込めます．

`NewTransferToRegisteredAccount()` で登録済みの口座への振込情報を作成し，`CommitTransfer()` で確定．


```golang
	tr, err := acc.NewTransferToRegisteredAccount("binzume", 5000000000000000)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(tr)

	recptNo, err := acc.CommitTransfer(tr, "123456(暗証番号等)")
	log.Println(recptNo, err)
```

- 新生銀行は 振込先名のところに口座番号．他の銀行は振込先として登録した名前を指定してください．

## TODO

- 証券口座の操作
- ドキュメント

# 注意

- このライブラリの挙動について，何の保証もできません
- 最悪，口座を凍結されたりお金がなくなっても，まぁいいか，と思える範囲で使ってください
- (実際，激しく使ってると，アカウントロックされる可能性があります...)
