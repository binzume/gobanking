package banking

import (
	"testing"
	"time"

	"github.com/binzume/gobanking/common"
	"github.com/binzume/gobanking/mizuho"
	"github.com/binzume/gobanking/rakuten"
	"github.com/binzume/gobanking/sbi"
	"github.com/binzume/gobanking/shinsei"
	"github.com/binzume/gobanking/stub"
	"github.com/binzume/gobanking/utils"
)

func TestAccount(t *testing.T) {
	// dummy
	var _ common.Account = &mizuho.Account{}
	var _ common.Account = &rakuten.Account{}
	var _ common.Account = &sbi.Account{}
	var _ common.Account = &shinsei.Account{}
	var _ common.Account = &stub.Account{}

	utils.Debug = true

	acc, err := LoginWithJsonFile("examples/accounts/stub.json")
	if err != nil {
		t.Errorf("login failed %v", err)
	}

	t.Log("Account Info: ", acc.AccountInfo())

	lastLogin, err := acc.LastLogin()
	if err != nil {
		t.Errorf("failed to get last login: %v", err)
	}
	t.Log("Last login:", lastLogin)

	balance, err := acc.TotalBalance()
	if err != nil {
		t.Errorf("failed to get balabce: %v", err)
	}
	t.Log("Balance:", balance)

	t.Log("Recent: ")
	trs, err := acc.Recent()
	if err != nil {
		t.Errorf("failed to get recent activities: %v", err)
	}
	for _, tr := range trs {
		t.Log("  ", tr)
	}

	t.Log("History: ")
	trs, err = acc.History(time.Now().AddDate(0, 0, -14), time.Now())
	if err != nil {
		t.Errorf("failed to get history: %v", err)
	}
	for _, tr := range trs {
		t.Log("  ", tr)
	}

	err = acc.Logout()
	if err != nil {
		t.Errorf("failed to logout: %v", err)
	}
}
