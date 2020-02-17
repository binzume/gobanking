package mizuho

import (
	"errors"
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/binzume/gobanking/common"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// Mizuho Direct
type Account struct {
	common.BankAccount

	client  *http.Client
	form    map[string]string
	baseUrl string

	recent    []*common.Transaction
	balance   int64
	lastLogin time.Time
}
type TransferState map[string]interface{}

var _ common.Account = &Account{}

const BankCode = "0001"
const BankName = "みずほ銀行"
const MizuhoUrl = "https://web1.ib.mizuhobank.co.jp/servlet/"
const DummyFingerPrint = "version%3D3%2E2%2E0%2E0%5F3%26pm%5Ffpua%3Dmozilla"

func Login(id, password string, options map[string]interface{}) (*Account, error) {
	client, err := common.NewHttpClient()
	if err != nil {
		return nil, err
	}
	a := &Account{client: client, baseUrl: MizuhoUrl}
	err = a.Login(id, password, options)
	return a, err
}

func (a *Account) Logout() error {
	_, err := a.fetch("MENSRV0100901B")
	return err
}

func (a *Account) Login(id, password string, options map[string]interface{}) error {
	_, err := a.fetch("LOGBNK0000000B")
	if err != nil {
		return err
	}
	html, err := a.execute("LOGBNK0000001B", map[string]string{
		"pm_fp":     DummyFingerPrint,
		"txbCustNo": id,
	}, true)
	if err != nil {
		return err
	}

	// aikotoba
	qa := map[string]string{}
	for k, v := range options {
		qa[k] = v.(string)
	}
	html, err = a.sendAikotoba(html, qa)
	if err != nil {
		return err
	}
	html, err = a.sendAikotoba(html, qa)
	if err != nil {
		return err
	}

	html, err = a.execute("LOGBNK0000501B", map[string]string{
		"PASSWD_LoginPwdInput": password,
	}, true)
	if err != nil {
		return err
	}
	return a.parseTopPage(html)
}

func (a *Account) AccountInfo() *common.BankAccount {
	return &a.BankAccount
}

func (a *Account) TotalBalance() (int64, error) {
	return a.balance, nil
}

func (a *Account) LastLogin() (time.Time, error) {
	return a.lastLogin, nil
}

func (a *Account) ReloadTopPage() error {
	html, err := a.execute("MENSRV0100001B", map[string]string{}, true)
	if err != nil {
		return err
	}
	return a.parseTopPage(html)
}

func (a *Account) GetRegistered() (map[string]string, error) {
	res, err := a.execute("MENSRV0100004B", map[string]string{}, true)
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`(?s)<span\s+id="txtNickNm_0*(\d+)">([^<]+)<`)
	registered := map[string]string{}
	for _, m := range re.FindAllStringSubmatch(res, -1) {
		registered[m[2]] = m[1]
	}
	return registered, nil
}

func (a *Account) NewTransferToRegisteredAccount(targetName string, amount int64) (common.TransferState, error) {
	registered, err := a.GetRegistered()
	if err != nil {
		return nil, err
	}
	val, ok := registered[targetName]
	if !ok {
		return nil, fmt.Errorf("not registered: %s in %v", targetName, registered)
	}
	acc := "0"
	email := ""
	message := ""
	name := ""

	_, err = a.execute("TRNTRN0500001B", map[string]string{
		"lstAccLst":             acc,
		"rdoChgOrNot":           "no", // use name?
		"txbClntNmConfigClntNm": name,
		"rdoTrnsfreeSel":        val,
	}, true)
	if err != nil {
		return nil, err
	}

	res, err := a.execute("TRNTRN0507001B", map[string]string{
		"txbTrnfrAmnt":    fmt.Sprint(amount),
		"txbRecpMailAddr": email,
		"txaTxt":          message,
	}, true)
	if err != nil {
		return nil, err
	}
	// log.Println(res)

	pp := []int{0, 0, 0, 0}
	re := regexp.MustCompile(`<span id="txtScndPwdDgt(\d+)">(\d+)<`)
	for _, m := range re.FindAllStringSubmatch(res, -1) {
		i, _ := strconv.Atoi(m[1])
		pp[i-1], _ = strconv.Atoi(m[2])
	}
	if pp[0] < 1 || pp[1] < 1 || pp[2] < 1 || pp[3] < 1 {
		return nil, fmt.Errorf("error pass2 get digits.: %v", pp)
	}

	tr := common.TransferStateMap{
		"pass2_digits": pp,
		"next":         "TRNTRN0508001B",
	}

	tr["fee"] = getMatched(res, `<span\s+id="txtTrnfrFee"[^>]*>([\d,]+)`, "")
	tr["amount"] = getMatched(res, `<span\s+id="txtTrnfrAmnt"[^>]*>([\d,]+)`, "")
	tr["date"] = getMatched(res, `<span\s+id="txtTrnfrAppDate"[^>]*>([^<]+)<`, "")
	tr["payee"] = getMatched(res, `<span\s+id="txtPayeeNm"[^>]*>([^<]+)<`, "")

	return tr, nil
}

func (a *Account) CommitTransfer(tr common.TransferState, pass2 string) (string, error) {
	tr1, ok := tr.(common.TransferStateMap)
	if !ok {
		return "", errors.New("invalid paramter type: tr")
	}
	pp := tr1["pass2_digits"].([]int)
	res, err := a.execute("TRNTRN0508001B", map[string]string{
		"PASSWD_ScndPwd1":   string(pass2[pp[0]-1]),
		"PASSWD_ScndPwd2":   string(pass2[pp[1]-1]),
		"PASSWD_ScndPwd3":   string(pass2[pp[2]-1]),
		"PASSWD_ScndPwd4":   string(pass2[pp[3]-1]),
		"chkTrnfrCntntConf": "on",
	}, true)

	return getMatched(res, `<span\s+id="txtRecptNo"[^>]*>([^<]+)`, ""), err
}

func (a *Account) parseTopPage(doc string) error {

	if m := getMatched(doc, `<span\s+id="txtCrntBal"[^>]*>([\d,]+)`, ""); m != "" {
		a.balance, _ = strconv.ParseInt(strings.Replace(m, ",", "", -1), 10, 64)
	}
	a.recent = a.parseHistory(doc, a.balance)

	a.BankCode = BankCode
	a.BankName = BankName
	a.OwnerName = getMatched(doc, `<span\s+id="txtLoginInfoCustNm"[^>]*>([^<]+)`, "")
	a.BranchCode = getMatched(doc, `<meta property="page.branchcd" content="(\d+)">`, "")
	a.BranchName = getMatched(doc, `<span\s+id="txtBrnch"[^>]*>([^<]+)`, "")
	a.AccountNum = getMatched(doc, `<span\s+id="txtAccNo"[^>]*>([^<]+)`, "")

	if m := getMatched(doc, `<span\s+id="txtLastUsgTm"[^>]*>([^<]+)`, ""); m != "" {
		m = strings.Replace(m, "\uC2A0", " ", -1)
		var timeformat = "2006.01.02 15:04"
		if t, err := time.Parse(timeformat, m); err == nil {
			a.lastLogin = t
		}
	}
	return nil
}

func (a *Account) sendAikotoba(html string, qa map[string]string) (string, error) {
	if q := getMatched(html, `<span id="txtQuery">([^<]+)`, ""); q != "" {
		var ans string
		for k, v := range qa {
			if strings.Contains(q, k) {
				ans = v
			}
		}
		log.Println(q, ans != "")
		if ans == "" {
			return "", nil
		}
		return a.execute("LOGWRD0010001B", map[string]string{
			"chkConfItemChk": "on",
			"txbTestWord":    common.ToSJIS(ans),
		}, true)
	}
	return html, nil
}

func (a *Account) parseHistory(doc string, balance int64) []*common.Transaction {
	re := regexp.MustCompile(`(?s)<span\s+id="txtDate_.*?</tr>`)
	dateRe := regexp.MustCompile(`(?s)<span\s+id="txtDate_\d+">([^<]*)<`)
	descRe := regexp.MustCompile(`(?s)<span\s+id="txtTransCntnt_\d+">([^<]*)`)
	amountRe := regexp.MustCompile(`(?s)<span\s+id="txtDrawAmnt_\d+">([\d,]+)`)
	damountRe := regexp.MustCompile(`(?s)<span\s+id="txtDpstAmnt_\d+">([\d,]+)`)
	trs := []*common.Transaction{}

	for _, s := range re.FindAllString(doc, -1) {
		var tr common.Transaction
		if match := dateRe.FindStringSubmatch(s); match != nil {
			var timeformat = "2006.01.02"
			if t, err := time.Parse(timeformat, html.UnescapeString(match[1])); err == nil {
				tr.Date = t
			}
		}
		if match := descRe.FindStringSubmatch(s); match != nil {
			tr.Description = match[1]
		}
		if match := amountRe.FindStringSubmatch(s); match != nil {
			am, _ := strconv.ParseInt(strings.Replace(match[1], ",", "", -1), 10, 64)
			tr.Amount = -am
		}
		if match := damountRe.FindStringSubmatch(s); match != nil {
			am, _ := strconv.ParseInt(strings.Replace(match[1], ",", "", -1), 10, 64)
			tr.Amount = am
		}
		trs = append(trs, &tr)
	}
	if balance >= 0 {
		for i := len(trs) - 1; i >= 0; i-- {
			trs[i].Balance = balance
			balance -= trs[i].Amount
		}
	}
	return trs
}

func (a *Account) Recent() ([]*common.Transaction, error) {
	return a.recent, nil
}

func (a *Account) History(from, to time.Time) ([]*common.Transaction, error) {
	acc := "0"
	mode := "2"
	_, err := a.execute("MENSRV0100003B", map[string]string{}, true)
	if err != nil {
		return nil, err
	}
	res, err := a.execute("ACCHST0400001B", map[string]string{
		"lstAccSel":        acc,
		"rdoInqMtdSpec":    mode,
		"lstTargetMnthSel": "NO_WRITE", // (THIS_MONTH,PREV_MONTH,BEFORE_LASTMONTH,NO_WRITE)
		"lstDateFrmYear":   fmt.Sprint(from.Year()),
		"lstDateFrmMnth":   fmt.Sprint(int(from.Month())),
		"lstDateFrmDay":    fmt.Sprint(from.Day()),
		"lstDateToYear":    fmt.Sprint(to.Year()),
		"lstDateToMnth":    fmt.Sprint(int(to.Month())),
		"lstDateToDay":     fmt.Sprint(to.Day()),
	}, true)

	return a.parseHistory(res, -1), err
}

func (a *Account) execute(pageId string, params map[string]string, check bool) (string, error) {
	// log.Println("execute ", pageId, params)

	values := url.Values{}
	for k, v := range a.form {
		values.Set(k, v)
	}
	for k, v := range params {
		values.Set(k, v)
	}

	req, err := http.NewRequest("POST", a.baseUrl+pageId+".do", strings.NewReader(values.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return a.request(req)
}

func (a *Account) fetch(pageId string) (string, error) {
	// log.Println("fetch ", pageId)
	values := url.Values{}
	for k, v := range a.form {
		values.Set(k, v)
	}
	url := a.baseUrl + pageId + ".do?" + values.Encode()
	req, err := http.NewRequest("GET", url, nil)
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
	html := string(b)
	a.baseUrl = getMatched(html, `<base href="(https://[^"]+)">`, a.baseUrl)

	form := map[string]string{
		"_FRAMEID":  getFormValue(html, "_FRAMEID"),
		"_TARGETID": getFormValue(html, "_TARGETID"),
		"_LUID":     getFormValue(html, "_LUID"),     // ?
		"_SUBINDEX": getFormValue(html, "_SUBINDEX"), // ?
		"_TOKEN":    getFormValue(html, "_TOKEN"),
		"_FORMID":   getFormValue(html, "_FORMID"),
		"POSTKEY":   getFormValue(html, "POSTKEY"),
	}
	if form["POSTKEY"] == "" {
		msg := getMatched(html, `<div\s[^>]*id="ErrorMessage"[^>]*>(.+?)</"`, "")
		return html, fmt.Errorf("execute error" + msg)
	}
	a.form = form
	return html, nil
}

func getFormValue(html, name string) string {
	return getMatched(html, `<input\s[^>]*?name="`+name+`"[^>]*?value="([^"]*)"`, "")
}

func getMatched(htmlStr, reStr, def string) string {
	return common.GetMatched(htmlStr, reStr, def)
}
