package main

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/vm"
)

func main() {
	v := vm.New()
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
	v.RegisterInteropGetter(func(id uint32) *vm.InteropFuncPrice {
		switch id {
		case vm.InteropNameToID([]byte("System.Block.GetTransaction")):
			return nil
		case vm.InteropNameToID([]byte("System.Block.GetTransaction")):
			return nil
		case vm.InteropNameToID([]byte("System.Block.GetTransactionCount")):
			return nil
		case vm.InteropNameToID([]byte("System.Block.GetTransactions")):
			return nil
		case vm.InteropNameToID([]byte("System.Blockchain.GetBlock")):
			return nil
		case vm.InteropNameToID([]byte("System.Blockchain.GetContract")):
			return nil
		case vm.InteropNameToID([]byte("System.Blockchain.GetHeader")):
			return nil
		case vm.InteropNameToID([]byte("System.Blockchain.GetHeight")):
			return nil
		case vm.InteropNameToID([]byte("System.Blockchain.GetTransaction")):
			return nil
		case vm.InteropNameToID([]byte("System.Blockchain.GetTransactionHeight")):
			return nil
		case vm.InteropNameToID([]byte("System.Contract.Destroy")):
			return nil
		case vm.InteropNameToID([]byte("System.Contract.GetStorageContext")):
			return nil
		case vm.InteropNameToID([]byte("System.ExecutionEngine.GetCallingScriptHash")):
			return nil
		case vm.InteropNameToID([]byte("System.ExecutionEngine.GetEntryScriptHash")):
			return nil
		case vm.InteropNameToID([]byte("System.ExecutionEngine.GetExecutingScriptHash")):
			return nil
		case vm.InteropNameToID([]byte("System.ExecutionEngine.GetScriptContainer")):
			return nil
		case vm.InteropNameToID([]byte("System.Header.GetHash")):
			return nil
		case vm.InteropNameToID([]byte("System.Header.GetIndex")):
			return nil
		case vm.InteropNameToID([]byte("System.Header.GetPrevHash")):
			return nil
		case vm.InteropNameToID([]byte("System.Header.GetTimestamp")):
			return nil
		case vm.InteropNameToID([]byte("System.Runtime.CheckWitness")):
			return nil
		case vm.InteropNameToID([]byte("System.Runtime.Deserialize")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					data := vm.Estack().Pop().Bytes()
					item, err := DeserializeItem(data)
					if err != nil {
						return err
					}
					vm.Estack().Push(&Element{value: item})
					return nil
				},
				Price: 0,
			}

		case vm.InteropNameToID([]byte("System.Runtime.GetTime")):
			return nil
		case vm.InteropNameToID([]byte("System.Runtime.GetTrigger")):
			return nil
		case vm.InteropNameToID([]byte("System.Runtime.Log")):
			return nil
		case vm.InteropNameToID([]byte("System.Runtime.Notify")):
			return nil

		case vm.InteropNameToID([]byte("System.Runtime.Platform")):
			return nil

		case vm.InteropNameToID([]byte("System.Runtime.Serialize")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					item := vm.Estack().Pop()
					data, err := SerializeItem(item.value)
					if err != nil {
						return err
					} else if len(data) > MaxItemSize {
						return errors.New("too big item")
					}
					vm.Estack().PushVal(data)
					return nil
				},
				Price: 0,
			}

		case vm.InteropNameToID([]byte("System.Storage.Delete")):
			return nil
		case vm.InteropNameToID([]byte("System.Storage.Get")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					// stcInterface := v.Estack().Pop().Value()
					// stc, ok := stcInterface.(*StorageContext)
					// if !ok {
					// return fmt.Errorf("%T is not a StorageContext", stcInterface)
					// }
					// contract, err := GetContractState(stc.ScriptHash) // CALL RPC
					// if err != nil {
					// return errors.New("no contract found")
					// }
					// if !contract.HasStorage() {
					// return fmt.Errorf("contract %s can't use storage", stc.ScriptHash)
					// }
					// key := v.Estack().Pop().Bytes()
					// FIRST READ CACHE
					// THEN RPC
					// si := GetStorageItem(stc.ScriptHash, key) // CALL RPC
					// if si != nil && si.Value != nil {
					// v.Estack().PushVal(si.Value)
					// } else {
					// v.Estack().PushVal([]byte{})
					// }
					return nil
				},
				Price: 0, // TODO FIX
			}
		case vm.InteropNameToID([]byte("System.Storage.GetContext")):
			return nil
		case vm.InteropNameToID([]byte("System.Storage.GetReadOnlyContext")):
			return nil
		case vm.InteropNameToID([]byte("System.Storage.Put")):
			return nil
		case vm.InteropNameToID([]byte("System.Storage.PutEx")):
			return nil
		case vm.InteropNameToID([]byte("System.StorageContext.AsReadOnly")):
			return nil
		case vm.InteropNameToID([]byte("System.Transaction.GetHash")):
			return nil
		}
		return nil
	})
	v.SetGasLimit(10)
	v.LoadScript([]byte{0x51, 0x52})
	err := v.Run()
	fmt.Println(err)
	fmt.Println(v.Estack().ToContractParameters())
}

var storage map[[32]byte][]byte

// vm -gaslimit 50 -xx 123 -script 21479823793892943849498
