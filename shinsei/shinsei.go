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
	"github.com/binzume/gobanking/utils"
)

type Account struct {
	common.BankAccount

	client *http.Client

	auth          string
	csrfToken     string
	mainAccountNo string

	balance           int64
	fundBalance       int64
	lastLogin         time.Time
	recentTransaction []*common.Transaction

	customerNameKana string

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
	Res struct {
		AuthStatus string      `json:"authStatus"`
		Token      string      `json:"token"`
		Error      interface{} `json:"errorMessage"`
	} `json:"responseJSON"`
}

type securityConnectResponse struct {
	UserID     string                 `json:"userId"`
	Attributes map[string]interface{} `json:"attributes"`
}

const BankCode = "0397"
const BankName = "新生銀行"
const baseUrl = "https://bk.web.shinseibank.com/SFC/app/"

type P map[string]string

func Login(id, password string, options map[string]interface{}) (*Account, error) {
	client, err := utils.NewHttpClient()
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

	r, err := a.postForm("ShinseiAuthenticatorRealm/login_auth_request_url", P{
		"fldUserID":     id,
		"password":      password,
		"langCode":      "JAP",
		"mode":          "1",
		"postubFlag":    "0",
		"jsc":           "_",
		"userAgentInfo": utils.UserAgent,
	})
	if err != nil {
		return err
	}
	var res authStatusResponse
	err = json.Unmarshal(r, &res)
	if err != nil {
		return err
	}
	if res.Res.AuthStatus != "success" {
		return fmt.Errorf("invalid authStatus: %v", res.Res.AuthStatus)
	}
	a.csrfToken = res.Res.Token

	var securityConnectRes securityConnectResponse
	err = a.rawQuery("IFCM_CommonAdapter", "securityConnect", nil, &securityConnectRes)
	if err != nil {
		return err
	}
	if securityConnectRes.UserID == "" {
		return fmt.Errorf("invalid response: %v", securityConnectRes)
	}
	if lastLoginTime, ok := securityConnectRes.Attributes["lastLoginTime"].(string); ok {
		a.lastLogin, _ = time.Parse("2006/01/02 15:04:05", lastLoginTime)
	}

	err = a.query("IFCM_CommonAdapter", "validateToken", nil, nil)
	if err != nil {
		return err
	}

	a.BranchCode = id[0:3]
	a.AccountNum = id[3:]
	return a.GetAccountsBalanceAndActivity()
}

func (a *Account) Logout() error {
	_, err := a.postForm("ShinseiAuthenticatorRealm/logout_request_url", P{})
	return err
}

func (a *Account) AccountInfo() *common.BankAccount {
	return &a.BankAccount
}

func (a *Account) TotalBalance() (int64, error) {
	return a.balance + a.fundBalance, nil
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
	req := P{
		"accountNo": a.mainAccountNo,
		"type":      typ,
		"fromDate":  fromStr,
		"toDate":    toStr,
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

	utils.DebugLog("target: ", target)

	req := map[string]interface{}{
		"senderAccountNo":        a.mainAccountNo,
		"senderName":             a.customerNameKana,
		"branch":                 target["branchNameKana"],
		"bank":                   target["bankNameKana"],
		"beneficiaryName":        target["beneficiaryName"],
		"beneficiaryAccountNo":   target["beneficiaryAccountNo"],
		"beneficiaryAccountType": target["beneficiaryAccountType"],
		"amount":                 amount,
		"namebackFlag":           "Y",
		"moretimeFlag":           "1",
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
	return utils.TransferStateMap{
		"target":     target,
		"request":    req,
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

	trmap := tr.(utils.TransferStateMap)
	target := trmap["target"].(map[string]string)
	preconfirm := trmap["preconfirm"].(map[string]string)
	transfarReq := trmap["request"].(map[string]interface{})
	gridChallenge := trmap["grid"].(map[string]string)
	req := map[string]interface{}{
		"beneficiaryAdd":         1,
		"senderName":             transfarReq["senderName"],
		"senderAccountNo":        transfarReq["senderAccountNo"],
		"beneficiaryName":        target["beneficiaryName"],
		"beneficiaryAccountNo":   target["beneficiaryAccountNo"],
		"beneficiaryAccountType": target["beneficiaryAccountType"],
		"bank":                   target["bankNameKana"],
		"bankNameKanji":          target["bankNameKanji"],
		"bankCode":               target["bankCode"],
		"branch":                 target["branchNameKana"],
		"branchNameKanji":        target["branchNameKanji"],
		"branchCode":             target["branchCode"],

		"amount":                    preconfirm["amount"],
		"totalAmount":               preconfirm["totalAmount"],
		"fee":                       preconfirm["fee"],
		"valueDate":                 preconfirm["transactionDate"], // TODO
		"deleteBeneficiaryName":     "",
		"sessionRegistTime":         time.Now().UnixNano() / int64(time.Millisecond), // TODO
		"namebackFlag":              transfarReq["namebackFlag"],
		"moretimeFlag":              transfarReq["moretimeFlag"],
		"authenticationStatus":      "G",
		"userAgentInfo":             utils.UserAgent,
		"registeredBeneficiaryFlag": "Y",
		"pin":                       pass2,
		"gridChallengeValue1":       a.getgrid(gridChallenge["challenge1"]),
		"gridChallengeValue2":       a.getgrid(gridChallenge["challenge2"]),
		"gridChallengeValue3":       a.getgrid(gridChallenge["challenge3"]),
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

func extractResponseParam(res map[string]interface{}) (interface{}, error) {
	if res == nil || res["responseParam"] == nil {
		return nil, fmt.Errorf("No response: %v", res)
	}
	if e, ok := res["errorInfo"].(map[string]interface{}); ok && len(e) > 0 {
		if e["statusMessage"] != "SUCCESS" {
			return res["responseParam"], fmt.Errorf("Error: %v", e)
		}
	}
	return res["responseParam"], nil
}

func findAccount(accounts []map[string]interface{}, cur string) map[string]interface{} {
	for _, a := range accounts {
		if a["currency"].(string) == cur {
			return a
		}
	}
	return nil
}

func (a *Account) NewFxTransfer(fromCur, toCur string, amount float32, pin string) (common.TransferState, error) {

	err := a.query("IFCM_CommonAdapter", "validateToken", nil, nil)
	if err != nil {
		return nil, err
	}

	var accountListRes struct {
		Overview struct {
			Param struct {
				SavingDetails []map[string]interface{} `json:"savingsDetails"`
			} `json:"responseParam"`
		} `json:"accountOverviewAPIParm"`
	}

	err = a.query("IFCM_CommonAdapter", "getAccountInformationListDisplay", P{"getPatternFlg": ""}, &accountListRes)
	if err != nil {
		return nil, err
	}
	var fromAccount = findAccount(accountListRes.Overview.Param.SavingDetails, fromCur)
	if fromAccount == nil {
		return nil, fmt.Errorf("No account for %v", fromCur)
	}
	var toAccount = findAccount(accountListRes.Overview.Param.SavingDetails, toCur)
	if toAccount == nil {
		return nil, fmt.Errorf("No account for %v", toAccount)
	}

	err = a.query("IFCM_CommonAdapter", "checkAuthenticationStatus", P{"pin": pin}, nil)
	if err != nil {
		return nil, err
	}

	params := map[string]interface{}{
		"amount":            amount,
		"creditAccountNo":   toAccount["accountNo"],
		"creditCcyCode":     toAccount["currency"],
		"creditProductCode": toAccount["productCode"],
		"debitAccountNo":    fromAccount["accountNo"],
		"debitCcyCode":      fromAccount["currency"],
		"debitProductCode":  fromAccount["productCode"],
	}

	tr := utils.TransferStateMap{
		"params": params,
		"from":   fromAccount,
		"to":     toAccount,
	}
	return tr, a.UpdateFxTransfer(tr)
}

func (a *Account) UpdateFxTransfer(tr common.TransferState) error {
	trmap := tr.(utils.TransferStateMap)

	var confirmRes map[string]map[string]interface{}
	err := a.query("IFFD_FxAdapter", "confirmPreRegistrationForeignCurrencyDeposits", trmap["params"], &confirmRes)
	if err != nil {
		return err
	}

	res, err := extractResponseParam(confirmRes["fxBuySellPreconfirmAPIParam"])
	if err != nil {
		return err
	}

	if r, ok := res.(map[string]interface{}); ok {
		trmap["result"] = r
		trmap["exchangeRate"] = r["exchangeRate"]
		trmap["buySpread"] = r["buySpread"]
		trmap["convertedAmount"] = r["convertedAmount"]
		return nil
	}
	return fmt.Errorf("Unexpected response %#v", confirmRes)
}

func (a *Account) CommitFxTransfer(tr common.TransferState) (string, error) {
	trmap := tr.(utils.TransferStateMap)
	params := trmap["params"].(map[string]interface{})
	params["exchangeRate"] = trmap["exchangeRate"]

	err := a.query("IFFD_FxAdapter", "registerForeignCurrencyDeposits", params, nil)
	return "", err
}

func (a *Account) GetAccountsBalanceAndActivity() error {
	var summaryRes struct {
		Summary struct {
			Param struct {
				FxCasaBalance  int64 `json:"fxCasaBalance,string"`
				SavingsBalance int64 `json:"savingsBalance,string"`
				YenTDBalance   int64 `json:"yenTDBalance,string"`
				TotalCredit    int64 `json:"totalCredit,string"`

				CustomerName      string `json:"customerName"`
				CustomerNameKana  string `json:"customerNameKana"`
				CustomerNameKanji string `json:"customerNameKanji"`
			} `json:"responseParam"`
		} `json:"summary"`
		FundBalance struct {
			Param struct {
				YenEqui int64 `json:"yenEqui,string"`
			} `json:"responseParam"`
		} `json:"mutualFundBalance"`
		Branch struct {
			Param struct {
				BranchName string `json:"branchName"`
				BranchCode string `json:"branchCode"`
			} `json:"responseParam"`
		} `json:"branchFetch"`
	}
	err := a.query("IFTP_TopAdapter", "getBalanceSummaryAndStage", nil, &summaryRes)
	if err != nil {
		return err
	}

	a.balance = summaryRes.Summary.Param.TotalCredit
	a.fundBalance = summaryRes.FundBalance.Param.YenEqui
	a.customerNameKana = summaryRes.Summary.Param.CustomerNameKana

	a.BankCode = BankCode
	a.BankName = BankName
	a.BranchName = summaryRes.Branch.Param.BranchName
	a.OwnerName = summaryRes.Summary.Param.CustomerName

	var accountsRes struct {
		Activity struct {
			Response activityResponse `json:"responseParam"`
		} `json:"activity"`
	}
	err = a.query("IFTP_TopAdapter", "getAccountsBalanceAndActivity", nil, &accountsRes)
	if err != nil {
		return err
	}

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

func (a *Account) post(path, reqBody, contentType string) ([]byte, error) {
	req, err := http.NewRequest("POST", baseUrl+path, strings.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Referer", baseUrl)
	if a.auth != "" {
		req.Header.Set("authorization", a.auth)
	}
	if a.csrfToken != "" {
		req.Header.Set("x-csrf-token", a.csrfToken)
	}

	res, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.Header.Get("authorization") != "" {
		a.auth = res.Header.Get("authorization")
	}

	b, err := ioutil.ReadAll(res.Body)
	utils.DebugLog("params: ", reqBody)
	utils.DebugLog("response:", string(b))
	return bytes.TrimSuffix(bytes.TrimPrefix(b, []byte("/*-secure-")), []byte("*/")), err
}

func (a *Account) postForm(path string, params P) ([]byte, error) {
	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}
	return a.post(path, values.Encode(), "application/x-www-form-urlencoded; charset=UTF-8")
}

func (a *Account) postJson(path string, reqJson string) ([]byte, error) {
	return a.post(path, reqJson, "application/json; charset=UTF-8")
}

func (a *Account) rawQuery(adapter, procedure string, req interface{}, res interface{}) error {
	parametersStr := ""
	if req != nil {
		reqJSON, err := json.Marshal(req)
		if err != nil {
			return err
		}
		parametersStr = string(reqJSON)
	}

	r, err := a.postJson(adapter+"/"+procedure, parametersStr)
	if err != nil {
		return err
	}
	return json.Unmarshal(r, res)
}

func (a *Account) query(adapter, procedure string, req interface{}, res interface{}) error {
	var result struct {
		Response *json.RawMessage       `json:"responseParam"`
		Headers  map[string]interface{} `json:"header"`
		AuthInfo map[string]interface{} `json:"WL-Authentication-Success,omitempty"`
	}
	err := a.rawQuery(adapter, procedure, map[string]interface{}{"requestParam": req}, &result)
	if err != nil {
		return err
	}
	if token, ok := result.Headers["newToken"].(string); ok {
		a.csrfToken = token
	}
	if res != nil {
		if result.Response == nil {
			return fmt.Errorf("Unexpected response %v", result)
		}
		err = json.Unmarshal(*result.Response, res)
		if err != nil {
			return err
		}
	}
	return nil
}
