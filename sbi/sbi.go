package sbi

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"

	"github.com/binzume/gobanking/common"
)

type Account struct {
	common.BankAccount
	balance   int64
	client    *http.Client
	lastLogin time.Time
}

const BankCode = "0038"
const BankName = "住信SBIネット銀行"
const baseUrl = "https://www.netbk.co.jp/wpl/NBGate/"

type P map[string]string

var _ common.Account = &Account{}

func Login(id, password string) (*Account, error) {
	client, err := common.NewHttpClient()
	if err != nil {
		return nil, err
	}
	a := &Account{client: client}
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
	a.balance, _ = getMatchedInt(res, `(?s)<strong>お預入れ合計<\/strong>.*?<strong>([\d,]+)\s*円<\/strong>`)

	// account infos
	// res, err := a.get("i020401CT")

	a.BankCode = BankCode
	a.BankName = BankName

	return err
}

func (a *Account) Logout() error {
	_, err := a.get("i010001CT")
	return err
}

func (a *Account) TotalBalance() (int64, error) {
	return a.balance, nil
}

func (a *Account) LastLogin() (time.Time, error) {
	return a.lastLogin, nil
}

func (a *Account) Recent() ([]*common.Transaction, error) {
	return nil, nil
}

func (a *Account) History(from, to time.Time) ([]*common.Transaction, error) {
	// TODO
	/*
		res, err := a.get("i020201CT/PD/01/01/001/01")
		// <form method="post" action="/wpl/NBGate" name="form0202_01_100">
		res, err := a.post("", P{
			"term":"01",
			"dsplyTrmSpcfdYearFrom":fmt.Sprintf("%04d", from.Year()),
			"dsplyTrmSpcfdMonthFrom":fmt.Sprintf("%02d", from.Month()),
			"dsplyTrmSpcfdDayFrom":fmt.Sprintf("%02d", from.Day()),
			"dsplyTrmSpcfdYearTo":fmt.Sprintf("%04d", to.Year()),
			"dsplyTrmSpcfdMonthTo":fmt.Sprintf("%02d", to.Month()),
			"dsplyTrmSpcfdDayTo":fmt.Sprintf("%02d", to.Day()),
			}
	*/
	return nil, nil
}

// transfar api
func (a *Account) NewTransferToRegisteredAccount(targetName string, amount int64) (common.TransferState, error) {
	return nil, nil
}

func (a *Account) CommitTransfer(tr common.TransferState, pass2 string) (string, error) {
	return "dummy", nil
}

func (a *Account) post(path string, params P) (string, error) {
	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}

	req, err := http.NewRequest("POST", baseUrl+path, strings.NewReader(values.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return a.request(req)
}

func (a *Account) get(path string) (string, error) {

	req, err := http.NewRequest("GET", baseUrl+path, nil)
	if err != nil {
		return "", err
	}
	return a.request(req)
}

func (a *Account) request(req *http.Request) (string, error) {
	res, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	b, err := ioutil.ReadAll(transform.NewReader(res.Body, japanese.ShiftJIS.NewDecoder()))
	if err != nil {
		return "", err
	}
	// TODO check error
	return string(b), err
}

func getMatchedInt(htmlStr, reStr string) (int64, error) {
	return strconv.ParseInt(strings.Replace(getMatched(htmlStr, reStr, ""), ",", "", -1), 10, 64)
}

func getMatched(htmlStr, reStr, def string) string {
	return common.GetMatched(htmlStr, reStr, def)
}
