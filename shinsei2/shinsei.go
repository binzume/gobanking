package shinsei2

import (
	"encoding/json"
	"time"

	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/binzume/gobanking/common"
)

type Account struct {
	client *http.Client

	instanceID    string
	csrfToken     string
	mainAccountNo string

	balance           int64
	lastLogin         time.Time
	recentTransaction []*common.Transaction
}

var _ common.Account = &Account{}

const BankCode = "0397"
const baseUrl = "https://bk.shinseibank.com/SFC/apps/services/"

type P map[string]string

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
	err := a.init()
	if err != nil {
		return err
	}

	var securityConnectRes struct {
		AuthStatus string `json:"authStatus"`
	}
	err = a.query("IFCM_CommonAdapter", "securityConnect", nil, &securityConnectRes)
	if err != nil {
		return err
	}
	if securityConnectRes.AuthStatus != "required" {
		return fmt.Errorf("invalid authStatus: %v", securityConnectRes.AuthStatus)
	}

	r, err := a.post("login_auth_request_url", P{
		"fldUserID":  id,
		"password":   password,
		"langCode":   "JAP",
		"mode":       "1",
		"postubFlag": "0",
	})
	if err != nil {
		return err
	}

	var res struct {
		AuthStatus string `json:"authStatus"`
		Token      string `json:"token"`
	}
	err = json.Unmarshal([]byte(r), &res)
	if err != nil {
		return err
	}
	if res.AuthStatus != "success" {
		return fmt.Errorf("invalid authStatus: %v", res.AuthStatus)
	}

	a.csrfToken = res.Token
	return a.GetAccountsBalanceAndActivity()
}

func (a *Account) Logout() error {
	return nil
}

func (a *Account) TotalBalance() (int64, error) {
	return a.balance, nil
}

func (a *Account) LastLogin() (time.Time, error) {
	return a.lastLogin, nil
}

func (a *Account) Recent() ([]*common.Transaction, error) {
	return a.recentTransaction, nil
}

func (a *Account) History(from, to time.Time) ([]*common.Transaction, error) {
	fromStr := ""
	toStr := ""
	if !from.IsZero() {
		toStr = fmt.Sprintf("%04d%02d%02d", from.Year(), from.Month(), from.Day())
	}
	if !to.IsZero() {
		toStr = fmt.Sprintf("%04d%02d%02d", to.Year(), to.Month(), to.Day())
	}
	req := map[string]interface{}{
		"requestParam": map[string]string{
			"accountNo": a.mainAccountNo,
			"type":      "0",
			"fromDate":  fromStr,
			"toDate":    toStr,
		},
	}
	var activityRes struct {
		IsSuccessful bool `json:"isSuccessful"`
		Response     struct {
			Activity struct {
				Response struct {
					AccountNo       string `json:"accountNo"`
					CurrentBalance  string `json:"currentBalance"`
					ActivityDetails []*struct {
						PostingDate    string `json:"postingDate"`
						Balance        int64  `json:"balance,string"`
						Description    string `json:"description"`
						TxnReferenceNo string `json:"txnReferenceNo"`
						Debit          string `json:"debit"`
						Credit         string `json:"credit"`
					} `json:"activityDetails"`
				} `json:"responseParam"`
			} `json:"activity"`
		} `json:"responseParam"`
	}
	err := a.query("IFAI_AccountAdapter", "getCasaAccountActivitySpecificPeriod", req, &activityRes)
	if err != nil {
		return nil, err
	}
	log.Print(activityRes)

	var trs []*common.Transaction
	for _, tr := range activityRes.Response.Activity.Response.ActivityDetails {
		date, _ := time.Parse("2006/01/02", tr.PostingDate)
		credit, _ := strconv.ParseInt(tr.Credit, 10, 0)
		debit, _ := strconv.ParseInt(tr.Debit, 10, 0)
		trs = append(trs, &common.Transaction{
			Date:        date,
			Balance:     tr.Balance,
			Description: tr.Description,
			Amount:      credit - debit,
		})
	}

	return trs, err
}

// transfar api
func (a *Account) NewTransferToRegisteredAccount(targetName string, amount int64) (common.TransferState, error) {
	// TODO
	return common.TransferStateMap{}, nil
}

func (a *Account) CommitTransfer(tr common.TransferState, pass2 string) (string, error) {
	// TODO
	return "", nil
}

func (a *Account) GetAccountsBalanceAndActivity() error {

	var accountsRes struct {
		IsSuccessful bool `json:"isSuccessful"`
		Response     struct {
			Overview struct {
				Response struct {
					TotalCreditBalance int64 `json:"totalCreditBalance,string"`
					TdBalance          int64 `json:"tdBalance,string"`
					SavingsBalance     int64 `json:"savingsBalance,string"`
				} `json:"responseParam"`
			} `json:"overview"`
			Activity struct {
				Response struct {
					AccountNo       string `json:"accountNo"`
					CurrentBalance  string `json:"currentBalance"`
					ActivityDetails []*struct {
						PostingDate    string `json:"postingDate"`
						Balance        int64  `json:"balance,string"`
						Description    string `json:"description"`
						TxnReferenceNo string `json:"txnReferenceNo"`
						Debit          string `json:"debit"`
						Credit         string `json:"credit"`
					} `json:"activityDetails"`
				} `json:"responseParam"`
			} `json:"activity"`
		} `json:"responseParam"`
	}
	err := a.query("IFTP_TopAdapter", "getAccountsBalanceAndActivity", nil, &accountsRes)
	if err != nil {
		return err
	}
	log.Print(accountsRes)

	overview := accountsRes.Response.Overview.Response
	a.balance = overview.TotalCreditBalance
	a.mainAccountNo = accountsRes.Response.Activity.Response.AccountNo

	var trs []*common.Transaction
	for _, tr := range accountsRes.Response.Activity.Response.ActivityDetails {
		date, _ := time.Parse("2006/01/02", tr.PostingDate)
		credit, _ := strconv.ParseInt(tr.Credit, 10, 0)
		debit, _ := strconv.ParseInt(tr.Debit, 10, 0)
		trs = append(trs, &common.Transaction{
			Date:        date,
			Balance:     tr.Balance,
			Description: tr.Description,
			Amount:      credit - debit,
		})
	}
	a.recentTransaction = trs
	return nil
}

func (a *Account) post(path string, params P) (string, error) {
	values := url.Values{}
	values.Set("MfcISAPICommand", "EntryFunc")
	for k, v := range params {
		values.Set(k, v)
	}
	// log.Println("post ", params)

	req, err := http.NewRequest("POST", baseUrl+path, strings.NewReader(values.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("WL-Instance-Id", a.instanceID)
	req.Header.Set("X-CSRF-Token", a.csrfToken)
	req.Header.Set("x-wl-app-version", "1.0")
	req.Header.Set("x-wl-clientlog-appname", "SFC")
	req.Header.Set("x-wl-clientlog-appversion", "1.0")
	req.Header.Set("x-wl-clientlog-env", "desktopbrowser")
	req.Header.Set("x-wl-clientlog-deviceId", "UNKNOWN")
	req.Header.Set("x-wl-clientlog-model", "UNKNOWN")
	req.Header.Set("x-wl-clientlog-osversion", "UNKNOWN")

	res, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	return strings.TrimSuffix(strings.TrimPrefix(string(b), "/*-secure-"), "*/"), err
}

func (a *Account) init() error {
	// api/SFC/desktopbrowser/init
	r, err := a.post("api/SFC/desktopbrowser/init", P{
		"isAjaxRequest": "1",
		"x":             "0",
	})
	if err != nil {
		return err
	}
	var res struct {
		Challenges struct {
			Realm map[string]string `json:"wl_antiXSRFRealm"`
		} `json:"challenges"`
	}

	err = json.Unmarshal([]byte(r), &res)
	if err != nil {
		return err
	}
	a.instanceID = res.Challenges.Realm["WL-Instance-Id"]

	r, err = a.post("api/SFC/desktopbrowser/init", P{
		"isAjaxRequest": "1",
		"x":             "0",
	})
	if err != nil {
		return err
	}
	return nil
}

func (a *Account) query(adapter, procedure string, parameters interface{}, res interface{}) error {
	var parametersStr string
	if parameters != nil {
		jsonParam, err := json.Marshal(parameters)
		if err != nil {
			return err
		}
		parametersStr = string(jsonParam)
	}
	r, err := a.post("api/SFC/desktopbrowser/query", P{
		"adapter":       adapter,
		"procedure":     procedure,
		"parameters":    "[" + parametersStr + "]",
		"isAjaxRequest": "1",
		"x":             "0",
	})
	if err != nil {
		return err
	}
	if res != nil {
		err = json.Unmarshal([]byte(r), res)
		if err != nil {
			return err
		}
	}
	return nil
}
