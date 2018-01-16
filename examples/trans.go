// +build ignore

package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/binzume/gobanking"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Println(" Usage: go run trans.go accounts/stub.json targetname 10000")
		return
	}
	target := os.Args[2]
	amount, err := strconv.Atoi(os.Args[3])
	if err != nil {
		fmt.Println(" Usage: go run trans.go accounts/stub.json targetname 10000")
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
		fmt.Println("LastLogin() error.", err)
	}
	fmt.Println("Last Login:", lastLogin)

	// Print balance.
	total, err := acc.TotalBalance()
	if err != nil {
		fmt.Println("TotalBalance() error.", err)
	}
	fmt.Println("Balance:", total)

	tr, err := acc.NewTransferToRegisteredAccount(target, int64(amount))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(tr)

	fmt.Print("Enter pass: ")
	reader := bufio.NewReader(os.Stdin)
	pass, _ := reader.ReadString('\n')
	pass = strings.TrimSpace(pass)
	if pass == "" {
		fmt.Println("canceled")
		return
	}

	recptId, err := acc.CommitTransfer(tr, pass)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("ok. recptId:", recptId)
}
