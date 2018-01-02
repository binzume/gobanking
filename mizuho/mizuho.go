package mizuho

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

	"../common" // TODO

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// Mizuho Direct
type Account struct {
	baseUrl string
	client  *http.Client
	form    map[string]string

	recent       []*common.Transaction
	balance      int64
	lastLogin    time.Time
	userName     string
	lastResponse string
}
type TempTransaction map[string]interface{}

var _ common.Account = &Account{}

const BankCode = "0001"
const MizuhoUrl = "https://web1.ib.mizuhobank.co.jp/servlet/"

func Login(id, password string, qa map[string]string) (*Account, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	a := &Account{
		baseUrl: MizuhoUrl,
		client:  &http.Client{Jar: jar},
	}
	err = a.Login(id, password, qa)
	return a, err
}

func (a *Account) Logout() error {
	_, err := a.fetch("MENSRV0100901B")
	return err
}

func (a *Account) execute(pageId string, params map[string]string, check bool) (string, error) {
	log.Println("execute ", pageId, params)

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
	req.Header.Set("User-Agent", "Mozilla/5.0 MizuhoDirectClient/0.1")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := a.client.Do(req)
	if res.Request.Method != "GET" {
		defer res.Body.Close()

		b, err := ioutil.ReadAll(transform.NewReader(res.Body, japanese.ShiftJIS.NewDecoder()))
		if err != nil {
			return "", err
		}

		re := regexp.MustCompile(`<div\s[^>]*id="ErrorMessage"[^>]*>(.+?)</"`)
		var msg string
		if match := re.FindStringSubmatch(string(b)); match != nil {
			msg = match[1]
		}
		a.lastResponse = string(b)
		return "", fmt.Errorf("execute error" + msg)
	}

	return a.parse(res)
}

func (a *Account) getFormValue(html, name string) string {
	re := regexp.MustCompile(`<input\s[^>]*?name="` + name + `"[^>]*?value="([^"]*)"`)
	match := re.FindStringSubmatch(html)
	if match != nil {
		return match[1]
	}
	return ""
}

func (a *Account) parse(res *http.Response) (string, error) {
	defer res.Body.Close()

	b, err := ioutil.ReadAll(transform.NewReader(res.Body, japanese.ShiftJIS.NewDecoder()))
	if err != nil {
		return "", err
	}
	html := string(b)
	a.lastResponse = html

	basere := regexp.MustCompile(`<base href="(https://[^"]+)">`)
	base := basere.FindStringSubmatch(html)
	if base != nil {
		a.baseUrl = base[1]
	}

	form := map[string]string{
		"_FRAMEID":  a.getFormValue(html, "_FRAMEID"),
		"_TARGETID": a.getFormValue(html, "_TARGETID"),
		"_LUID":     a.getFormValue(html, "_LUID"),     // ?
		"_SUBINDEX": a.getFormValue(html, "_SUBINDEX"), // ?
		"_TOKEN":    a.getFormValue(html, "_TOKEN"),
		"_FORMID":   a.getFormValue(html, "_FORMID"),
		"POSTKEY":   a.getFormValue(html, "POSTKEY"),
	}
	if len(form) == 0 {
		re := regexp.MustCompile(`<div\s[^>]*id="ErrorMessage"[^>]*>(.+?)</"`)
		var msg string
		if match := re.FindStringSubmatch(html); match != nil {
			msg = match[1]
		}
		return html, fmt.Errorf("execute error" + msg)
	}
	a.form = form
	return html, nil
}

func (a *Account) fetch(pageId string) (string, error) {
	log.Println("fetch ", pageId)
	values := url.Values{}
	for k, v := range a.form {
		values.Set(k, v)
	}
	url := a.baseUrl + pageId + ".do?" + values.Encode()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 MizuhoDirectClient/0.1")

	res, err := a.client.Do(req)
	if err != nil {
		return "", err
	}

	return a.parse(res)
}

func (a *Account) Login(id, password string, params interface{}) error {
	qa := params.(map[string]string)

	_, err := a.fetch("LOGBNK0000000B")
	if err != nil {
		return err
	}
	html, err := a.execute("LOGBNK0000001B", map[string]string{
		"pm_fp":     "version%3D3%2E2%2E0%2E0%5F3%26pm%5Ffpua%3Dmozilla", // FingerPrint
		"txbCustNo": id,
	}, true)
	if err != nil {
		return err
	}

	// aikotoba
	html, err = a.sendAikotoba(html, qa)
	if err != nil {
		return err
	}
	html, err = a.sendAikotoba(html, qa)
	if err != nil {
		return err
	}
	html, err = a.sendPassword(html, password)
	if err != nil {
		return err
	}

	return a.parseTopPage(html)
}

func (a *Account) TotalBalance() (int64, error) {
	return a.balance, nil
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

func (a *Account) NewTransactionWithNick(targetName string, amount int) (TempTransaction, error) {
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

	pp := []int{0, 0, 0, 0}
	re := regexp.MustCompile(`<span id="txtScndPwdDgt(\d+)">(\d+)<`)
	for _, m := range re.FindAllStringSubmatch(res, -1) {
		i, _ := strconv.Atoi(m[1])
		pp[i-1], _ = strconv.Atoi(m[2])
	}
	if pp[0] < 1 || pp[1] < 1 || pp[2] < 1 || pp[3] < 1 {
		return nil, fmt.Errorf("error pass2 get digits.: %v", pp)
	}

	tr := map[string]interface{}{
		"pass2_digits": pp,
		"next":         "TRNTRN0508001B",
	}

	re = regexp.MustCompile(`<span\s+id="txtTrnfrFee"[^>]*>([\d,]+)`)
	if m := re.FindStringSubmatch(res); m != nil {
		tr["fee"] = m[1]
	}
	re = regexp.MustCompile(`<span\s+id="txtTrnfrAmnt"[^>]*>([\d,]+)`)
	if m := re.FindStringSubmatch(res); m != nil {
		tr["amount"] = m[1]
	}
	re = regexp.MustCompile(`<span\s+id="txtTrnfrAppDate"[^>]*>([^<]+)<`)
	if m := re.FindStringSubmatch(res); m != nil {
		tr["date"] = m[1]
	}
	re = regexp.MustCompile(`<span\s+id="txtPayeeNm"[^>]*>([^<]+)<`)
	if m := re.FindStringSubmatch(res); m != nil {
		tr["payee"] = m[1]
	}

	log.Println(res)

	return tr, nil
}

func (a *Account) Commit(tr TempTransaction, pass2 string) (string, error) {
	pp := tr["pass2_digits"].([]int)
	res, err := a.execute("TRNTRN0508001B", map[string]string{
		"PASSWD_ScndPwd1":   string(pass2[pp[0]-1]),
		"PASSWD_ScndPwd2":   string(pass2[pp[1]-1]),
		"PASSWD_ScndPwd3":   string(pass2[pp[2]-1]),
		"PASSWD_ScndPwd4":   string(pass2[pp[3]-1]),
		"chkTrnfrCntntConf": "on",
	}, true)

	re := regexp.MustCompile(`<span\s+id="txtRecptNo"[^>]*>([^<]+)`)
	if m := re.FindStringSubmatch(res); m != nil {
		return m[1], err
	}

	return "", err
}

func (a *Account) parseTopPage(doc string) error {
	a.recent = a.parseHistory(doc)
	re := regexp.MustCompile(`<span\s+id="txtCrntBal"[^>]*>([\d,]+)`)
	if m := re.FindStringSubmatch(doc); m != nil {
		a.balance, _ = strconv.ParseInt(strings.Replace(m[1], ",", "", -1), 10, 64)
	}

	re = regexp.MustCompile(`<span\s+id="txtLoginInfoCustNm"[^>]*>([^<]+)`)
	if m := re.FindStringSubmatch(doc); m != nil {
		a.userName = html.UnescapeString(m[1])
	}

	re = regexp.MustCompile(`<span\s+id="txtLastUsgTm"[^>]*>([^<]+)`)
	if m := re.FindStringSubmatch(doc); m != nil {
		m[1] = strings.Replace(m[1], "&nbsp;", " ", -1)
		var timeformat = "2006.01.02 15:04"
		if t, err := time.Parse(timeformat, html.UnescapeString(m[1])); err == nil {
			a.lastLogin = t
		}
	}
	return nil
}

func (a *Account) sendAikotoba(html string, qa map[string]string) (string, error) {
	re := regexp.MustCompile(`<span id="txtQuery">([^<]+)`)
	if match := re.FindStringSubmatch(html); match != nil {
		var ans string
		for k, v := range qa {
			if strings.Contains(match[1], k) {
				ans = v
			}
		}
		log.Println(match[1], "->", ans)
		if ans == "" {
			return "", nil
		}
		buf := &bytes.Buffer{}
		w := transform.NewWriter(buf, japanese.ShiftJIS.NewEncoder())
		w.Write([]byte(ans))
		ans = buf.String()
		return a.execute("LOGWRD0010001B", map[string]string{
			"chkConfItemChk": "on",
			"txbTestWord":    ans,
		}, true)
	}
	return html, nil
}

func (a *Account) sendPassword(html, password string) (string, error) {
	return a.execute("LOGBNK0000501B", map[string]string{
		"PASSWD_LoginPwdInput": password,
	}, true)
}

func (a *Account) parseHistory(doc string) []*common.Transaction {
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

	return a.parseHistory(res), err
}
