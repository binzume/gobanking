package rakuten

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"

	"github.com/binzume/gobanking/common"
)

type Account struct {
	common.BankAccount

	client   *http.Client
	sequence int

	balance   int64
	userName  string
	lastLogin time.Time
}

var _ common.Account = &Account{}

const BankCode = "0036"
const BankName = "楽天銀行"
const baseurl = "https://fes.rakuten-bank.co.jp/MS/main/"

func Login(id, password string, params interface{}) (*Account, error) {
	client, err := common.NewHttpClient()
	if err != nil {
		return nil, err
	}
	a := &Account{client: client}
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

	a.balance, _ = getMatchedInt(res, `(?s)総額（評価額）.*?>\s*([0-9,]+)\s*<`)
	// a.balance, _ = getMatchedInt(res, `(?s)（支払可能残高）.*?>\s*([0-9,]+)\s*<`)
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
		cell := re2.FindAllStringSubmatch(match[1], -1)
		if len(cell) > 3 {
			var tr common.Transaction
			if t, err := time.Parse("2006/01/02", cell[0][1]); err == nil {
				tr.Date = t
			}
			tr.Description = cell[1][1]
			cell[2][1] = strings.TrimSpace(re3.ReplaceAllString(cell[2][1], ""))
			tr.Amount, _ = strconv.ParseInt(strings.Replace(cell[2][1], ",", "", -1), 10, 32)
			tr.Balance, _ = strconv.ParseInt(strings.Replace(cell[3][1], ",", "", -1), 10, 32)
			trs = append(trs, &tr)
		}
	}

	// reverse
	for i, j := 0, len(trs)-1; i < j; i, j = i+1, j-1 {
		trs[i], trs[j] = trs[j], trs[i]
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
	trs := []*common.Transaction{}
	for _, line := range strings.Split(res, "\n")[1:] {
		var row = strings.Split(line, ",")
		if len(row) >= 4 {
			var tr common.Transaction
			if t, err := time.Parse("20060102", row[0]); err == nil {
				tr.Date = t
			}
			tr.Amount, _ = strconv.ParseInt(row[1], 10, 64)
			tr.Balance, _ = strconv.ParseInt(row[2], 10, 64)
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
	res, err := a.post("mainservice/Transfer/TransferMenu/TransferMenu/TransferMenu", params)
	if err != nil {
		return nil, err
	}

	re1 := regexp.MustCompile(`(?s)<tr>\s*<td[^>]*>\s*<div class="innercellline">.*?<input id="SELECT_REGISTER_ACCOUNT:_idJsp431:[^>]+>\s*</div>\s*</td>\s*</tr>`)
	list := map[string]string{}
	for _, match := range re1.FindAllString(res, -1) {
		// log.Println(match)
		name := getMatched(match, `(?s)<div class="innercellline">\s*<span[^>]*>([^<]+)</span>`, "")
		id := getMatched(match, `<input [^>]*name="SELECT_REGISTER_ACCOUNT:_idJsp431:(\w+):_idJsp446"`, "")
		if name != "" && id != "" {
			list[name] = id
		}
	}
	return list, nil
}

func (a *Account) GetRegistered2() (map[string]string, error) {

	params := map[string]string{
		"SELECT_REGISTER_ACCOUNT_SUBMIT":        "1",
		"SELECT_REGISTER_ACCOUNT:_link_hidden_": "SELECT_REGISTER_ACCOUNT:_idJsp416", // or 412(all)
		"KANA_INDEX_KEY":                        "",
	}
	res, err := a.post("mainservice/Transfer/TransferMenu/TransferSelect/TransferSelect", params)
	if err != nil {
		return nil, err
	}

	re1 := regexp.MustCompile(`(?s)<tr>\s*<td[^>]*>\s*<div class="innercellline">.*?<input id="SELECT_REGISTER_ACCOUNT:_idJsp431:[^>]+>\s*</div>\s*</td>\s*</tr>`)
	list := map[string]string{}
	for _, match := range re1.FindAllString(res, -1) {
		log.Println(match)
		name := getMatched(match, `(?s)<div class="innercellline">\s*<span[^>]*>([^<]+)</span>`, "")
		id := getMatched(match, `<input [^>]*name="SELECT_REGISTER_ACCOUNT:_idJsp431:(\w+):_idJsp446"`, "")
		if name != "" && id != "" {
			list[name] = id
		}
	}
	return list, nil
}

// transfar api
func (a *Account) NewTransactionWithNick(targetName string, amount int64) (common.TempTransaction, error) {
	registered, err := a.GetRegistered()
	if err != nil {
		return nil, err
	}
	n, ok := registered[targetName]
	if !ok {
		registered, err = a.GetRegistered2()
		if err != nil {
			return nil, err
		}
		n, ok = registered[targetName]
		if !ok {
			return nil, fmt.Errorf("not registered: %s in %v", targetName, registered)
		}
	}

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

	action := getMatched(res, `name="FORM" [^>]*action="/MS/main/fcs/rb/fes/jsp/([^"]+)\.jsp"`, "")
	btn := getMatched(res, `name="(FORM:_idJsp\d+)" [^>]*value="次へ（確認）"`, "")
	params = map[string]string{
		"FORM_SUBMIT":                "1",
		"FORM:_link_hidden_":         "",
		btn:                          btn, // _idJsp230 181
		"FORM:COMMENT":               "",
		"FORM:DEBIT_OWNER_NAME_KANA": common.ToSJIS(getMatched(res, `name="FORM:DEBIT_OWNER_NAME_KANA" [^>]*value="([^"]+)"`, "")),
		"FORM:AMOUNT":                fmt.Sprint(amount),
	}
	res, err = a.post(action, params)
	if err != nil {
		return nil, err
	}
	// log.Println(res)

	token := getMatched(res, `name="SECURITY_BOARD:TOKEN" [^>]*value="([^"]*)"`, "")
	fee := getMatched(res, `(?s)振込手数料</div>\s*</th>\s*<td[^>]*>\s*(.*?)</td>`, "")
	date := getMatched(res, `(?s)振込予定日</div>\s*</th>\s*<td[^>]*>\s*(.*?)</td>`, "")
	to := getMatched(res, `(?s)振込先</div>\s*</th>\s*<td[^>]*>\s*(.*?)</td>`, "")
	if token == "" {
		err = fmt.Errorf("get token error")
	}
	feeint, _ := strconv.Atoi(strings.Replace(fee, ",", "", -1))
	btn = getMatched(res, `name="(SECURITY_BOARD:_idJsp\d+)" [^>]*value="振込実行"`, "")
	action = getMatched(res, `name="SECURITY_BOARD" [^>]*action="/MS/main/fcs/rb/fes/jsp/([^"]+)\.jsp"`, "")
	return common.TempTransactionMap{"token": token, "button": btn, "action": action,
		"fee_msg": fee, "fee": int(feeint), "date": date, "to": to, "amount": amount}, err
}

func (a *Account) CommitTransaction(tr common.TempTransaction, pass2 string) (string, error) {
	tr1, ok := tr.(common.TempTransactionMap)
	if !ok {
		return "", errors.New("invalid paramter type: tr")
	}
	button := tr1["button"].(string)
	params := map[string]string{
		"SECURITY_BOARD_SUBMIT":        "1",
		"SECURITY_BOARD:_link_hidden_": "",
		"SECURITY_BOARD:USER_PASSWORD": pass2,
		"SECURITY_BOARD:TOKEN":         tr1["token"].(string),
		button:                         button, // _idJsp250
	}
	res, err := a.post(tr1["action"].(string), params)
	recptNo := getMatched(res, `(?s)備考</div>\s*</th>\s*<td[^>]*>\s*<div class="innercell">\s*(\d+-\d+)\s*</div>`, res)
	return recptNo, err
}

func (a *Account) get(path string) (string, error) {
	req, err := http.NewRequest("GET", baseurl+path, nil)
	if err != nil {
		return "", err
	}

	// log.Println("GET", path)
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

	// log.Println("POST", cmd, values.Encode())
	a.sequence++
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
	doc := string(b)
	if seq := getMatched(doc, `name="jsf_sequence" [^>]*value=["'](\d+)["']`, ""); seq != "" {
		s, _ := strconv.Atoi(seq)
		a.sequence = s
	}
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
