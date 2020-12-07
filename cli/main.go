package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"math"
	"net/http"
)

func main() {
	nvm := vm.New()
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
	nvm.RegisterInteropGetter(func(id uint32) *vm.InteropFuncPrice {
		switch id {
		case vm.InteropNameToID([]byte("System.Block.GetTransaction")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					blockInterface := v.Estack().Pop().Value()
					block, ok := blockInterface.(*block.Block)
					if !ok {
						return errors.New("value is not a block")
					}
					index := v.Estack().Pop().BigInt().Int64()
					if index < 0 || index >= int64(len(block.Transactions)) {
						return errors.New("wrong transaction index")
					}
					tx := block.Transactions[index]
					v.Estack().PushVal(vm.NewInteropItem(tx))
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.Block.GetTransactionCount")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					blockInterface := v.Estack().Pop().Value()
					block, ok := blockInterface.(*block.Block)
					if !ok {
						return errors.New("value is not a block")
					}
					v.Estack().PushVal(len(block.Transactions))
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.Block.GetTransactions")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					blockInterface := v.Estack().Pop().Value()
					block, ok := blockInterface.(*block.Block)
					if !ok {
						return errors.New("value is not a block")
					}
					if len(block.Transactions) > vm.MaxArraySize {
						return errors.New("too many transactions")
					}
					txes := make([]vm.StackItem, 0, len(block.Transactions))
					for _, tx := range block.Transactions {
						txes = append(txes, vm.NewInteropItem(tx))
					}
					v.Estack().PushVal(txes)
					return nil
				},
				Price: 1,
			}

		case vm.InteropNameToID([]byte("System.Blockchain.GetBlock")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					hash, err := getBlockHashFromElement(v.Estack().Pop())
					if err != nil {
						return err
					}
					data := make(map[string]interface{})
					data["jsonrpc"] = "2.0"
					data["method"] = "getblock"
					data["params"] = []interface{}{hash.StringBE(), 1}
					data["id"] = 1
					bytesData, _ := json.Marshal(data)
					resp, err := http.Post(rpcaddr, "application/json", bytes.NewReader(bytesData))
					if err != nil {
						return err
					}
					defer resp.Body.Close()

					decoder := json.NewDecoder(resp.Body)
					err = decoder.Decode(&data)
					if err != nil {
						return err
					}
					blk := new(block.Block)
					bytes, err := json.Marshal(data["result"])
					if err != nil {
						return err
					}
					err = blk.UnmarshalJSON(bytes)
					if err != nil {
						return err
					}
					// cannot get the error for ic.bc.GetBlock(hash)
					// v.Estack().PushVal([]byte{})
					v.Estack().PushVal(vm.NewInteropItem(blk))
					return nil
				},
				Price: 1,
			}

		case vm.InteropNameToID([]byte("System.Blockchain.GetContract")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					hashbytes := v.Estack().Pop().Bytes()
					hash, err := util.Uint160DecodeBytesBE(hashbytes)
					fmt.Println(hash)
					if err != nil {
						return err
					}
					// TODO: call adhoccontractstate://hash-height/ here to fetch the contract State
					// cs, err := ic.dao.GetContractState(hash)
					if err != nil {
						v.Estack().PushVal([]byte{})
					} else {
						v.Estack().PushVal(1)
					}
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.Blockchain.GetHeader")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					hash, err := getBlockHashFromElement(v.Estack().Pop())
					fmt.Println(hash)
					if err != nil {
						return err
					}
					// TODO: call RPC to get the block by hash or Height
					// header, err := ic.bc.GetHeader(hash)
					if err != nil {
						v.Estack().PushVal([]byte{})
					} else {
						v.Estack().PushVal(1)
					}
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.Blockchain.GetHeight")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					// CALL RPC to get the height _is there available rpc the get the height?
					blockchainHeight, _ := Request("getheight", []int{})
					v.Estack().PushVal(blockchainHeight)
					return nil
				},
				Price: 1,
			}

		case vm.InteropNameToID([]byte("System.Blockchain.GetTransaction")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					tx, _, err := getTransactionAndHeight(v)
					fmt.Println(tx)
					if err != nil {
						return err
					}
					v.Estack().PushVal(1)
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.Blockchain.GetTransactionHeight")):
			//
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					_, h, err := getTransactionAndHeight(v)
					if err != nil {
						return err
					}
					v.Estack().PushVal(h)
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.Contract.Destroy")):
			return nil
		case vm.InteropNameToID([]byte("System.Contract.GetStorageContext")):
			return nil
		case vm.InteropNameToID([]byte("System.ExecutionEngine.GetCallingScriptHash")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					return vm.RuntimeSerialize(v)
				},
				Price: 1,
			}
			return nil
		case vm.InteropNameToID([]byte("System.ExecutionEngine.GetEntryScriptHash")):
			return nil
		case vm.InteropNameToID([]byte("System.ExecutionEngine.GetExecutingScriptHash")):
			return nil
		case vm.InteropNameToID([]byte("System.ExecutionEngine.GetScriptContainer")):
			return nil
		case vm.InteropNameToID([]byte("System.Header.GetHash")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					header, err := popHeaderFromVM(v)
					if err != nil {
						return err
					}
					v.Estack().PushVal(header.Hash().BytesBE())
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.Header.GetIndex")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					header, err := popHeaderFromVM(v)
					if err != nil {
						return err
					}
					v.Estack().PushVal(header.Index)
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.Header.GetPrevHash")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					header, err := popHeaderFromVM(v)
					if err != nil {
						return err
					}
					v.Estack().PushVal(header.PrevHash.BytesBE())
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.Header.GetTimestamp")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					header, err := popHeaderFromVM(v)
					if err != nil {
						return err
					}
					v.Estack().PushVal(header.Timestamp)
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.Runtime.CheckWitness")):
			return nil
		case vm.InteropNameToID([]byte("System.Runtime.Deserialize")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					return vm.RuntimeDeserialize(v)
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.Runtime.GetTime")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					// User current
					// we will get the header by call rpc
					// first GetTheCurrentHeight
					height, _ := Request("getheight", []int{})
					// header://height/{UINT64}
					header, _ := Request("getheader", []int{height.ProtoMajor})
					fmt.Println(header)
					v.Estack().PushVal(1)
					return nil
				},
				Price: 1,
			}
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
					return vm.RuntimeSerialize(v)
				},
				Price: 1,
			}

		case vm.InteropNameToID([]byte("System.Storage.Delete")):
			return nil
		case vm.InteropNameToID([]byte("System.Storage.Get")):
			//return &vm.InteropFuncPrice{
			//	Func: func(v *vm.VM) error {
			//		stcInterface := v.Estack().Pop().Value()
			//		stc, ok := stcInterface.(*StorageContext)
			//		if !ok {
			//		return fmt.Errorf("%T is not a StorageContext", stcInterface)
			//		}
			//		contract, err := GetContractState(stc.ScriptHash) // CALL RPC
			//		if err != nil {
			//		return errors.New("no contract found")
			//		}
			//		if !contract.HasStorage() {
			//		return fmt.Errorf("contract %s can't use storage", stc.ScriptHash)
			//		}
			//		key := v.Estack().Pop().Bytes()
			//		FIRST READ CACHE
			//		THEN RPC
			//		si := GetStorageItem(stc.ScriptHash, key) // CALL RPC
			//		if si != nil && si.Value != nil {
			//		v.Estack().PushVal(si.Value)
			//		} else {
			//		v.Estack().PushVal([]byte{})
			//		}
			//		return nil
			//	},
			//	Price: 0, // TODO FIX
			//}
			return nil
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
	nvm.SetGasLimit(10)
	nvm.LoadScript([]byte{0x51, 0x52})
	err := nvm.Run()
	fmt.Println(err)
	fmt.Println(nvm.Estack().ToContractParameters())
}

var storage map[[32]byte][]byte

// vm -gaslimit 50 -xx 123 -script 21479823793892943849498

func getBlockHashFromElement(element *vm.Element) (util.Uint256, error) {
	var hash util.Uint256
	hashbytes := element.Bytes()
	if len(hashbytes) <= 5 {
		hashint := element.BigInt().Int64()
		if hashint < 0 || hashint > math.MaxUint32 {
			return hash, errors.New("bad block index")
		}
		data := make(map[string]interface{})
		data["jsonrpc"] = "2.0"
		data["method"] = "getblockhash"
		data["params"] = []interface{}{hashint, 1}
		data["id"] = 1
		bytesData, _ := json.Marshal(data)
		resp, err := http.Post(rpcaddr, "application/json", bytes.NewReader(bytesData))
		if err != nil {
			return util.Uint256{0}, err
		}
		defer resp.Body.Close()
		decoder := json.NewDecoder(resp.Body)
		err = decoder.Decode(&data)
		if err != nil {
			return util.Uint256{0}, err
		}
		str, ok := data["result"].(string)
		if ok == false {
			return util.Uint256{0}, errors.New("error")
		}
		return util.Uint256DecodeStringLE(str)
	} else {
		return util.Uint256DecodeBytesBE(hashbytes)
	}
}

var rpcaddr = "http://seed1.ngd.network:10332"
