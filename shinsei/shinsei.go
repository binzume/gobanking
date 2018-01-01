package shinsei

import (
	"time"

	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"../common" // TODO
)

type SubAccount struct {
	Id          string // 400xxxx or any
	Type        string // CHECKING or SAVINGS
	Curr        string // JPY or USD ...
	Description string
	BaseBalance int64
	CurrBalance float64
}

type Account struct {
	client    *http.Client
	userAgent string

	balance        int64
	userName       string
	lastLogin      time.Time
	ssid           string
	accounts       map[string]*SubAccount
	currentAccount *SubAccount
}

type P map[string]string

const baseUrl = "https://pdirect04.shinseibank.com/FLEXCUBEAt/LiveConnect.dll"

type TempTransaction map[string]string

var _ common.Account = &Account{}

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

func (a *Account) Login(id, password string, loginParams interface{}) error {

	paramsMap, _ := loginParams.(map[string]string)

	num := paramsMap["numId"]
	params := P{
		"MfcISAPICommand": "EntryFunc",
		"fldAppID":        "RT",
		"fldTxnID":        "LGN",
		"fldScrSeqNo":     "01",
		"fldRequestorID":  "41",
		"fldDeviceID":     "01",
		"fldLangID":       "JPN",
		"fldUserID":       id,
		"fldUserNumId":    num,
		"fldUserPass":     password,
		"fldRegAuthFlag":  "A",
	}
	values, err := a.execute(params)
	if err != nil {
		return err
	}

	a.ssid = values["fldSessionID"]
	log.Println(values)
	grid := strings.Split(paramsMap["grid"], " ")

	_, err = a.execute(P{
		"MfcISAPICommand":   "EntryFunc",
		"fldAppID":          "RT",
		"fldTxnID":          "LGN",
		"fldScrSeqNo":       "41",
		"fldRequestorID":    "55",
		"fldSessionID":      a.ssid,
		"fldDeviceID":       "01",
		"fldLangID":         "JPN",
		"fldGridChallange1": a.getgrid(grid, values["fldGridChallange1"]),
		"fldGridChallange2": a.getgrid(grid, values["fldGridChallange2"]),
		"fldGridChallange3": a.getgrid(grid, values["fldGridChallange3"]),
		"fldUserID":         "",
		"fldUserNumId":      "",
		"fldNumSeq":         "1",
		"fldRegAuthFlag":    values["fldRegAuthFlag"],
	})
	if err != nil {
		return err
	}

	err = a.ReloadTopPage()

	return err
}

func (a *Account) Logout() error {
	return nil
}

func (a *Account) TotalBalance() (int64, error) {
	return a.balance, nil
}

func (a *Account) Recent() ([]*common.Transaction, error) {
	return a.History(time.Time{}, time.Time{})
}

func (a *Account) History(from, to time.Time) ([]*common.Transaction, error) {
	fromStr := ""
	toStr := "" // from.strftime("%Y%m%d") : ""
	if !from.IsZero() {
		toStr = fmt.Sprintf("%04d%02d%02d", from.Year(), from.Month(), from.Day())
	}
	if !to.IsZero() {
		toStr = fmt.Sprintf("%04d%02d%02d", to.Year(), to.Month(), to.Day())
	}
	values, err := a.execute(P{
		"MfcISAPICommand": "EntryFunc",
		"fldAppID":        "RT",
		"fldTxnID":        "ACA",
		"fldScrSeqNo":     "01",
		"fldRequestorID":  "9",
		"fldSessionID":    a.ssid,

		"fldAcctID":     a.currentAccount.Id,
		"fldAcctType":   a.currentAccount.Type,
		"fldIncludeBal": "N",

		"fldStartDate": fromStr,
		"fldEndDate":   toStr,
		"fldStartNum":  "0",
		"fldEndNum":    "0",
		"fldCurDef":    "JPY",
		"fldPeriod":    "1",
	})
	trs := []*common.Transaction{}
	for i := 0; i < 999; i++ {
		date, ok := values[fmt.Sprintf("fldDate[%d]", i)]
		if !ok {
			break
		}
		var tr common.Transaction
		if t, err := time.Parse("2006/01/02", date); err == nil {
			tr.Date = t
		}
		tr.Description = values[fmt.Sprintf("fldDesc[%d]", i)]
		tr.Amount, _ = strconv.ParseInt(values[fmt.Sprintf("fldAmount[%d]", i)], 10, 64)
		trs = append(trs, &tr)
	}
	return trs, err
}

// transfar api
func (a *Account) NewTransactionWithNick(targetName string, amount int64) (TempTransaction, error) {
	values, err := a.execute(P{
		"MfcISAPICommand": "EntryFunc",
		"fldAppID":        "RT",
		"fldTxnID":        "ZNT",
		"fldScrSeqNo":     "00",
		"fldRequestorID":  "71",
		"fldSessionID":    a.ssid,
	})
	if err != nil {
		return nil, err
	}
	log.Println(values)

	type TargetAccount struct {
		Id, Type, Name, Bank, BankKanji, BankKana, Branch, BranchKanji, BranchKana string
	}
	var registeredAccounts []*TargetAccount
	for i := 0; i < 999; i++ {
		n := fmt.Sprint(i)
		if _, ok := values["fldListPayeeAcctId["+n+"]"]; !ok {
			break
		}
		acc := &TargetAccount{
			Id:          values["fldListPayeeAcctId["+n+"]"],
			Type:        values["fldListPayeeAcctType["+n+"]"],
			Name:        values["fldListPayeeName["+n+"]"],
			Bank:        values["fldListPayeeBank["+n+"]"],
			BankKanji:   values["fldListPayeeBankKanji["+n+"]"],
			BankKana:    values["fldListPayeeBankKana["+n+"]"],
			Branch:      values["fldListPayeeBranch["+n+"]"],
			BranchKanji: values["fldListPayeeBranchKanji["+n+"]"],
			BranchKana:  values["fldListPayeeBranchKana["+n+"]"],
		}
		registeredAccounts = append(registeredAccounts, acc)
	}

	var target *TargetAccount
	for _, acc := range registeredAccounts {
		log.Printf("%v\n", acc)
		if acc.Id == targetName { // FIXME
			target = acc
		}
	}

	if target == nil {
		return nil, fmt.Errorf("not registered: %s in %v", targetName, registeredAccounts)
	}
	limit, _ := strconv.ParseInt(values["fldDomFTLimit"], 10, 64)
	if amount > limit {
		return nil, fmt.Errorf("amount limited: %d > %d", amount, limit)
	}
	memo := values["fldRemitterName"] // FIXME
	// rem := fldRemReimburse

	values, err = a.execute(P{
		"MfcISAPICommand": "EntryFunc",
		"fldAppID":        "RT",
		"fldTxnID":        "ZNT",
		"fldScrSeqNo":     "07",
		"fldRequestorID":  "74",
		"fldSessionID":    a.ssid,

		"fldAcctId":       a.currentAccount.Id,
		"fldAcctType":     a.currentAccount.Type,
		"fldAcctDesc":     a.currentAccount.Description,
		"fldMemo":         memo,
		"fldRemitterName": values["fldRemitterName"],
		//"fldInvoice":"",
		//"fldInvoicePosition":"B",
		"fldTransferAmount": fmt.Sprint(amount),
		"fldTransferType":   "P", // P(pre registerd) or D
		//"fldPayeeId":"",
		"fldPayeeName":     target.Name,
		"fldPayeeAcctId":   target.Id,
		"fldPayeeAcctType": target.Type,
		//fldPayeeBankCode:undefined
		"fldPayeeBankName":      target.Bank,
		"fldPayeeBankNameKana":  target.BankKana,
		"fldPayeeBankNameKanji": target.BankKanji,
		//fldPayeeBranchCode:undefined
		"fldPayeeBranchName":      target.Branch,
		"fldPayeeBranchNameKana":  target.BranchKana,
		"fldPayeeBranchNameKanji": target.BranchKanji,
	})
	if err != nil {
		return nil, err
	}
	log.Println(values)
	// fee := fldTransferFee - fldReimbursedAmt

	return values, nil
}

func (a *Account) Commit(tr TempTransaction, pass2 string) (string, error) {
	params := P{
		"MfcISAPICommand": "EntryFunc",
		"fldAppID":        "RT",
		"fldTxnID":        "ZNT",
		"fldScrSeqNo":     "08",
		"fldRequestorID":  "76",
		"fldSessionID":    a.ssid,
	}
	fields := []string{
		"fldAcctId", "fldAcctType", "fldAcctDesc", "fldRemitterName",
		"fldPayeeName", "fldPayeeAcctId", "fldPayeeAcctType",
		"fldPayeeBankName", "fldPayeeBankNameKana", "fldPayeeBankNameKanji",
		"fldPayeeBranchName", "fldPayeeBranchNameKana", "fldPayeeBranchNameKanji",
		"fldTransferType", "fldReimbursedAmt", "fldRemReimburse",
		"fldMemo", "fldInvoicePosition", "fldTransferType", "fldTransferDate",
	}
	params["fldTransferAmount"] = tr["fldTransferAmountUnformatted"]
	params["fldTransferFee"] = tr["fldTransferFeeUnformatted"]
	for _, f := range fields {
		if v, ok := tr[f]; ok {
			params[f] = v
		} else {
			log.Println("fields not found", f)
		}
	}
	values, err := a.execute(params)
	if err != nil {
		return "", err
	}
	log.Println(values)
	return values["fldHostRef"], nil
}

func (a *Account) ReloadTopPage() error {
	values, err := a.execute(P{
		"MfcISAPICommand": "EntryFunc",
		"fldAppID":        "RT",
		"fldTxnID":        "ACS",
		"fldScrSeqNo":     "00",
		"fldRequestorID":  "23",
		"fldSessionID":    a.ssid,

		"fldAcctID":     "", // 400????
		"fldAcctType":   "CHECKING",
		"fldIncludeBal": "Y",
		"fldPeriod":     "",
		"fldCurDef":     "JPY",
	})
	if err != nil {
		return err
	}
	// log.Println(values)
	a.balance, _ = strconv.ParseInt(strings.Replace(values["fldGrandTotalCR"], ",", "", -1), 10, 64)

	accounts := map[string]*SubAccount{}

	re := regexp.MustCompile(`fldAccountID\[(\d+)\]`)
	for k, v := range values {
		if m := re.FindStringSubmatch(k); m != nil {
			acc := &SubAccount{
				Id:          v,
				Type:        values["fldAccountType["+m[1]+"]"],
				Curr:        values["fldCurrCcy["+m[1]+"]"],
				Description: values["fldAccountDesc["+m[1]+"]"],
			}
			accounts[v] = acc
			if m[1] == "0" {
				a.currentAccount = acc
			}
		}
	}
	a.accounts = accounts

	return err
}

func (a *Account) execute(params P) (map[string]string, error) {
	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}
	log.Println("execute ", params)

	req, err := http.NewRequest("POST", baseUrl, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", a.userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := a.client.Do(req)
	defer res.Body.Close()

	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	// TODO check error fldErrorID!="0"
	/*
		<div id="main">
			<p class="error">[message]</p>
		</div>
	*/
	return a.parse(string(b)), err
}

func (a *Account) getgrid(grid []string, pos string) string {
	return string(grid[int(pos[1]-'0')][int(pos[0]-'A')])
}

func (a *Account) parse(doc string) map[string]string {
	values := map[string]string{}
	re := regexp.MustCompile(`(fld[A-Z]\w*(\[\d+\])?)=['"]([^'"]+)['"]`)
	for _, m := range re.FindAllStringSubmatch(doc, -1) {
		values[m[1]] = m[3]
	}
	return values
}
