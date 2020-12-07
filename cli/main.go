package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"log"
	"math"
	"net/http"
)

func main() {
	nvm := vm.New()
	nvm.SetScriptGetter(func(hash util.Uint160) ([]byte, bool) {
		data := make(map[string]interface{})
		data["jsonrpc"] = "2.0"
		data["method"] = "getcontractstate"
		data["params"] = []interface{}{hash.StringBE(), 1}
		data["id"] = 1
		bytesData, err := json.Marshal(data)
		if err != nil {
			return nil, false
		}
		resp, err := http.Post(rpcaddr, "application/json", bytes.NewReader(bytesData))
		if err != nil {
			return nil, false
		}
		defer resp.Body.Close()
		decoder := json.NewDecoder(resp.Body)
		err = decoder.Decode(&data)
		if err != nil {
			return nil, false
		}

		sc, err := hex.DecodeString(data["result"].(map[string]interface{})["script"].(string))
		if err != nil {
			return nil, false
		}
		return sc, data["result"].(map[string]interface{})["properties"].(map[string]interface{})["dynamic_invoke"].(bool)
	})
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
					bytesData, err := json.Marshal(data)
					if err != nil {
						return err
					}
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
					jsonbytes, err := json.Marshal(data["result"])
					if err != nil {
						return err
					}
					err = blk.UnmarshalJSON(jsonbytes)
					if err != nil {
						return err
					}
					// cannot get the error for ic.bc.GetBlock(hash)
					// v.Estack().PushVal([]byte{})
					v.Estack().PushVal(vm.NewInteropItem(blk))
					return nil
				},
				Price: 200,
			}

		case vm.InteropNameToID([]byte("System.Blockchain.GetContract")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					hashbytes := v.Estack().Pop().Bytes()
					hash, err := util.Uint160DecodeBytesBE(hashbytes)
					if err != nil {
						return err
					}
					data := make(map[string]interface{})
					data["jsonrpc"] = "2.0"
					data["method"] = "getcontractstate"
					data["params"] = []interface{}{hash.StringBE(), 1}
					data["id"] = 1
					bytesData, err := json.Marshal(data)
					if err != nil {
						return err
					}
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
					cs := new(state.Contract)

					author, ok := data["result"].(map[string]interface{})["author"].(string)
					if ok == false {
						return err
					}
					cs.Author = author
					code_version, ok := data["result"].(map[string]interface{})["code_version"].(string)
					if ok == false {
						return err
					}
					cs.CodeVersion = code_version
					description, ok := data["result"].(map[string]interface{})["description"].(string)
					if ok == false {
						return err
					}
					cs.Description = description
					email, ok := data["result"].(map[string]interface{})["email"].(string)
					if ok == false {
						return err
					}
					cs.Email = email
					name, ok := data["result"].(map[string]interface{})["name"].(string)
					if ok == false {
						return err
					}
					cs.Name = name

					sc, err := hex.DecodeString(data["result"].(map[string]interface{})["script"].(string))
					if err != nil {
						return err
					}
					cs.Script = sc

					for _, item := range data["result"].(map[string]interface{})["parameters"].([]interface{}) {
						switch item {
						case "Signature":
							cs.ParamList = append(cs.ParamList, smartcontract.SignatureType)
						case "Boolean":
							cs.ParamList = append(cs.ParamList, smartcontract.BoolType)
						case "Integer":
							cs.ParamList = append(cs.ParamList, smartcontract.IntegerType)
						case "Hash160":
							cs.ParamList = append(cs.ParamList, smartcontract.Hash160Type)
						case "Hash256":
							cs.ParamList = append(cs.ParamList, smartcontract.Hash256Type)
						case "ByteArray":
							cs.ParamList = append(cs.ParamList, smartcontract.ByteArrayType)
						case "PublicKey":
							cs.ParamList = append(cs.ParamList, smartcontract.PublicKeyType)
						case "String":
							cs.ParamList = append(cs.ParamList, smartcontract.StringType)
						case "Array":
							cs.ParamList = append(cs.ParamList, smartcontract.ArrayType)
						case "Map":
							cs.ParamList = append(cs.ParamList, smartcontract.MapType)
						case "InteropInterface":
							cs.ParamList = append(cs.ParamList, smartcontract.InteropInterfaceType)
						case "Void":
							cs.ParamList = append(cs.ParamList, smartcontract.VoidType)
						}
					}

					switch data["result"].(map[string]interface{})["returntype"] {
					case "Signature":
						cs.ReturnType = smartcontract.SignatureType
					case "Boolean":
						cs.ReturnType = smartcontract.BoolType
					case "Integer":
						cs.ReturnType = smartcontract.IntegerType
					case "Hash160":
						cs.ReturnType = smartcontract.Hash160Type
					case "Hash256":
						cs.ReturnType = smartcontract.Hash256Type
					case "ByteArray":
						cs.ReturnType = smartcontract.ByteArrayType
					case "PublicKey":
						cs.ReturnType = smartcontract.PublicKeyType
					case "String":
						cs.ReturnType = smartcontract.StringType
					case "Array":
						cs.ReturnType = smartcontract.ArrayType
					case "Map":
						cs.ReturnType = smartcontract.MapType
					case "InteropInterface":
						cs.ReturnType = smartcontract.InteropInterfaceType
					case "Void":
						cs.ReturnType = smartcontract.VoidType
					}

					for key, val := range data["result"].(map[string]interface{})["properties"].(map[string]interface{}) {
						switch key {
						case "storage":
							if val == true {
								cs.Properties |= smartcontract.HasStorage
							}
						case "dynamic_invoke":
							if val == true {
								cs.Properties |= smartcontract.HasDynamicInvoke
							}
						}
					}

					//if err != nil {
					//	v.Estack().PushVal([]byte{})
					v.Estack().PushVal(vm.NewInteropItem(cs))
					return nil
				},
				Price: 1,
			}

		case vm.InteropNameToID([]byte("System.Blockchain.GetHeader")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					hash, err := getBlockHashFromElement(v.Estack().Pop())
					if err != nil {
						return err
					}

					data := make(map[string]interface{})
					data["jsonrpc"] = "2.0"
					data["method"] = "getheader"
					data["params"] = []interface{}{hash.StringBE(), 1}
					data["id"] = 1
					bytesData, err := json.Marshal(data)
					if err != nil {
						return err
					}
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

					hd := new(block.Header)
					jsonbytes, err := json.Marshal(data["result"])
					if err != nil {
						return err
					}
					err = hd.UnmarshalJSON(jsonbytes)
					if err != nil {
						return err
					}
					// cannot get the error for ic.bc.GetHeader(hash)
					// v.Estack().PushVal([]byte{})
					v.Estack().PushVal(vm.NewInteropItem(hd))
					return nil
				},
				Price: 100,
			}

		case vm.InteropNameToID([]byte("System.Blockchain.GetHeight")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {

					data := make(map[string]interface{})
					data["jsonrpc"] = "2.0"
					data["method"] = "getheight"
					data["params"] = []int{}
					data["id"] = 1
					bytesData, err := json.Marshal(data)
					if err != nil {
						return err
					}
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
					v.Estack().PushVal(data["reuslt"].(uint32))
					return nil
				},
				Price: 1,
			}

		case vm.InteropNameToID([]byte("System.Blockchain.GetTransaction")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					tx, _, err := getTransactionAndHeight(v)
					if err != nil {
						return err
					}
					v.Estack().PushVal(vm.NewInteropItem(tx))
					return nil
				},
				Price: 1,
			}

		case vm.InteropNameToID([]byte("System.Blockchain.GetTransactionHeight")):
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
			////return &vm.InteropFuncPrice{
			////Func: func(v *vm.VM) error {
			////	var res bool
			////	var err error
			////
			////	hashOrKey := v.Estack().Pop().Bytes()
			////	hash, err := util.Uint160DecodeBytesBE(hashOrKey)
			////	if err != nil {
			////		// We only accept compressed keys here as per C# implementation.
			////		if len(hashOrKey) != 33 {
			////			return errors.New("bad parameter length")
			////		}
			////		key := &keys.PublicKey{}
			////		err = key.DecodeBytes(hashOrKey)
			////		if err != nil {
			////			return errors.New("parameter given is neither a key nor a hash")
			////		}
			////		res, err = ic.checkKeyedWitness(key)
			////	} else {
			////		res, err = ic.checkHashedWitness(hash)
			////	}
			////	if err != nil {
			////		return gherr.Wrap(err, "failed to check")
			////	}
			////	v.Estack().PushVal(res)
			////	return nil
			////},
			//}
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
	script, err := hex.DecodeString("20d782db8a38b0eea0d7394e0f007c61c71798867578c77c387c08113903946cc9681a53797374656d2e426c6f636b636861696e2e476574426c6f636b")
	if err != nil {
		log.Fatalln(err)
	}
	nvm.LoadScript(script)
	err = nvm.Run()
	fmt.Println(err)
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
		bytesData, err := json.Marshal(data)
		if err != nil {
			return util.Uint256{0}, err
		}
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
		str = str[2:]
		return util.Uint256DecodeStringBE(str)
	} else {
		return util.Uint256DecodeBytesBE(hashbytes)
	}
}

func getTransactionAndHeight(v *vm.VM) (*transaction.Transaction, uint32, error) {
	//hashbytes := v.Estack().Pop().Bytes()
	//hash, err := util.Uint256DecodeBytesBE(hashbytes)
	//if err != nil {
	//	return nil, 0, err
	//}
	//
	//data := make(map[string]interface{})
	//data["jsonrpc"] = "2.0"
	//data["method"] = "getrawtransaction"
	//data["params"] = []interface{}{hash.StringBE(), 1}
	//data["id"] = 1
	//bytesData, err := json.Marshal(data)
	//
	//tx = new()
	//
	//if err != nil {
	//	return err
	//}
	//resp, err := http.Post(rpcaddr, "application/json", bytes.NewReader(bytesData))
	//if err != nil {
	//	return err
	//}
	//defer resp.Body.Close()
	//decoder := json.NewDecoder(resp.Body)
	//err = decoder.Decode(&data)
	//if err != nil {
	//	return err
	//}
	//
	//
	//
	//return cd.GetTransaction(hash)
}


var rpcaddr = "http://seed1.ngd.network:10332"
