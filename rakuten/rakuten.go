package rakuten

import (
	"fmt"
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
	common.BankAccount

	client    *http.Client
	userAgent string
	sequence  int

	balance   int64
	userName  string
	lastLogin time.Time
}

type TempTransaction map[string]string

var _ common.Account = &Account{}

const BankCode = "0036"
const BankName = "楽天銀行"
const baseurl = "https://fes.rakuten-bank.co.jp/MS/main/"

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
	_, err := a.get("RbS?CurrentPageID=START&&COMMAND=LOGIN")
	if err != nil {
		return err
	}
	a.sequence = 1

	params := map[string]string{
		"LOGIN_SUBMIT":         "1",
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
		qq := getMatched(res, `(?s)質問<.*?>\s*([^\s<]+)\s*<`, "")
		ans := ""
		for k, v := range qa {
			if strings.Contains(qq, k) {
				ans = v
			}
		}
		log.Println("q:", qq, ans != "")
		params := map[string]string{
			"INPUT_FORM_SUBMIT":        "1",
			"INPUT_FORM:_link_hidden_": "",
			"INPUT_FORM:_idJsp157":     "INPUT_FORM:_idJsp157",
			"INPUT_FORM:TOKEN":         getMatched(res, `name="INPUT_FORM:TOKEN"\s+value="([^"]+)"`, ""),
			"INPUT_FORM:SECRET_WORD":   common.ToSJIS(ans),
		}
		_, err := a.post("commonservice/Security/LoginAuthentication/SecretWordAuthentication/SecretWordAuthentication", params)
		if err != nil {
			return err
		}
	}

	res, err = a.get("gns?COMMAND=BALANCE_INQUIRY_START&&CurrentPageID=HEADER_FOOTER_LINK")
	if err != nil {
		return err
	}

	a.balance, _ = getMatchedInt(res, `(?s)（支払可能残高）.*?>\s*([0-9,]+)\s*<`)
	lastLoginStr := getMatched(res, `>\s+前回ログイン日時\s+([^<]+?)\s+<`, "")
	if t, err := time.Parse("2006/01/02 15:04:05", lastLoginStr); err == nil {
		a.lastLogin = t
	}

	a.BankCode = BankCode
	a.BankName = BankName
	a.OwnerName = getMatched(res, `>\s+([^<]+?)\s+様\s*<`, "")
	a.BranchCode = getMatched(res, `>\s+支店番号\s+([^<]+?)\s*<`, "")
	a.AccountNum = getMatched(res, `>\s+口座番号\s+([^<]+?)\s*<`, "")
	a.BranchName = getMatched(res, `(?s)>\s+([^<]+支店)\s*</FONT>\s*</TD>\s*<TD>\s*<IMG[^>]*>`, "")

	return err
}

func (a *Account) Logout() error {
	_, err := a.get("gns?COMMAND=LOGOUT_START&&CurrentPageID=HEADER_FOOTER_LINK")
	return err
}

func (a *Account) TotalBalance() (int64, error) {
	return a.balance, nil
}

func (a *Account) LastLogin() (time.Time, error) {
	return a.lastLogin, nil
}

func (a *Account) Recent() ([]*common.Transaction, error) {
	res, err := a.get("gns?COMMAND=CREDIT_DEBIT_INQUIRY_START&CurrentPageID=HEADER_FOOTER_LINK")
	re1 := regexp.MustCompile(`(?s)<tr class="td\d\dline">(.*?)<\/tr>`)
	re2 := regexp.MustCompile(`(?s)<td[^>]*>\s*<[^>]+>\s*(.*?)<[^>]+>\s*<\/td>`)
	re3 := regexp.MustCompile(`<[^>]+>`)

	trs := []*common.Transaction{}
	for _, match := range re1.FindAllStringSubmatch(res, -1) {
		// log.Println(match[1])
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
	params := map[string]string{
		"FORM_DOWNLOAD_SUBMIT":                   "1",
		"FORM_DOWNLOAD:_link_hidden_":            "",
		"FORM_DOWNLOAD:_idJsp481":                "",
		"FORM_DOWNLOAD:EXPECTED_DATE_FROM_YEAR":  fmt.Sprintf("%04d", from.Year()),
		"FORM_DOWNLOAD:EXPECTED_DATE_FROM_MONTH": fmt.Sprintf("%02d", from.Month()),
		"FORM_DOWNLOAD:EXPECTED_DATE_FROM_DAY":   fmt.Sprintf("%02d", from.Day()),
		"FORM_DOWNLOAD:EXPECTED_DATE_TO_YEAR":    fmt.Sprintf("%04d", to.Year()),
		"FORM_DOWNLOAD:EXPECTED_DATE_TO_MONTH":   fmt.Sprintf("%02d", to.Month()),
		"FORM_DOWNLOAD:EXPECTED_DATE_TO_DAY":     fmt.Sprintf("%02d", to.Day()),
		"FORM_DOWNLOAD:DOWNLOAD_TYPE":            "0",
	}
	res, err := a.post("mainservice/Inquiry/CreditDebitInquiry/CreditDebitInquiry/CreditDebitInquiry", params)
	a.sequence-- // FIXME (no redirect response?)
	trs := []*common.Transaction{}
	for _, line := range strings.Split(res, "\n")[1:] {
		var row = strings.Split(line, ",")
		if len(row) >= 4 {
			var tr common.Transaction
			if t, err := time.Parse("20060102", row[0]); err == nil {
				tr.Date = t
			}
			tr.Amount, _ = strconv.ParseInt(row[1], 10, 64)
			// tr.Balance, _ = strconv.ParseInt(row[2], 10, 64)
			tr.Description = row[3]
			trs = append(trs, &tr)
		}
	}
	return trs, err
}

func (a *Account) GetRegistered() (map[string]string, error) {
	_, err := a.get("gns?COMMAND=TRANSFER_MENU_START&&CurrentPageID=HEADER_FOOTER_LINK")
	if err != nil {
		return nil, err
	}

	params := map[string]string{
		"FORM_SUBMIT":        "1",
		"FORM:_link_hidden_": "FORM:_idJsp430",
	}
	_, err = a.post("mainservice/Transfer/TransferMenu/TransferMenu/TransferMenu", params)
	if err != nil {
		return nil, err
	}
	// TODO
	return nil, nil
}

// transfar api
// FIXME: targetName = registered index (0,1,2...)
func (a *Account) NewTransactionWithNick(targetName string, amount int) (TempTransaction, error) {
	_, err := a.GetRegistered()
	if err != nil {
		return nil, err
	}
	n := targetName // TODO

	params := map[string]string{
		"SELECT_REGISTER_ACCOUNT_SUBMIT":                        "1",
		"SELECT_REGISTER_ACCOUNT:_link_hidden_":                 "",
		"SELECT_REGISTER_ACCOUNT:_idJsp431:" + n + ":_idJsp446": "SELECT_REGISTER_ACCOUNT:_idJsp431:" + n + ":_idJsp446",
		"KANA_INDEX_KEY":                                        "",
	}
	res, err := a.post("mainservice/Transfer/TransferMenu/TransferSelect/TransferSelect", params)
	if err != nil {
		return nil, err
	}
	// log.Println(res)

	params = map[string]string{
		"FORM_SUBMIT":                "1",
		"FORM:_link_hidden_":         "",
		"FORM:_idJsp230":             "FORM:_idJsp230",
		"FORM:COMMENT":               "",
		"FORM:DEBIT_OWNER_NAME_KANA": common.ToSJIS(getMatched(res, `name="FORM:DEBIT_OWNER_NAME_KANA" [^>]*value="([^"]+)"`, "")),
		"FORM:AMOUNT":                fmt.Sprint(amount),
	}
	res, err = a.post("mainservice/Transfer/Basic/Basic/BasicRegisteredInput", params)
	if err != nil {
		return nil, err
	}
	log.Println(res)

	token := getMatched(res, `name="SECURITY_BOARD:TOKEN" [^>]*value="([^"]*)"`, "")
	fee := getMatched(res, `(?s)振込手数料</div>\s*</th>\s*<td[^>]*>\s*(.*?)</td>`, "")
	date := getMatched(res, `(?s)振込予定日</div>\s*</th>\s*<td[^>]*>\s*(.*?)</td>`, "")
	to := getMatched(res, `(?s)振込先</div>\s*</th>\s*<td[^>]*>\s*(.*?)</td>`, "")
	if token == "" {
		err = fmt.Errorf("get token error")
	}
	return TempTransaction{"token": token, "fee": fee, "date": date, "to": to}, err
}

func (a *Account) Commit(tr TempTransaction, pass2 string) (string, error) {
	params := map[string]string{
		"SECURITY_BOARD_SUBMIT:1":      "1",
		"SECURITY_BOARD:_link_hidden_": "",
		"SECURITY_BOARD:_idJsp905":     "SECURITY_BOARD:_idJsp905",
		"SECURITY_BOARD:USER_PASSWORD": pass2, // TODO
		"SECURITY_BOARD:TOKEN":         tr["token"],
	}
	res, err := a.post("mainservice/Transfer/Basic/Basic/BasicConfirm", params)
	return res, err
}

func (a *Account) get(path string) (string, error) {
	req, err := http.NewRequest("GET", baseurl+path, nil)
	if err != nil {
		return "", err
	}

	log.Println("GET", path)
	a.sequence += 2
	return a.request(req)
}

func (a *Account) post(cmd string, params map[string]string) (string, error) {
	values := url.Values{}
	values.Set("jsf_sequence", fmt.Sprint(a.sequence))
	for k, v := range params {
		values.Set(k, v)
	}

	req, err := http.NewRequest("POST", baseurl+"fcs/rb/fes/jsp/"+cmd+".jsp", strings.NewReader(values.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	log.Println("POST", cmd, values.Encode())
	a.sequence++
	return a.request(req)
}

func (a *Account) request(req *http.Request) (string, error) {
	req.Header.Set("User-Agent", a.userAgent)
	res, err := a.client.Do(req)
	if err != nil {
		return "", err
	}

	defer res.Body.Close()
	b, err := ioutil.ReadAll(transform.NewReader(res.Body, japanese.ShiftJIS.NewDecoder()))
	if err != nil {
		return "", err
	}
	doc := string(b)
	// log.Println(getMatched(doc, `name="jsf_sequence" [^>]*value=["'](\d+)["']`, "not found"))
	if msg := getMatched(doc, `class="errortxt">(.+?)</`, ""); msg != "" {
		return doc, fmt.Errorf("ERROR: %s", msg)
	}
	return doc, err
}

func getMatchedInt(htmlStr, reStr string) (int64, error) {
	return strconv.ParseInt(strings.Replace(getMatched(htmlStr, reStr, ""), ",", "", -1), 10, 64)
}

func getMatched(htmlStr, reStr, def string) string {
	return common.GetMatched(htmlStr, reStr, def)
}
