package banking

import (
	"testing"

	"github.com/binzume/gobanking/common"
	"github.com/binzume/gobanking/mizuho"
	"github.com/binzume/gobanking/rakuten"
	"github.com/binzume/gobanking/sbi"
	"github.com/binzume/gobanking/shinsei"
	"github.com/binzume/gobanking/stub"
)

func TestAccount(t *testing.T) {
	// dummy
	var _ common.Account = &mizuho.Account{}
	var _ common.Account = &rakuten.Account{}
	var _ common.Account = &sbi.Account{}
	var _ common.Account = &shinsei.Account{}
	var _ common.Account = &stub.Account{}
	_, err := LoginWithJsonFile("examples/accounts/stub.json")
	if err != nil {
		t.Errorf("login failed %v", err)
	}
}
