

適当に.goファイルを置いてあるだけなので go run で実行してください．

``` console
go run account_info.go accounts/rakuten.json
```


# サンプル内で使われているjson

## みずほ銀行

``` json
{
  "bank": "mizuho",
  "id":"1234567890",
  "password": "PASSWORD",
  "options": {
		"質問の部分文字列1": "答え1",
		"質問の部分文字列2": "答え2"
	}
}
```

## 楽天
``` json
{
  "bank": "rakuten",
  "id":"username",
  "password": "PASSWORD",
  "options": {
		"質問の部分文字列1": "答え1",
		"質問の部分文字列2": "答え2"
	}
}
```

## 新生銀行

``` json
{
  "bank": "shinsei",
  "id":"4001234567",
  "password": "PASSWORD",
  "options": {"grid":["ABCDEF1234", "ABCDEF1234", "ABCDEF1234", "ABCDEF1234", "ABCDEF1234"]}
}
```

## SBI
``` json
{
  "bank": "sbi",
  "id":"username",
  "password": "PASSWORD",
  "options": {}
}
```
