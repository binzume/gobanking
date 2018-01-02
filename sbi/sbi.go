package sbi

import (
	"html"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"

	"../common" // TODO
)

type Account struct {
	balance   int64
	client    *http.Client
	userAgent string
}

const BankCode = "0038"
const baseUrl = "https://www.netbk.co.jp/wpl/NBGate/"

type TempTransaction map[string]interface{}
type P map[string]string

var _ common.Account = &Account{}

func Login(id, password string) (*Account, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	a := &Account{
		client:    &http.Client{Jar: jar},
		userAgent: "Mozilla/5.0 NetBankingtClient/0.1",
	}
	err = a.Login(id, password, nil)
	return a, err
}

func (a *Account) Login(id, password string, loginParams interface{}) error {
	_, err := a.post("i010101CT", P{
		"userName":    id,
		"loginPwdSet": password,
		"x":           "0",
		"y":           "0",
	})
	if err != nil {
		return err
	}

	// top
	res, err := a.get("i020101CT/DI02010100")
	// log.Println(res)
	a.balance, _ = a.getMachedInt(res, `(?s)<strong>お預入れ合計<\/strong>.*?<strong>([\d,]+)\s*円<\/strong>`)
	return err
}

func (a *Account) Logout() error {
	_, err := a.get("i010001CT")
	return err
}

func (a *Account) TotalBalance() (int64, error) {
	return a.balance, nil
}

func (a *Account) Recent() ([]*common.Transaction, error) {
	return nil, nil
}

func (a *Account) History(from, to time.Time) ([]*common.Transaction, error) {
	return nil, nil
}

// transfar api
func (a *Account) NewTransactionWithNick(targetName string, amount int) (TempTransaction, error) {
	return nil, nil
}

func (a *Account) Commit(tr TempTransaction, pass2 string) (string, error) {
	return "dummy", nil
}

func (a *Account) post(path string, params P) (string, error) {
	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}
	log.Println("POST", path, params)

	req, err := http.NewRequest("POST", baseUrl+path, strings.NewReader(values.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", a.userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := a.client.Do(req)
	defer res.Body.Close()

	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	// TODO check error
	return string(b), err
}

func (a *Account) get(path string) (string, error) {
	log.Println("GET", path)

	req, err := http.NewRequest("GET", baseUrl+path, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", a.userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := a.client.Do(req)
	defer res.Body.Close()

	b, err := ioutil.ReadAll(transform.NewReader(res.Body, japanese.ShiftJIS.NewDecoder()))
	if err != nil {
		return "", err
	}
	// TODO check error
	return string(b), err
}

func (a *Account) getMachedInt(s, reStr string) (int64, error) {
	return strconv.ParseInt(strings.Replace(a.getMached(s, reStr, ""), ",", "", -1), 10, 64)
}

func (a *Account) getMached(s, reStr, def string) string {
	re := regexp.MustCompile(reStr)
	if m := re.FindStringSubmatch(s); m != nil {
		re := regexp.MustCompile(`<[^>]+>`)
		return strings.TrimSpace(html.UnescapeString(re.ReplaceAllString(m[1], "")))
	}
	return def
}
