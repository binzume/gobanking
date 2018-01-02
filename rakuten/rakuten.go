package rakuten

import (
	"bytes"
	"fmt"
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
	client    *http.Client
	userAgent string

	balance   int64
	userName  string
	lastLogin time.Time
}

type TempTransaction map[string]interface{}

var _ common.Account = &Account{}

const BankCode = "0036"
const baseurl = "https://fes.rakuten-bank.co.jp/"

func Login(id, password string, params interface{}) (*Account, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	a := &Account{
		client:    &http.Client{Jar: jar},
		userAgent: "Mozilla/5.0 NetBankingtClient/0.1",
	}
	err = a.Login(id, password, params)
	return a, err
}

func (a *Account) Login(id, password string, loginparams interface{}) error {
	qa, _ := loginparams.(map[string]string)
	_, err := a.get("MS/main/RbS?CurrentPageID=START&&COMMAND=LOGIN")
	if err != nil {
		return err
	}

	params := map[string]string{
		"LOGIN_SUBMIT":         "1",
		"jsf_sequence":         "1",
		"LOGIN:_link_hidden_":  "",
		"LOGIN:_idJsp84":       "",
		"LOGIN:USER_ID":        id,
		"LOGIN:LOGIN_PASSWORD": password,
	}
	res, err := a.post("mainservice/Security/LoginAuthentication/Login/Login", params)
	if err != nil {
		return err
	}

	if strings.Contains(res, "INPUT_FORM:SECRET_WORD") {
		qq := a.getMached(res, `(?s)質問<.*?>\s*([^\s<]+)\s*<`, "")
		ans := ""
		for k, v := range qa {
			if strings.Contains(qq, k) {
				ans = v
			}
		}
		log.Println("q:", qq, "->", ans)
		buf := &bytes.Buffer{}
		w := transform.NewWriter(buf, japanese.ShiftJIS.NewEncoder())
		w.Write([]byte(ans))
		ans = buf.String()

		params := map[string]string{
			"INPUT_FORM_SUBMIT":        "1",
			"jsf_sequence":             "2",
			"INPUT_FORM:_link_hidden_": "",
			"INPUT_FORM:_idJsp157":     "INPUT_FORM:_idJsp157",
			"INPUT_FORM:TOKEN":         a.getMached(res, `name="INPUT_FORM:TOKEN"\s+value="([^"]+)"`, ""),
			"INPUT_FORM:SECRET_WORD":   ans,
		}
		res, err := a.post("commonservice/Security/LoginAuthentication/SecretWordAuthentication/SecretWordAuthentication", params)
		if err != nil {
			return err
		}
		log.Println(res)
	}

	res, err = a.get("MS/main/gns?COMMAND=BALANCE_INQUIRY_START&&CurrentPageID=HEADER_FOOTER_LINK")
	if err != nil {
		return err
	}
	log.Println(res)

	a.userName = a.getMached(res, `>\s+([^<]+?)\s+様\s+<`, "")
	a.balance, _ = a.getMachedInt(res, `(?s)（支払可能残高）.*?>\s*([0-9,]+)\s*<`)
	lastLoginStr := a.getMached(res, `>\s+前回ログイン日時\s+([^<]+?)\s+<`, "")
	if t, err := time.Parse("2006/01/02 15:04:05", lastLoginStr); err == nil {
		a.lastLogin = t
	}
	// :branch => get_match(res.body, />\s+支店番号\s+([^<]+?)\s+</),
	// :acc_num => get_match(res.body, />\s+口座番号\s+([^<]+?)\s+</),

	return err
}

func (a *Account) Logout() error {
	_, err := a.get("MS/main/gns?COMMAND=LOGOUT_START&&CurrentPageID=HEADER_FOOTER_LINK")
	return err
}

func (a *Account) TotalBalance() (int64, error) {
	return a.balance, nil
}

func (a *Account) Recent() ([]*common.Transaction, error) {
	res, err := a.get("MS/main/gns?COMMAND=CREDIT_DEBIT_INQUIRY_START&CurrentPageID=HEADER_FOOTER_LINK")
	re1 := regexp.MustCompile(`(?s)<tr class="td\d\dline">(.*?)<\/tr>`)
	re2 := regexp.MustCompile(`(?s)<td[^>]*>\s*<[^>]+>\s*(.*?)<[^>]+>\s*<\/td>`)
	re3 := regexp.MustCompile(`<[^>]+>`)

	trs := []*common.Transaction{}
	for _, match := range re1.FindAllStringSubmatch(res, -1) {
		log.Println(match[1])
		cell := re2.FindAllStringSubmatch(match[1], -1)
		if len(cell) > 3 {
			var tr common.Transaction
			if t, err := time.Parse("2006/01/02", cell[0][1]); err == nil {
				tr.Date = t
			}
			tr.Description = cell[1][1]
			cell[2][1] = strings.TrimSpace(re3.ReplaceAllString(cell[2][1], ""))
			tr.Amount, _ = strconv.ParseInt(strings.Replace(cell[2][1], ",", "", -1), 10, 32)
			// balance = cell[3][1]
			trs = append(trs, &tr)
		}
	}
	return trs, err
}

func (a *Account) History(from, to time.Time) ([]*common.Transaction, error) {
	return a.Recent()
}

// transfar api
func (a *Account) NewTransactionWithNick(targetName string, amount int) (TempTransaction, error) {
	return nil, nil
}

func (a *Account) Commit(tr TempTransaction, pass2 string) (string, error) {
	return "dummy", nil
}

func (a *Account) get(path string) (string, error) {
	req, err := http.NewRequest("GET", baseurl+path, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", a.userAgent)

	res, err := a.client.Do(req)
	defer res.Body.Close()

	b, err := ioutil.ReadAll(transform.NewReader(res.Body, japanese.ShiftJIS.NewDecoder()))
	if err != nil {
		return "", err
	}
	doc := string(b)
	if msg := a.getMached(doc, `class="errortxt">(.+?)</`, ""); msg != "" {
		return doc, fmt.Errorf("ERROR: %s", html.UnescapeString(msg))
	}
	return doc, err
}

func (a *Account) post(cmd string, params map[string]string) (string, error) {
	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}
	log.Println("post ", cmd, values.Encode())

	req, err := http.NewRequest("POST", baseurl+"MS/main/fcs/rb/fes/jsp/"+cmd+".jsp", strings.NewReader(values.Encode()))
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
	doc := string(b)
	if msg := a.getMached(doc, `class="errortxt">(.+?)</`, ""); msg != "" {
		return doc, fmt.Errorf("ERROR: %s", html.UnescapeString(msg))
	}
	return doc, err
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
