package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"log"
	"math"
	"net/http"
	"strings"
)

func main() {
	nvm := vm.New()
	nvm.SetPriceGetter(getPrice)
	nvm.SetScriptGetter(func(hash util.Uint160) ([]byte, bool) {
		data := make(map[string]interface{})
		data["jsonrpc"] = "2.0"
		data["method"] = "Data.GetContractByHashHeightInHex"
		data["params"] = []interface{}{map[string]interface{}{"Hash": hash.StringLE(), "Height": height}}
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
					data["method"] = "Data.GetBlockByHashInHex"
					data["params"] = map[string]interface{}{"Hash": hash.StringLE()}
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
					log.Println(data)
					blk := new(block.Block)
					b, err := hex.DecodeString(data["result"].(string))
					if err != nil {
						return err
					}
					reader := io.NewBinReaderFromBuf(b)
					blk.DecodeBinary(reader)
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
					data["method"] = "Data.GetContractByHashHeightInHex"
					data["params"] = []interface{}{map[string]interface{}{"Hash": hash.StringLE(), "Height": height}}
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

					// we must maintain a cd here.

					//if err != nil {
					//	v.Estack().PushVal([]byte{})
					v.Estack().PushVal(vm.NewInteropItem(cs))
					return nil
				},
				Price: 100,
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
					data["method"] = "Data.GetHeaderByHashInHex"
					data["params"] = map[string]interface{}{"Hash": hash.StringLE()}
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
					b, err := hex.DecodeString(data["result"].(string))
					if err != nil {
						return err
					}
					reader := io.NewBinReaderFromBuf(b)
					hd.DecodeBinary(reader)
					v.Estack().PushVal(vm.NewInteropItem(hd))
					return nil
				},
				Price: 100,
			}

		case vm.InteropNameToID([]byte("System.Blockchain.GetHeight")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					v.Estack().PushVal(height)
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
				Price: 200,
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
				Price: 100,
			}

		case vm.InteropNameToID([]byte("System.Contract.Destroy")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					// TODO : to be implemented.
					return nil
				},
				Price: 1,
			}

		case vm.InteropNameToID([]byte("System.Contract.GetStorageContext")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					csInterface := v.Estack().Pop().Value()
					cs, ok := csInterface.(*state.Contract)
					if !ok {
						return fmt.Errorf("%T is not a contract state", cs)
					}
					// TODO: to be checked.
					stc := &StorageContext{
						ScriptHash: cs.ScriptHash(),
					}
					v.Estack().PushVal(vm.NewInteropItem(stc))
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.ExecutionEngine.GetCallingScriptHash")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					return pushContextScriptHash(v, 1)
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.ExecutionEngine.GetEntryScriptHash")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					return pushContextScriptHash(v, v.Istack().Len()-1)
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.ExecutionEngine.GetExecutingScriptHash")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					return pushContextScriptHash(v, 0)
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.ExecutionEngine.GetScriptContainer")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					// TODO: tx have not been generated.
					// v.Estack().PushVal(vm.NewInteropItem(ic.tx))
					return nil
				},
			}
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
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					var err error
					var hash util.Uint160

					hashOrKey := v.Estack().Pop().Bytes()
					hash, err = util.Uint160DecodeBytesBE(hashOrKey)
					if err != nil {
						// We only accept compressed keys here as per C# implementation.
						if len(hashOrKey) != 33 {
							return errors.New("bad parameter length")
						}
						key := &keys.PublicKey{}
						err = key.DecodeBytes(hashOrKey)
						if err != nil {
							return errors.New("parameter given is neither a key nor a hash")
						}
						hash = key.GetScriptHash()
					}
					if _, ok := witnesses[hash]; ok {
						v.Estack().PushVal(true)
					} else {
						v.Estack().PushVal(false)
					}
					return nil
				},
				Price: 200,
			}
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
					data := make(map[string]interface{})
					data["jsonrpc"] = "2.0"
					data["method"] = "Data.GetHeaderByHeightInHex"
					data["params"] = map[string]interface{}{"Index": height}
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
					b, err := hex.DecodeString(data["result"].(string))
					if err != nil {
						return err
					}
					reader := io.NewBinReaderFromBuf(b)
					hd.DecodeBinary(reader)
					v.Estack().PushVal(hd.Timestamp)
					return nil
				},
				Price: 1,
			}
			return nil

		case vm.InteropNameToID([]byte("System.Runtime.GetTrigger")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					// TODO : to be implemented
					v.Estack().PushVal(byte(trigger.Application))
					return nil
				},
				Price: 1,
			}
			return nil

		case vm.InteropNameToID([]byte("System.Runtime.Log")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					v.Estack().Pop().Bytes()
					return nil
				},
			}
			return nil
		case vm.InteropNameToID([]byte("System.Runtime.Notify")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					v.Estack().Pop()
					return nil
				},
				Price: 1,
			}

		case vm.InteropNameToID([]byte("System.Runtime.Platform")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					v.Estack().PushVal([]byte("NEO"))
					return nil
				},
				Price: 1,
			}

		case vm.InteropNameToID([]byte("System.Runtime.Serialize")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					return vm.RuntimeSerialize(v)
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.Storage.Delete")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					stcInterface := v.Estack().Pop().Value()
					stc, ok := stcInterface.(*StorageContext)
					if !ok {
						return fmt.Errorf("%T is not a StorageContext", stcInterface)
					}
					if stc.ReadOnly {
						return errors.New("StorageContext is read only")
					}
					// TODO: check
					key := v.Estack().Pop().Bytes()

					sc := hex.EncodeToString(stc.ScriptHash.BytesBE()) + hex.EncodeToString(key)
					storage[sc] = []byte{}
					return nil
				},
				Price: 100,
			}
		case vm.InteropNameToID([]byte("System.Storage.Get")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					stcInterface := v.Estack().Pop().Value()
					stc, ok := stcInterface.(*StorageContext)
					if !ok {
						return fmt.Errorf("%T is not a StorageContext", stcInterface)
					}
					// then get online
					// TODO: to be checked
					key := v.Estack().Pop().Bytes()
					// fetch from storage first
					sc := hex.EncodeToString(stc.ScriptHash.BytesBE()) + hex.EncodeToString(key)

					if ret, ok := storage[sc]; ok {
						v.Estack().PushVal(ret)
					} else {
						// query from remote
						data := make(map[string]interface{})
						data["jsonrpc"] = "2.0"
						data["method"] = "Data.GetStorageByDBKeyHeightInHex"
						data["params"] = []interface{}{map[string]interface{}{"DBKey": sc, "Height": height}}
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
						scremote, err := hex.DecodeString(data["result"].(string))
						if err != nil {
							return err
						}
						v.Estack().PushVal(scremote)

					}
					return nil
				},
				Price: 100,
			}
		case vm.InteropNameToID([]byte("System.Storage.GetContext")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					sc := &StorageContext{
						ScriptHash: getContextScriptHash(v, 0),
						ReadOnly:   false,
					}
					v.Estack().PushVal(vm.NewInteropItem(sc))
					return nil
				},
				Price: 1,
			}

		case vm.InteropNameToID([]byte("System.Storage.GetReadOnlyContext")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					sc := &StorageContext{
						ScriptHash: getContextScriptHash(v, 0),
						ReadOnly:   true,
					}
					v.Estack().PushVal(vm.NewInteropItem(sc))
					return nil
				},
				Price: 1,
			}

		case vm.InteropNameToID([]byte("System.Storage.Put")):
			// put into local storage
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					stcInterface := v.Estack().Pop().Value()
					stc, ok := stcInterface.(*StorageContext)
					if !ok {
						return fmt.Errorf("%T is not a StorageContext", stcInterface)
					}
					key := v.Estack().Pop().Bytes()
					value := v.Estack().Pop().Bytes()
					sc := hex.EncodeToString(stc.ScriptHash.BytesBE()) + hex.EncodeToString(key)
					storage[sc] = value
					return nil
				},
				Price: 1000,
			}

		case vm.InteropNameToID([]byte("System.Storage.PutEx")):
			// put into local storage
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					stcInterface := v.Estack().Pop().Value()
					stc, ok := stcInterface.(*StorageContext)
					if !ok {
						return fmt.Errorf("%T is not a StorageContext", stcInterface)
					}
					key := v.Estack().Pop().Bytes()
					value := v.Estack().Pop().Bytes()
					sc := hex.EncodeToString(stc.ScriptHash.BytesBE()) + hex.EncodeToString(key)
					// TODO: fix
					v.Estack().Pop().BigInt().Int64()
					storage[sc] = value
					return nil
				},
				Price: 1000,
			}

		case vm.InteropNameToID([]byte("System.StorageContext.AsReadOnly")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					stcInterface := v.Estack().Pop().Value()
					stc, ok := stcInterface.(*StorageContext)
					if !ok {
						return fmt.Errorf("%T is not a StorageContext", stcInterface)
					}
					if !stc.ReadOnly {
						stx := &StorageContext{
							ScriptHash: stc.ScriptHash,
							ReadOnly:   true,
						}
						stc = stx
					}
					v.Estack().PushVal(vm.NewInteropItem(stc))
					return nil
				},
				Price: 1,
			}

		case vm.InteropNameToID([]byte("System.Transaction.GetHash")):
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					txInterface := v.Estack().Pop().Value()
					tx, ok := txInterface.(*transaction.Transaction)
					if !ok {
						return errors.New("value is not a transaction")
					}
					v.Estack().PushVal(tx.Hash().BytesBE())
					return nil
				},
				Price: 1,
			}
		}
		return nil
	})
	nvm.SetGasLimit(util.Fixed8(gaslimit))
	nvm.LoadScript(script)
	err := nvm.Run()
	if err != nil {
		log.Fatalln(err)
	}
	result := map[string]interface{}{
		"script":       hex.EncodeToString(script),
		"state":        nvm.State(),
		"gas_consumed": nvm.GasConsumed(),
		"stack":        nvm.Estack().ToContractParameters(),
	}
	res, err := json.Marshal(result)
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(string(res))
}

func init() {
	var hexscript string
	var wits string
	flag.StringVar(&hexscript, "script", "", "scriptHexFormat")
	flag.Int64Var(&gaslimit, "gaslimit", 50000000000, "gaslimit")
	flag.StringVar(&rpcaddr, "rpc", "", "rpcaddr")
	flag.StringVar(&wits, "wits", "", "witnesses")
	flag.Parse()

	storage = make(map[string][]byte)
	witnesses = make(map[util.Uint160]struct{})
	for _, v := range strings.Split(wits, ":") {
		if len(v) == 0 {
			continue
		}
		sc, err := util.Uint160DecodeStringBE(v)
		if err != nil {
			log.Fatalln(err)
		}
		witnesses[sc] = struct{}{}
	}

	data := make(map[string]interface{})
	data["jsonrpc"] = "2.0"
	data["method"] = "Data.GetCountInUInt64"
	data["params"] = map[string]interface{}{}
	data["id"] = 1
	bytesData, err := json.Marshal(data)
	if err != nil {
		log.Fatalln(err)
	}
	resp, err := http.Post(rpcaddr, "application/json", bytes.NewReader(bytesData))
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&data)
	if err != nil {
		log.Fatalln(err)
	}
	height = uint32(data["result"].(float64))
	script, err = hex.DecodeString(hexscript)
	if err != nil {
		log.Fatalln(err)
	}
}

var script []byte
var gaslimit int64
var storage map[string][]byte
var witnesses map[util.Uint160]struct{}
var height uint32
var rpcaddr string

const interopGasRatio = 100000

type StorageContext struct {
	ScriptHash util.Uint160
	ReadOnly   bool
}

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
		data["method"] = "Data.GetHashByHeightInHex"
		data["params"] = map[string]interface{}{"Index": uint32(hashint)}
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
		return util.Uint256DecodeStringBE(str)
	} else {
		return util.Uint256DecodeBytesBE(hashbytes)
	}
}

func getTransactionAndHeight(v *vm.VM) (*transaction.Transaction, uint32, error) {
	hashbytes := v.Estack().Pop().Bytes()
	hash, err := util.Uint256DecodeBytesBE(hashbytes)
	if err != nil {
		return nil, 0, err
	}

	data := make(map[string]interface{})
	data["jsonrpc"] = "2.0"
	data["method"] = "getrawtransaction"
	// query json here
	data["params"] = []interface{}{hash.StringBE(), 1}
	data["id"] = 1
	bytesData, err := json.Marshal(data)

	resp, err := http.Post(rpcaddr, "application/json", bytes.NewReader(bytesData))
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	tx := new(transaction.Transaction)
	jsonbytes, err := json.Marshal(data["result"])

	if err != nil {
		return nil, 0, err
	}
	err = tx.UnmarshalJSON(jsonbytes)
	if err != nil {
		return nil, 0, err
	}
	return tx, 0, err
}

func getContextScriptHash(v *vm.VM, n int) util.Uint160 {
	ctxIface := v.Istack().Peek(n).Value()
	ctx := ctxIface.(*vm.Context)
	return ctx.ScriptHash()
}

func pushContextScriptHash(v *vm.VM, n int) error {
	h := getContextScriptHash(v, n)
	v.Estack().PushVal(h.BytesBE())
	return nil
}

func popHeaderFromVM(v *vm.VM) (*block.Header, error) {
	iface := v.Estack().Pop().Value()
	header, ok := iface.(*block.Header)
	if !ok {
		block, ok := iface.(*block.Block)
		if !ok {
			return nil, errors.New("value is not a header or block")
		}
		return block.Header(), nil
	}
	return header, nil
}

func getPrice(v *vm.VM, op opcode.Opcode, parameter []byte) util.Fixed8 {
	if op <= opcode.NOP {
		return 0
	}

	switch op {
	case opcode.APPCALL, opcode.TAILCALL:
		return toFixed8(10)
	case opcode.SYSCALL:
		interopID := vm.GetInteropID(parameter)
		return getSyscallPrice(v, interopID)
	case opcode.SHA1, opcode.SHA256:
		return toFixed8(10)
	case opcode.HASH160, opcode.HASH256:
		return toFixed8(20)
	case opcode.CHECKSIG, opcode.VERIFY:
		return toFixed8(100)
	case opcode.CHECKMULTISIG:
		estack := v.Estack()
		if estack.Len() == 0 {
			return toFixed8(1)
		}

		var cost int

		item := estack.Peek(0)
		switch item.Item().(type) {
		case *vm.ArrayItem, *vm.StructItem:
			cost = len(item.Array())
		default:
			cost = int(item.BigInt().Int64())
		}

		if cost < 1 {
			return toFixed8(1)
		}

		return toFixed8(int64(100 * cost))
	default:
		return toFixed8(1)
	}
}

func toFixed8(n int64) util.Fixed8 {
	return util.Fixed8(n * interopGasRatio)
}

func getSyscallPrice(v *vm.VM, id uint32) util.Fixed8 {
	ifunc := v.GetInteropByID(id)
	if ifunc != nil && ifunc.Price > 0 {
		return toFixed8(int64(ifunc.Price))
	}

	const (
		neoAssetCreate           = 0x1fc6c583 // Neo.Asset.Create
		antSharesAssetCreate     = 0x99025068 // AntShares.Asset.Create
		neoAssetRenew            = 0x71908478 // Neo.Asset.Renew
		antSharesAssetRenew      = 0xaf22447b // AntShares.Asset.Renew
		neoContractCreate        = 0x6ea56cf6 // Neo.Contract.Create
		neoContractMigrate       = 0x90621b47 // Neo.Contract.Migrate
		antSharesContractCreate  = 0x2a28d29b // AntShares.Contract.Create
		antSharesContractMigrate = 0xa934c8bb // AntShares.Contract.Migrate
		systemStoragePut         = 0x84183fe6 // System.Storage.Put
		systemStoragePutEx       = 0x3a9be173 // System.Storage.PutEx
		neoStoragePut            = 0xf541a152 // Neo.Storage.Put
		antSharesStoragePut      = 0x5f300a9e // AntShares.Storage.Put
	)

	estack := v.Estack()

	switch id {
	case neoAssetCreate, antSharesAssetCreate:
		return util.Fixed8FromInt64(5000)
	case neoAssetRenew, antSharesAssetRenew:
		arg := estack.Peek(1).BigInt().Int64()
		return util.Fixed8FromInt64(arg * 5000)
	case neoContractCreate, neoContractMigrate, antSharesContractCreate, antSharesContractMigrate:
		return smartcontract.GetDeploymentPrice(smartcontract.PropertyState(estack.Peek(3).BigInt().Int64()))
	case systemStoragePut, systemStoragePutEx, neoStoragePut, antSharesStoragePut:
		// price for storage PUT is 1 GAS per 1 KiB
		keySize := len(estack.Peek(1).Bytes())
		valSize := len(estack.Peek(2).Bytes())
		return util.Fixed8FromInt64(int64((keySize+valSize-1)/1024 + 1))
	default:
		return util.Fixed8FromInt64(1)
	}
}
