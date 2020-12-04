package main

import (
	"fmt"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
)

func main() {
	var tx *transaction.Transaction
	// if count := len(scriptHashesForVerifying); count != 0 {
	// 	tx := new(transaction.Transaction)
	// 	tx.Attributes = make([]transaction.Attribute, count)
	// 	for i, a := range tx.Attributes {
	// 		a.Data = scriptHashesForVerifying[i].BytesBE()
	// 		a.Usage = transaction.Script
	// 	}
	// }
	chain := core.Blockchain{}
	vm := chain.GetTestVM(tx)
	vm.SetGasLimit(10)
	vm.LoadScript([]byte{0x00})
	err := vm.Run()
	fmt.Println(err)
	fmt.Println(vm.Estack())
}
