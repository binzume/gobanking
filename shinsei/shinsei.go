package shinsei

import (
	"bytes"
	"encoding/json"
	"time"

	"fmt"
	"io/ioutil"
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

	secureGrid []string
}

type activityResponse struct {
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
}

type authStatusResponse struct {
	AuthStatus string `json:"authStatus"`
	Token      string `json:"token"`
}

var _ common.Account = &Account{}

const BankCode = "0397"
const baseUrl = "https://bk.shinseibank.com/SFC/apps/services/"

type P map[string]string

func Login(id, password string, options map[string]interface{}) (*Account, error) {
	client, err := common.NewHttpClient()
	if err != nil {
		return nil, err
	}
	a := &Account{client: client}
	err = a.Login(id, password, options)
	return a, err
}

func (a *Account) Login(id, password string, options map[string]interface{}) error {
	if grid, ok := options["grid"].([]string); ok {
		a.secureGrid = grid
	}
	if grid, ok := options["grid"].([]interface{}); ok {
		for _, f := range grid {
			a.secureGrid = append(a.secureGrid, f.(string))
		}
	}

	err := a.init()
	if err != nil {
		return err
	}

	var securityConnectRes authStatusResponse
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

	var res authStatusResponse
	err = json.Unmarshal(r, &res)
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
	_, err := a.post("api/SFC/desktopbrowser/logout", P{
		"realm":         "ShinseiAuthenticatorRealm",
		"isAjaxRequest": "1",
		"x":             "0",
	})
	return err
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
	typ := "0"
	if !from.IsZero() {
		fromStr = fmt.Sprintf("%04d%02d%02d", from.Year(), from.Month(), from.Day())
		typ = "1"
	}
	if !to.IsZero() {
		toStr = fmt.Sprintf("%04d%02d%02d", to.Year(), to.Month(), to.Day())
		typ = "1"
	}
	req := map[string]interface{}{
		"requestParam": map[string]string{
			"accountNo": a.mainAccountNo,
			"type":      typ,
			"fromDate":  fromStr,
			"toDate":    toStr,
		},
	}

	var activityRes struct {
		Activity struct {
			Response activityResponse `json:"responseParam"`
		} `json:"activity"`
	}
	err := a.query("IFAI_AccountAdapter", "getCasaAccountActivitySpecificPeriod", req, &activityRes)
	if err != nil {
		return nil, err
	}

	var trs []*common.Transaction
	for _, tr := range activityRes.Activity.Response.ActivityDetails {
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
	var res struct {
		BeneficiaryList struct {
			Response struct {
				Details []map[string]string `json:"details"`
			} `json:"responseParam"`
		} `json:"beneficiaryListAPIParam"`
	}
	err := a.query("IFTR_TransferAdapter", "getTransferBeneficiaryList", nil, &res)
	if err != nil {
		return nil, err
	}

	var target map[string]string
	for _, detail := range res.BeneficiaryList.Response.Details {
		if detail["beneficiaryAccountNo"] == targetName {
			target = detail
		}
	}
	if target == nil {
		return nil, fmt.Errorf("not found")
	}

	common.DebugLog("target: ", target)

	req := map[string]interface{}{
		"requestParam": map[string]interface{}{
			"senderAccountNo":        a.mainAccountNo,
			"branch":                 target["branchNameKana"],
			"bank":                   target["bankNameKana"],
			"beneficiaryName":        target["beneficiaryName"],
			"beneficiaryAccountNo":   target["beneficiaryAccountNo"],
			"beneficiaryAccountType": target["beneficiaryAccountType"],
			"amount":                 amount,
			"senderName":             target["beneficiaryName"], // FIXME
			"namebackFlag":           "Y",
			"moretimeFlag":           "1",
		},
	}
	var preconfirmRes struct {
		Preconfirm struct {
			Response map[string]string `json:"responseParam"`
		} `json:"preconfirm"`
	}
	err = a.query("IFTR_TransferAdapter", "registerPreconfirmation", &req, &preconfirmRes)
	if err != nil {
		return nil, err
	}

	var gridRes struct {
		Result struct {
			Response map[string]string `json:"responseParam"`
		} `json:"gridChallengeApiResponse"`
	}
	err = a.query("IFCM_CommonAdapter", "getCallengeGridPosition", nil, &gridRes)
	if err != nil {
		return nil, err
	}

	preconfirm := preconfirmRes.Preconfirm.Response
	amount, _ = strconv.ParseInt(preconfirm["amount"], 10, 0)
	fee, _ := strconv.ParseInt(preconfirm["fee"], 10, 0)
	return common.TransferStateMap{
		"target":     target,
		"request":    req["requestParam"],
		"preconfirm": preconfirm,
		"grid":       gridRes.Result.Response,
		"amount":     amount,
		"fee":        fee,
	}, nil
}

func (a *Account) CommitTransfer(tr common.TransferState, pass2 string) (string, error) {

	if a.secureGrid == nil {
		return "", fmt.Errorf("empty secure grid")
	}

	trmap := tr.(common.TransferStateMap)
	target := trmap["target"].(map[string]string)
	preconfirm := trmap["preconfirm"].(map[string]string)
	transfarReq := trmap["request"].(map[string]interface{})
	gridChallenge := trmap["grid"].(map[string]string)
	req := map[string]interface{}{
		"requestParam": map[string]interface{}{
			"beneficiaryAdd":         1,
			"senderName":             transfarReq["senderName"],
			"senderAccountNo":        transfarReq["senderAccountNo"],
			"beneficiaryName":        target["beneficiaryName"],
			"beneficiaryAccountNo":   target["beneficiaryAccountNo"],
			"beneficiaryAccountType": target["beneficiaryAccountType"],
			"bank":            target["bankNameKana"],
			"bankNameKanji":   target["bankNameKanji"],
			"bankCode":        target["bankCode"],
			"branch":          target["branchNameKana"],
			"branchNameKanji": target["branchNameKanji"],
			"branchCode":      target["branchCode"],

			"amount":                    preconfirm["amount"],
			"totalAmount":               preconfirm["totalAmount"],
			"fee":                       preconfirm["fee"],
			"valueDate":                 preconfirm["transactionDate"], // TODO
			"deleteBeneficiaryName":     "",
			"sessionRegistTime":         time.Now().UnixNano() / int64(time.Millisecond), // TODO
			"namebackFlag":              transfarReq["namebackFlag"],
			"moretimeFlag":              transfarReq["moretimeFlag"],
			"authenticationStatus":      "G",
			"userAgentInfo":             "test",
			"registeredBeneficiaryFlag": "Y",
			"pin": pass2,
			"gridChallengeValue1": a.getgrid(gridChallenge["challenge1"]),
			"gridChallengeValue2": a.getgrid(gridChallenge["challenge2"]),
			"gridChallengeValue3": a.getgrid(gridChallenge["challenge3"]),
		},
	}
	var confirmRes struct {
		Response struct {
			Param map[string]string `json:"responseParam"`
		} `json:"confirmApiResponse"`
	}
	err := a.query("IFTR_TransferAdapter", "registerConfirmation", &req, &confirmRes)
	if err != nil {
		return "", err
	}

	return confirmRes.Response.Param["txnReferenceNo"], nil
}

func (a *Account) GetAccountsBalanceAndActivity() error {
	var accountsRes struct {
		Overview struct {
			Response struct {
				TotalCreditBalance int64 `json:"totalCreditBalance,string"`
				TdBalance          int64 `json:"tdBalance,string"`
				SavingsBalance     int64 `json:"savingsBalance,string"`
			} `json:"responseParam"`
		} `json:"overview"`
		Activity struct {
			Response activityResponse `json:"responseParam"`
		} `json:"activity"`
	}
	err := a.query("IFTP_TopAdapter", "getAccountsBalanceAndActivity", nil, &accountsRes)
	if err != nil {
		return err
	}

	overview := accountsRes.Overview.Response
	a.balance = overview.TotalCreditBalance
	a.mainAccountNo = accountsRes.Activity.Response.AccountNo

	var trs []*common.Transaction
	for _, tr := range accountsRes.Activity.Response.ActivityDetails {
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
	// reverse
	for i, j := 0, len(trs)-1; i < j; i, j = i+1, j-1 {
		trs[i], trs[j] = trs[j], trs[i]
	}
	a.recentTransaction = trs
	return nil
}

func (a *Account) getgrid(pos string) string {
	return string(a.secureGrid[int(pos[1]-'0')][int(pos[0]-'A')])
}

func (a *Account) post(path string, params P) ([]byte, error) {
	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}

	req, err := http.NewRequest("POST", baseUrl+path, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
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
		return nil, err
	}
	defer res.Body.Close()

	b, err := ioutil.ReadAll(res.Body)
	return bytes.TrimSuffix(bytes.TrimPrefix(b, []byte("/*-secure-")), []byte("*/")), err
}

func (a *Account) init() error {
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

	err = json.Unmarshal(r, &res)
	if err != nil {
		return err
	}
	a.instanceID = res.Challenges.Realm["WL-Instance-Id"]
	return nil
}

func (a *Account) query(adapter, procedure string, req interface{}, res interface{}) error {
	parametersStr := "[]"
	if req != nil {
		reqJSON, err := json.Marshal(req)
		if err != nil {
			return err
		}
		parametersJSON, _ := json.Marshal([]string{string(reqJSON)})
		parametersStr = string(parametersJSON)
	}
	params := P{
		"adapter":    adapter,
		"procedure":  procedure,
		"parameters": parametersStr,
	}
	r, err := a.post("api/SFC/desktopbrowser/query", params)
	common.DebugLog("params: ", params)
	common.DebugLog("response:", string(r))
	if err != nil {
		return err
	}

	// get auth status.
	if _, ok := res.(*authStatusResponse); ok {
		return json.Unmarshal(r, res)
	}

	var result struct {
		IsSuccessful bool                   `json:"isSuccessful"`
		Response     json.RawMessage        `json:"responseParam"`
		Headers      map[string]interface{} `json:"header"`
		AuthInfo     map[string]interface{} `json:"WL-Authentication-Success,omitempty"`
	}
	err = json.Unmarshal(r, &result)
	if err != nil {
		return err
	}
	if !result.IsSuccessful {
		return fmt.Errorf("response.IsSuccessful=false")
	}
	if result.AuthInfo != nil {
		if realm, ok := result.AuthInfo["ShinseiAuthenticatorRealm"].(map[string]interface{}); ok {
			if attributes, ok := realm["attributes"].(map[string]interface{}); ok {
				if lastLoginTime, ok := attributes["lastLoginTime"].(string); ok {
					a.lastLogin, _ = time.Parse("2006/01/02 15:04:05", lastLoginTime)
				}
			}
		}
	}
	if token, ok := result.Headers["newToken"].(string); ok {
		a.csrfToken = token
	}
	if res != nil {
		err = json.Unmarshal(result.Response, res)
		if err != nil {
			return err
		}
	}
	return nil
}
