package main

import (
	"testing"

	"./common"
	"./mizuho"
	"./rakuten"
	"./sbi"
	"./shinsei"
	"./stub"
)

func TestAccount(t *testing.T) {
	// dummy
	var _ common.Account = &mizuho.Account{}
	var _ common.Account = &rakuten.Account{}
	var _ common.Account = &sbi.Account{}
	var _ common.Account = &shinsei.Account{}
	var _ common.Account = &stub.Account{}
}
