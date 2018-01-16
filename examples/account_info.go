// +build ignore

package main

import (
	"fmt"
	"os"

	"github.com/binzume/gobanking"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println(" Usage: go run account_info.go accounts/stub.json")
		return
	}

	// Login with json file.
	acc, err := banking.LoginWithJsonFile(os.Args[1])
	if err != nil {
		fmt.Println("Login error.", err)
		return
	}
	defer acc.Logout()

	lastLogin, err := acc.LastLogin()
	if err != nil {
		fmt.Println("LastLogin error.", err)
	}
	fmt.Println("Last Login:", lastLogin)

	// Print balance.
	total, err := acc.TotalBalance()
	if err != nil {
		fmt.Println("TotalBalance error.", err)
	}
	fmt.Println("Balance:", total)

	// Print recent logs.
	trs, err := acc.Recent()
	if err != nil {
		fmt.Println("Get recent history error.", err)
	}
	fmt.Println("--------")
	for _, tr := range trs {
		fmt.Println(tr.Date, tr.Amount, tr.Description, tr.Balance)
	}
	fmt.Println("--------")

	// Print History.
	/*
		trs, err = acc.History(time.Now().Add(-time.Hour*24*60), time.Now().Add(-time.Hour*24*20))
		if err != nil {
			log.Fatal(err)
		}
		for _, tr := range trs {
			fmt.Println(tr.Date, tr.Amount, tr.Description, tr.Balance)
		}
	*/
}
