package main

import (
	"fmt"
	"github.com/nspcc-dev/neo-go/pkg/vm"
)

func main() {
	vm := vm.New()
	// vm.SetScriptGetter(func(hash util.Uint160) ([]byte, bool) {
	// 	cs, err := ic.dao.GetContractState(hash)
	// 	if err != nil {
	// 		return nil, false
	// 	}
	// 	hasDynamicInvoke := (cs.Properties & smartcontract.HasDynamicInvoke) != 0
	// 	return cs.Script, hasDynamicInvoke
	// })
	// vm.RegisterInteropGetter(ic.getSystemInterop)
	// vm.RegisterInteropGetter(ic.getNeoInterop)
	// if ic.bc != nil && ic.bc.GetConfig().EnableStateRoot {
		// vm.RegisterInteropGetter(ic.getNeoxInterop)
	// }
	
	vm.SetGasLimit(10)
	vm.LoadScript([]byte{0x51,0x52,})
	err := vm.Run()
	fmt.Println(err)
	fmt.Println(vm.Estack().ToContractParameters())
}
