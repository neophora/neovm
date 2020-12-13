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
	"math/rand"
	"net/http"
	"strings"
)

func main() {
	nvm := vm.New()
	nvm.SetPriceGetter(getPrice)
	nvm.SetScriptGetter(func(hash util.Uint160) ([]byte, bool) {
		data := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      rand.Uint32(),
			"method":  "get_contract_by_hash_height_in_hex",
			"params":  map[string]interface{}{"Hash": hash.StringBE(), "Height": height},
		}
		log.Println("[REQ]", data)
		resp := mERR(http.Post(rpcaddr, "application/json", bytes.NewReader(mERR(json.Marshal(data)).([]byte)))).(*http.Response)
		defer resp.Body.Close()
		mCHK(json.NewDecoder(resp.Body).Decode(&data))
		log.Println("[RESP]", data)
		cs := new(state.Contract)
		cs.DecodeBinary(io.NewBinReaderFromBuf(mERR(hex.DecodeString(data["result"].(string))).([]byte)[1:]))
		log.Println("[CONTRACT]", hash)
		return cs.Script, (cs.Properties & smartcontract.HasDynamicInvoke) != 0
	})

	nvm.RegisterInteropGetter(func(id uint32) *vm.InteropFuncPrice {
		switch id {
		case vm.InteropNameToID([]byte("System.Block.GetTransaction")):
			log.Println("[SYSCALL]", "System.Block.GetTransaction")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					block := v.Estack().Pop().Value().(*block.Block)
					index := v.Estack().Pop().BigInt().Int64()
					tx := block.Transactions[index]
					v.Estack().PushVal(vm.NewInteropItem(tx))
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.Block.GetTransactionCount")):
			log.Println("[SYSCALL]", "System.Block.GetTransactionCount")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					block := v.Estack().Pop().Value().(*block.Block)
					v.Estack().PushVal(len(block.Transactions))
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.Block.GetTransactions")):
			log.Println("[SYSCALL]", "System.Block.GetTransactions")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					block := v.Estack().Pop().Value().(*block.Block)
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
			log.Println("[SYSCALL]", "System.Blockchain.GetBlock")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					hash := mERR(getBlockHashFromElement(v.Estack().Pop())).(util.Uint256)
					data := map[string]interface{}{
						"jsonrpc": "2.0",
						"id":      rand.Uint32(),
						"method":  "get_block_by_hash_in_hex",
						"params":  map[string]interface{}{"Hash": hash.StringLE()},
					}
					log.Println("[REQ]", data)
					resp := mERR(http.Post(rpcaddr, "application/json", bytes.NewReader(mERR(json.Marshal(data)).([]byte)))).(*http.Response)
					defer resp.Body.Close()
					mCHK(json.NewDecoder(resp.Body).Decode(&data))
					log.Println("[RESP]", data)
					blk := new(block.Block)
					blk.DecodeBinary(io.NewBinReaderFromBuf(mERR(hex.DecodeString(data["result"].(string))).([]byte)))
					v.Estack().PushVal(vm.NewInteropItem(blk))
					return nil
				},
				Price: 200,
			}
		case vm.InteropNameToID([]byte("System.Blockchain.GetContract")):
			log.Println("[SYSCALL]", "System.Blockchain.GetContract")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					hash := mERR(util.Uint160DecodeBytesBE(v.Estack().Pop().Bytes())).(util.Uint160)
					data := map[string]interface{}{
						"jsonrpc": "2.0",
						"id":      rand.Uint32(),
						"method":  "get_contract_by_hash_height_in_hex",
						"params":  map[string]interface{}{"Hash": hash.StringBE(), "Height": height},
					}
					log.Println("[REQ]", data)
					resp := mERR(http.Post(rpcaddr, "application/json", bytes.NewReader(mERR(json.Marshal(data)).([]byte)))).(*http.Response)
					defer resp.Body.Close()
					mCHK(json.NewDecoder(resp.Body).Decode(&data))
					log.Println("[RESP]", data)
					cs := new(state.Contract)
					cs.DecodeBinary(io.NewBinReaderFromBuf(mERR(hex.DecodeString(data["result"].(string))).([]byte)[1:]))
					v.Estack().PushVal(vm.NewInteropItem(cs))
					return nil
				},
				Price: 100,
			}
		case vm.InteropNameToID([]byte("System.Blockchain.GetHeader")):
			log.Println("[SYSCALL]", "System.Blockchain.GetHeader")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					hash := mERR(getBlockHashFromElement(v.Estack().Pop())).(util.Uint256)
					data := map[string]interface{}{
						"jsonrpc": "2.0",
						"id":      rand.Uint32(),
						"method":  "get_header_by_hash_in_hex",
						"params":  map[string]interface{}{"Hash": hash.StringLE()},
					}
					log.Println("[REQ]", data)
					resp := mERR(http.Post(rpcaddr, "application/json", bytes.NewReader(mERR(json.Marshal(data)).([]byte)))).(*http.Response)
					defer resp.Body.Close()
					mCHK(json.NewDecoder(resp.Body).Decode(&data))
					log.Println("[RESP]", data)
					hd := new(block.Header)
					hd.DecodeBinary(io.NewBinReaderFromBuf(mERR(hex.DecodeString(data["result"].(string))).([]byte)))
					v.Estack().PushVal(vm.NewInteropItem(hd))
					return nil
				},
				Price: 100,
			}
		case vm.InteropNameToID([]byte("System.Blockchain.GetHeight")):
			log.Println("[SYSCALL]", "System.Blockchain.GetHeight")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					v.Estack().PushVal(height)
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.Blockchain.GetTransaction")):
			log.Println("[SYSCALL]", "System.Blockchain.GetTransaction")
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
			log.Println("[SYSCALL]", "System.Blockchain.GetTransactionHeight")
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
			log.Println("[SYSCALL]", "System.Contract.Destroy")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					// TODO : IMPL
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.Contract.GetStorageContext")):
			log.Println("[SYSCALL]", "System.Contract.GetStorageContext")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					cs := v.Estack().Pop().Value().(*state.Contract)
					// TODO: CHECK
					stc := &StorageContext{
						ScriptHash: cs.ScriptHash(),
					}
					v.Estack().PushVal(vm.NewInteropItem(stc))
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.ExecutionEngine.GetCallingScriptHash")):
			log.Println("[SYSCALL]", "System.ExecutionEngine.GetCallingScriptHash")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					return pushContextScriptHash(v, 1)
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.ExecutionEngine.GetEntryScriptHash")):
			log.Println("[SYSCALL]", "System.ExecutionEngine.GetEntryScriptHash")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					return pushContextScriptHash(v, v.Istack().Len()-1)
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.ExecutionEngine.GetExecutingScriptHash")):
			log.Println("[SYSCALL]", "System.ExecutionEngine.GetExecutingScriptHash")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					return pushContextScriptHash(v, 0)
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.ExecutionEngine.GetScriptContainer")):
			log.Println("[SYSCALL]", "System.ExecutionEngine.GetScriptContainer")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					// TODO: IMPL
					return nil
				},
			}
		case vm.InteropNameToID([]byte("System.Header.GetHash")):
			log.Println("[SYSCALL]", "System.Header.GetHash")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					header := mERR(popHeaderFromVM(v)).(*block.Header)
					v.Estack().PushVal(header.Hash().BytesBE())
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.Header.GetIndex")):
			log.Println("[SYSCALL]", "System.Header.GetIndex")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					header := mERR(popHeaderFromVM(v)).(*block.Header)
					v.Estack().PushVal(header.Index)
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.Header.GetPrevHash")):
			log.Println("[SYSCALL]", "System.Header.GetPrevHash")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					header := mERR(popHeaderFromVM(v)).(*block.Header)
					v.Estack().PushVal(header.PrevHash.BytesBE())
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.Header.GetTimestamp")):
			log.Println("[SYSCALL]", "System.Header.GetTimestamp")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					header := mERR(popHeaderFromVM(v)).(*block.Header)
					v.Estack().PushVal(header.Timestamp)
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.Runtime.CheckWitness")):
			log.Println("[SYSCALL]", "System.Runtime.CheckWitness")
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
			log.Println("[SYSCALL]", "System.Runtime.Deserialize")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					return vm.RuntimeDeserialize(v)
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.Runtime.GetTime")):
			log.Println("[SYSCALL]", "System.Runtime.GetTime")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					data := map[string]interface{}{
						"jsonrpc": "2.0",
						"id":      rand.Uint32(),
						"method":  "get_header_by_height_in_hex",
						"params":  map[string]interface{}{"Height": height},
					}
					log.Println("[REQ]", data)
					resp := mERR(http.Post(rpcaddr, "application/json", bytes.NewReader(mERR(json.Marshal(data)).([]byte)))).(*http.Response)
					defer resp.Body.Close()
					mCHK(json.NewDecoder(resp.Body).Decode(&data))
					log.Println("[RESP]", data)
					hd := new(block.Header)
					hd.DecodeBinary(io.NewBinReaderFromBuf(mERR(hex.DecodeString(data["result"].(string))).([]byte)))
					v.Estack().PushVal(hd.Timestamp)
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.Runtime.GetTrigger")):
			log.Println("[SYSCALL]", "System.Runtime.GetTrigger")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					v.Estack().PushVal(byte(trigger.Application))
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.Runtime.Log")):
			log.Println("[SYSCALL]", "System.Runtime.Log")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					v.Estack().Pop().Bytes()
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.Runtime.Notify")):
			log.Println("[SYSCALL]", "System.Runtime.Notify")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					v.Estack().Pop()
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.Runtime.Platform")):
			log.Println("[SYSCALL]", "System.Runtime.Platform")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					v.Estack().PushVal([]byte("NEO"))
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.Runtime.Serialize")):
			log.Println("[SYSCALL]", "System.Runtime.Serialize")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					return vm.RuntimeSerialize(v)
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("System.Storage.Delete")):
			log.Println("[SYSCALL]", "System.Storage.Delete")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					stc := v.Estack().Pop().Value().(*StorageContext)
					if stc.ReadOnly {
						return errors.New("StorageContext is read only")
					}
					// TODO: CHECK
					key := v.Estack().Pop().Bytes()
					sc := hex.EncodeToString(stc.ScriptHash.BytesBE()) + hex.EncodeToString(key)
					storage[sc] = []byte{}
					return nil
				},
				Price: 100,
			}
		case vm.InteropNameToID([]byte("System.Storage.Get")):
			log.Println("[SYSCALL]", "System.Storage.Get")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					stc := v.Estack().Pop().Value().(*StorageContext)
					key := v.Estack().Pop().Bytes()
					sc := hex.EncodeToString(stc.ScriptHash.BytesBE()) + hex.EncodeToString(key)
					if ret, ok := storage[sc]; ok {
						v.Estack().PushVal(ret)
					} else {
						data := map[string]interface{}{
							"jsonrpc": "2.0",
							"id":      rand.Uint32(),
							"method":  "get_storage_by_dbkey_height_in_hex",
							"params":  map[string]interface{}{"DBKey": sc, "Height": height},
						}
						log.Println("[REQ]", data)
						resp := mERR(http.Post(rpcaddr, "application/json", bytes.NewReader(mERR(json.Marshal(data)).([]byte)))).(*http.Response)
						defer resp.Body.Close()
						mCHK(json.NewDecoder(resp.Body).Decode(&data))
						log.Println("[RESP]", data)
						val := mERR(hex.DecodeString(data["result"].(string))).([]byte)
						v.Estack().PushVal(val)
					}
					return nil
				},
				Price: 100,
			}
		case vm.InteropNameToID([]byte("System.Storage.GetContext")):
			log.Println("[SYSCALL]", "System.Storage.GetContext")
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
			log.Println("[SYSCALL]", "System.Storage.GetReadOnlyContext")
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
			log.Println("[SYSCALL]", "System.Storage.Put")
			// put into local storage
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					stc := v.Estack().Pop().Value().(*StorageContext)
					key := v.Estack().Pop().Bytes()
					value := v.Estack().Pop().Bytes()
					sc := hex.EncodeToString(stc.ScriptHash.BytesBE()) + hex.EncodeToString(key)
					storage[sc] = value
					return nil
				},
				Price: 1000,
			}
		case vm.InteropNameToID([]byte("System.Storage.PutEx")):
			log.Println("[SYSCALL]", "System.Storage.PutEx")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					stc := v.Estack().Pop().Value().(*StorageContext)
					key := v.Estack().Pop().Bytes()
					value := v.Estack().Pop().Bytes()
					sc := hex.EncodeToString(stc.ScriptHash.BytesBE()) + hex.EncodeToString(key)
					// TODO: IMPL
					v.Estack().Pop().BigInt().Int64()
					storage[sc] = value
					return nil
				},
				Price: 1000,
			}
		case vm.InteropNameToID([]byte("System.StorageContext.AsReadOnly")):
			log.Println("[SYSCALL]", "System.StorageContext.AsReadOnly")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					stc := v.Estack().Pop().Value().(*StorageContext)
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
			log.Println("[SYSCALL]", "System.Transaction.GetHash")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					tx := v.Estack().Pop().Value().(*transaction.Transaction)
					v.Estack().PushVal(tx.Hash().BytesBE())
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Account.GetBalance")):
			log.Println("[SYSCALL]", "Neo.Account.GetBalance")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					acc := v.Estack().Pop().Value().(*state.Account)
					asbytes := v.Estack().Pop().Bytes()
					ashash := mERR(util.Uint256DecodeBytesBE(asbytes)).(util.Uint256)
					balance, ok := acc.GetBalanceValues()[ashash]
					if !ok {
						balance = util.Fixed8(0)
					}
					v.Estack().PushVal(int64(balance))
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Account.GetScriptHash")):
			log.Println("[SYSCALL]", "Neo.Account.GetScriptHash")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					acc := v.Estack().Pop().Value().(*state.Account)
					v.Estack().PushVal(acc.ScriptHash.BytesBE())
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Account.GetVotes")):
			log.Println("[SYSCALL]", "Neo.Account.GetVotes")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					acc := v.Estack().Pop().Value().(*state.Account)
					votes := make([]vm.StackItem, 0, len(acc.Votes))
					for _, key := range acc.Votes {
						votes = append(votes, vm.NewByteArrayItem(key.Bytes()))
					}
					v.Estack().PushVal(votes)
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Account.IsStandard")):
			log.Println("[SYSCALL]", "Neo.Account.IsStandard")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					accbytes := v.Estack().Pop().Bytes()
					acchash := mERR(util.Uint160DecodeBytesBE(accbytes)).(util.Uint160)
					data := map[string]interface{}{
						"jsonrpc": "2.0",
						"id":      rand.Uint32(),
						"method":  "get_contract_by_hash_height_in_hex",
						"params":  map[string]interface{}{"Hash": acchash, "Height": height},
					}
					log.Println("[REQ]", data)
					resp := mERR(http.Post(rpcaddr, "application/json", bytes.NewReader(mERR(json.Marshal(data)).([]byte)))).(*http.Response)
					defer resp.Body.Close()
					mCHK(json.NewDecoder(resp.Body).Decode(&data))
					log.Println("[RESP]", data)
					cs := new(state.Contract)
					cs.DecodeBinary(io.NewBinReaderFromBuf(mERR(hex.DecodeString(data["result"].(string))).([]byte)[1:]))
					res := len(cs.Script) == 0 || vm.IsStandardContract(cs.Script)
					v.Estack().PushVal(res)
					return nil
				},
				Price: 100,
			}
		case vm.InteropNameToID([]byte("Neo.Asset.Create")):
			log.Println("[SYSCALL]", "Neo.Asset.Create")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					// TODO : IMPL
					return nil
				},
				Price: 0,
			}
		case vm.InteropNameToID([]byte("Neo.Asset.GetAdmin")):
			log.Println("[SYSCALL]", "Neo.Asset.GetAdmin")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					as := v.Estack().Pop().Value().(*state.Asset)
					v.Estack().PushVal(as.Admin.BytesBE())
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Asset.GetAmount")):
			log.Println("[SYSCALL]", "Neo.Asset.GetAmount")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					as := v.Estack().Pop().Value().(*state.Asset)
					v.Estack().PushVal(int64(as.Amount))
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Asset.GetAssetId")):
			log.Println("[SYSCALL]", "Neo.Asset.GetAssetId")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					as := v.Estack().Pop().Value().(*state.Asset)
					v.Estack().PushVal(as.ID.BytesBE())
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Asset.GetAssetType")):
			log.Println("[SYSCALL]", "Neo.Asset.GetAssetType")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					as := v.Estack().Pop().Value().(*state.Asset)
					v.Estack().PushVal(int(as.AssetType))
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Asset.GetAvailable")):
			log.Println("[SYSCALL]", "Neo.Asset.GetAvailable")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					as := v.Estack().Pop().Value().(*state.Asset)
					v.Estack().PushVal(int(as.Available))
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Asset.GetIssuer")):
			log.Println("[SYSCALL]", "Neo.Asset.GetIssuer")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					as := v.Estack().Pop().Value().(*state.Asset)
					v.Estack().PushVal(as.Issuer.BytesBE())
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Asset.GetOwner")):
			log.Println("[SYSCALL]", "Neo.Asset.GetOwner")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					as := v.Estack().Pop().Value().(*state.Asset)
					v.Estack().PushVal(as.Owner.Bytes())
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Asset.GetPrecision")):
			log.Println("[SYSCALL]", "Neo.Asset.GetPrecision")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					as := v.Estack().Pop().Value().(*state.Asset)
					v.Estack().PushVal(int(as.Precision))
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Asset.Renew")):
			log.Println("[SYSCALL]", "Neo.Asset.Renew")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					// TODO: IMPL
					return nil
				},
				Price: 0,
			}
		case vm.InteropNameToID([]byte("Neo.Attribute.GetData")):
			log.Println("[SYSCALL]", "Neo.Attribute.GetData")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					attr := v.Estack().Pop().Value().(*transaction.Attribute)
					v.Estack().PushVal(attr.Data)
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Attribute.GetUsage")):
			log.Println("[SYSCALL]", "Neo.Attribute.GetUsage")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					attr := v.Estack().Pop().Value().(*transaction.Attribute)
					v.Estack().PushVal(int(attr.Usage))
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Block.GetTransaction")):
			log.Println("[SYSCALL]", "Neo.Block.GetTransaction")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					block := v.Estack().Pop().Value().(*block.Block)
					index := v.Estack().Pop().BigInt().Int64()
					tx := block.Transactions[index]
					v.Estack().PushVal(vm.NewInteropItem(tx))
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Block.GetTransactionCount")):
			log.Println("[SYSCALL]", "Neo.Block.GetTransactionCount")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					block := v.Estack().Pop().Value().(*block.Block)
					v.Estack().PushVal(len(block.Transactions))
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Block.GetTransactions")):
			log.Println("[SYSCALL]", "Neo.Block.GetTransactions")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					block := v.Estack().Pop().Value().(*block.Block)
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
		case vm.InteropNameToID([]byte("Neo.Blockchain.GetAccount")):
			log.Println("[SYSCALL]", "Neo.Blockchain.GetAccount")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					// TODO: IMPL
					return nil
				},
				Price: 100,
			}
		case vm.InteropNameToID([]byte("Neo.Blockchain.GetAsset")):
			log.Println("[SYSCALL]", "Neo.Blockchain.GetAsset")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					// TODO: IMPL
					return nil
				},
				Price: 100,
			}
		case vm.InteropNameToID([]byte("Neo.Blockchain.GetBlock")):
			log.Println("[SYSCALL]", "Neo.Blockchain.GetBlock")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					hash := mERR(getBlockHashFromElement(v.Estack().Pop())).(util.Uint256)
					data := map[string]interface{}{
						"jsonrpc": "2.0",
						"id":      rand.Uint32(),
						"method":  "get_block_by_hash_in_hex",
						"params":  map[string]interface{}{"Hash": hash.StringLE()},
					}
					log.Println("[REQ]", data)
					resp := mERR(http.Post(rpcaddr, "application/json", bytes.NewReader(mERR(json.Marshal(data)).([]byte)))).(*http.Response)
					defer resp.Body.Close()
					mCHK(json.NewDecoder(resp.Body).Decode(&data))
					log.Println("[RESP]", data)
					blk := new(block.Block)
					blk.DecodeBinary(io.NewBinReaderFromBuf(mERR(hex.DecodeString(data["result"].(string))).([]byte)))
					v.Estack().PushVal(vm.NewInteropItem(blk))
					return nil
				},
				Price: 200,
			}
		case vm.InteropNameToID([]byte("Neo.Blockchain.GetContract")):
			log.Println("[SYSCALL]", "Neo.Blockchain.GetContract")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					hash := mERR(util.Uint160DecodeBytesBE(v.Estack().Pop().Bytes())).(util.Uint160)
					data := map[string]interface{}{
						"jsonrpc": "2.0",
						"id":      rand.Uint32(),
						"method":  "get_contract_by_hash_height_in_hex",
						"params":  map[string]interface{}{"Hash": hash.StringBE(), "Height": height},
					}
					log.Println("[REQ]", data)
					resp := mERR(http.Post(rpcaddr, "application/json", bytes.NewReader(mERR(json.Marshal(data)).([]byte)))).(*http.Response)
					defer resp.Body.Close()
					mCHK(json.NewDecoder(resp.Body).Decode(&data))
					log.Println("[RESP]", data)
					cs := new(state.Contract)
					cs.DecodeBinary(io.NewBinReaderFromBuf(mERR(hex.DecodeString(data["result"].(string))).([]byte)[1:]))
					v.Estack().PushVal(vm.NewInteropItem(cs))
					return nil
				},
				Price: 100,
			}
		case vm.InteropNameToID([]byte("Neo.Blockchain.GetHeader")):
			log.Println("[SYSCALL]", "Neo.Blockchain.GetHeader")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					hash := mERR(getBlockHashFromElement(v.Estack().Pop())).(util.Uint256)
					data := map[string]interface{}{
						"jsonrpc": "2.0",
						"id":      rand.Uint32(),
						"method":  "get_header_by_hash_in_hex",
						"params":  map[string]interface{}{"Hash": hash.StringLE()},
					}
					log.Println("[REQ]", data)
					resp := mERR(http.Post(rpcaddr, "application/json", bytes.NewReader(mERR(json.Marshal(data)).([]byte)))).(*http.Response)
					defer resp.Body.Close()
					mCHK(json.NewDecoder(resp.Body).Decode(&data))
					log.Println("[RESP]", data)
					hd := new(block.Header)
					hd.DecodeBinary(io.NewBinReaderFromBuf(mERR(hex.DecodeString(data["result"].(string))).([]byte)))
					v.Estack().PushVal(vm.NewInteropItem(hd))
					return nil
				},
				Price: 100,
			}
		case vm.InteropNameToID([]byte("Neo.Blockchain.GetHeight")):
			log.Println("[SYSCALL]", "Neo.Blockchain.GetHeight")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					v.Estack().PushVal(height)
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Blockchain.GetTransaction")):
			log.Println("[SYSCALL]", "Neo.Blockchain.GetTransaction")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					tx, _, err := getTransactionAndHeight(v)
					if err != nil {
						return err
					}
					v.Estack().PushVal(vm.NewInteropItem(tx))
					return nil
				},
				Price: 100,
			}
		case vm.InteropNameToID([]byte("Neo.Blockchain.GetTransactionHeight")):
			log.Println("[SYSCALL]", "Neo.Blockchain.GetTransactionHeight")
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
		case vm.InteropNameToID([]byte("Neo.Blockchain.GetValidators")):
			log.Println("[SYSCALL]", "Neo.Blockchain.GetValidators")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					// TODO: IMPL
					return nil
				},
				Price: 200,
			}
		case vm.InteropNameToID([]byte("Neo.Contract.Create")):
			log.Println("[SYSCALL]", "Neo.Contract.Create")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					// TODO: IMPL
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Contract.Destroy")):
			log.Println("[SYSCALL]", "Neo.Contract.Destroy")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					// TODO : IMPL
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Contract.GetScript")):
			log.Println("[SYSCALL]", "Neo.Contract.GetScript")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					cs := v.Estack().Pop().Value().(*state.Contract)
					v.Estack().PushVal(cs.Script)
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Contract.GetStorageContext")):
			log.Println("[SYSCALL]", "Neo.Contract.GetStorageContext")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					cs := v.Estack().Pop().Value().(*state.Contract)
					// TODO: CHECK
					stc := &StorageContext{
						ScriptHash: cs.ScriptHash(),
					}
					v.Estack().PushVal(vm.NewInteropItem(stc))
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Contract.IsPayable")):
			log.Println("[SYSCALL]", "Neo.Contract.IsPayable")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					cs := v.Estack().Pop().Value().(*state.Contract)
					v.Estack().PushVal(cs.IsPayable())
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Contract.Migrate")):
			log.Println("[SYSCALL]", "Neo.Contract.Migrate")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					// TODO : IMPL
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Enumerator.Concat")):
			log.Println("[SYSCALL]", "Neo.Enumerator.Concat")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					return vm.EnumeratorConcat(v)
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Enumerator.Create")):
			log.Println("[SYSCALL]", "Neo.Enumerator.Create")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					return vm.EnumeratorCreate(v)
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Enumerator.Next")):
			log.Println("[SYSCALL]", "Neo.Enumerator.Next")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					return vm.EnumeratorNext(v)
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Enumerator.Value")):
			log.Println("[SYSCALL]", "Neo.Enumerator.Value")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					return vm.EnumeratorValue(v)
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Header.GetConsensusData")):
			log.Println("[SYSCALL]", "Neo.Header.GetConsensusData")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					header := mERR(popHeaderFromVM(v)).(*block.Header)
					v.Estack().PushVal(header.ConsensusData)
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Header.GetHash")):
			log.Println("[SYSCALL]", "Neo.Header.GetHash")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					header := mERR(popHeaderFromVM(v)).(*block.Header)
					v.Estack().PushVal(header.Hash().BytesBE())
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Header.GetIndex")):
			log.Println("[SYSCALL]", "Neo.Header.GetIndex")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					header := mERR(popHeaderFromVM(v)).(*block.Header)
					v.Estack().PushVal(header.Index)
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Header.GetMerkleRoot")):
			log.Println("[SYSCALL]", "Neo.Header.GetMerkleRoot")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					header := mERR(popHeaderFromVM(v)).(*block.Header)
					v.Estack().PushVal(header.MerkleRoot.BytesBE())
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Header.GetNextConsensus")):
			log.Println("[SYSCALL]", "Neo.Header.GetNextConsensus")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					header := mERR(popHeaderFromVM(v)).(*block.Header)
					v.Estack().PushVal(header.NextConsensus.BytesBE())
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Header.GetPrevHash")):
			log.Println("[SYSCALL]", "Neo.Header.GetPrevHash")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					header := mERR(popHeaderFromVM(v)).(*block.Header)
					v.Estack().PushVal(header.PrevHash.BytesBE())
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Header.GetTimestamp")):
			log.Println("[SYSCALL]", "Neo.Header.GetTimestamp")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					header := mERR(popHeaderFromVM(v)).(*block.Header)
					v.Estack().PushVal(header.Timestamp)
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Header.GetVersion")):
			log.Println("[SYSCALL]", "Neo.Header.GetVersion")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					header := mERR(popHeaderFromVM(v)).(*block.Header)
					v.Estack().PushVal(header.Version)
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Input.GetHash")):
			log.Println("[SYSCALL]", "Neo.Input.GetHash")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					input := mERR(popInputFromVM(v)).(*transaction.Input)
					v.Estack().PushVal(input.PrevHash.BytesBE())
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Input.GetIndex")):
			log.Println("[SYSCALL]", "Neo.Input.GetIndex")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					input := mERR(popInputFromVM(v)).(*transaction.Input)
					v.Estack().PushVal(input.PrevIndex)
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.InvocationTransaction.GetScript")):
			log.Println("[SYSCALL]", "Neo.InvocationTransaction.GetScript")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					tx := v.Estack().Pop().Value().(*transaction.Transaction)
					inv := tx.Data.(*transaction.InvocationTX)
					script := make([]byte, len(inv.Script))
					copy(script, inv.Script)
					v.Estack().PushVal(script)
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Iterator.Concat")):
			log.Println("[SYSCALL]", "Neo.Iterator.Concat")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					return vm.IteratorConcat(v)
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Iterator.Create")):
			log.Println("[SYSCALL]", "Neo.Iterator.Create")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					return vm.IteratorCreate(v)
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Iterator.Key")):
			log.Println("[SYSCALL]", "Neo.Iterator.Key")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					return vm.IteratorKey(v)
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Iterator.Keys")):
			log.Println("[SYSCALL]", "Neo.Iterator.Keys")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					return vm.IteratorKeys(v)
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Iterator.Values")):
			log.Println("[SYSCALL]", "Neo.Iterator.Values")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					return vm.IteratorValues(v)
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Output.GetAssetId")):
			log.Println("[SYSCALL]", "Neo.Output.GetAssetId")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					as := v.Estack().Pop().Value().(*state.Asset)
					v.Estack().PushVal(as.ID.BytesBE())
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Output.GetScriptHash")):
			log.Println("[SYSCALL]", "Neo.Output.GetScriptHash")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					output := mERR(popOutputFromVM(v)).(*transaction.Output)
					v.Estack().PushVal(output.ScriptHash.BytesBE())
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Output.GetValue")):
			log.Println("[SYSCALL]", "Neo.Output.GetValue")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					output := mERR(popOutputFromVM(v)).(*transaction.Output)
					v.Estack().PushVal(int64(output.Amount))
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Runtime.CheckWitness")):
			log.Println("[SYSCALL]", "Neo.Runtime.CheckWitness")
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
		case vm.InteropNameToID([]byte("Neo.Runtime.Deserialize")):
			log.Println("[SYSCALL]", "Neo.Runtime.Deserialize")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					return vm.RuntimeDeserialize(v)
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Runtime.GetTime")):
			log.Println("[SYSCALL]", "Neo.Runtime.GetTime")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					data := map[string]interface{}{
						"jsonrpc": "2.0",
						"id":      rand.Uint32(),
						"method":  "get_header_by_height_in_hex",
						"params":  map[string]interface{}{"Height": height},
					}
					log.Println("[REQ]", data)
					resp := mERR(http.Post(rpcaddr, "application/json", bytes.NewReader(mERR(json.Marshal(data)).([]byte)))).(*http.Response)
					defer resp.Body.Close()
					mCHK(json.NewDecoder(resp.Body).Decode(&data))
					log.Println("[RESP]", data)
					hd := new(block.Header)
					hd.DecodeBinary(io.NewBinReaderFromBuf(mERR(hex.DecodeString(data["result"].(string))).([]byte)))
					v.Estack().PushVal(hd.Timestamp)
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Runtime.GetTrigger")):
			log.Println("[SYSCALL]", "Neo.Runtime.GetTrigger")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					v.Estack().PushVal(byte(trigger.Application))
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Runtime.Log")):
			log.Println("[SYSCALL]", "Neo.Runtime.Log")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					v.Estack().Pop().Bytes()
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Runtime.Notify")):
			log.Println("[SYSCALL]", "Neo.Runtime.Notify")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					v.Estack().Pop()
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Runtime.Serialize")):
			log.Println("[SYSCALL]", "Neo.Runtime.Serialize")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					return vm.RuntimeSerialize(v)
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Storage.Delete")):
			log.Println("[SYSCALL]", "Neo.Storage.Delete")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					stc := v.Estack().Pop().Value().(*StorageContext)
					if stc.ReadOnly {
						return errors.New("StorageContext is read only")
					}
					// TODO: CHECK
					key := v.Estack().Pop().Bytes()
					sc := hex.EncodeToString(stc.ScriptHash.BytesBE()) + hex.EncodeToString(key)
					storage[sc] = []byte{}
					return nil
				},
				Price: 100,
			}
		case vm.InteropNameToID([]byte("Neo.Storage.Find")):
			log.Println("[SYSCALL]", "Neo.Storage.Find")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					// TODO : IMPL
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Storage.Get")):
			log.Println("[SYSCALL]", "Neo.Storage.Get")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					stc := v.Estack().Pop().Value().(*StorageContext)
					key := v.Estack().Pop().Bytes()
					sc := hex.EncodeToString(stc.ScriptHash.BytesBE()) + hex.EncodeToString(key)
					if ret, ok := storage[sc]; ok {
						v.Estack().PushVal(ret)
					} else {
						data := map[string]interface{}{
							"jsonrpc": "2.0",
							"id":      rand.Uint32(),
							"method":  "get_storage_by_dbkey_height_in_hex",
							"params":  map[string]interface{}{"DBKey": sc, "Height": height},
						}
						log.Println("[REQ]", data)
						resp := mERR(http.Post(rpcaddr, "application/json", bytes.NewReader(mERR(json.Marshal(data)).([]byte)))).(*http.Response)
						defer resp.Body.Close()
						mCHK(json.NewDecoder(resp.Body).Decode(&data))
						log.Println("[RESP]", data)
						val := mERR(hex.DecodeString(data["result"].(string))).([]byte)
						v.Estack().PushVal(val)
					}
					return nil
				},
				Price: 100,
			}
		case vm.InteropNameToID([]byte("Neo.Storage.GetContext")):
			log.Println("[SYSCALL]", "Neo.Storage.GetContext")
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
		case vm.InteropNameToID([]byte("Neo.Storage.GetReadOnlyContext")):
			log.Println("[SYSCALL]", "Neo.Storage.GetReadOnlyContext")
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
		case vm.InteropNameToID([]byte("Neo.Storage.Put")):
			log.Println("[SYSCALL]", "Neo.Storage.Put")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					stc := v.Estack().Pop().Value().(*StorageContext)
					key := v.Estack().Pop().Bytes()
					value := v.Estack().Pop().Bytes()
					sc := hex.EncodeToString(stc.ScriptHash.BytesBE()) + hex.EncodeToString(key)
					storage[sc] = value
					return nil
				},
				Price: 1000,
			}
		case vm.InteropNameToID([]byte("Neo.StorageContext.AsReadOnly")):
			log.Println("[SYSCALL]", "Neo.StorageContext.AsReadOnly")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					stc := v.Estack().Pop().Value().(*StorageContext)
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
		case vm.InteropNameToID([]byte("Neo.Transaction.GetAttributes")):
			log.Println("[SYSCALL]", "Neo.Transaction.GetAttributes")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					tx := v.Estack().Pop().Value().(*transaction.Transaction)
					if len(tx.Attributes) > vm.MaxArraySize {
						return errors.New("too many attributes")
					}
					attrs := make([]vm.StackItem, 0, len(tx.Attributes))
					for i := range tx.Attributes {
						attrs = append(attrs, vm.NewInteropItem(&tx.Attributes[i]))
					}
					v.Estack().PushVal(attrs)
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Transaction.GetHash")):
			log.Println("[SYSCALL]", "Neo.Transaction.GetHash")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					tx := v.Estack().Pop().Value().(*transaction.Transaction)
					v.Estack().PushVal(tx.Hash().BytesBE())
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Transaction.GetInputs")):
			log.Println("[SYSCALL]", "Neo.Transaction.GetInputs")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					tx := v.Estack().Pop().Value().(*transaction.Transaction)
					if len(tx.Inputs) > vm.MaxArraySize {
						return errors.New("too many inputs")
					}
					inputs := make([]vm.StackItem, 0, len(tx.Inputs))
					for i := range tx.Inputs {
						inputs = append(inputs, vm.NewInteropItem(&tx.Inputs[i]))
					}
					v.Estack().PushVal(inputs)
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Transaction.GetOutputs")):
			log.Println("[SYSCALL]", "Neo.Transaction.GetOutputs")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					tx := v.Estack().Pop().Value().(*transaction.Transaction)
					if len(tx.Outputs) > vm.MaxArraySize {
						return errors.New("too many outputs")
					}
					outputs := make([]vm.StackItem, 0, len(tx.Outputs))
					for i := range tx.Outputs {
						outputs = append(outputs, vm.NewInteropItem(&tx.Outputs[i]))
					}
					v.Estack().PushVal(outputs)
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Transaction.GetReferences")):
			log.Println("[SYSCALL]", "Neo.Transaction.GetReferences")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					// TODO: IMPL
					return nil
				},
				Price: 200,
			}
		case vm.InteropNameToID([]byte("Neo.Transaction.GetType")):
			log.Println("[SYSCALL]", "Neo.Transaction.GetType")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					tx := v.Estack().Pop().Value().(*transaction.Transaction)
					v.Estack().PushVal(int(tx.Type))
					return nil
				},
				Price: 1,
			}
		case vm.InteropNameToID([]byte("Neo.Transaction.GetUnspentCoins")):
			log.Println("[SYSCALL]", "Neo.Transaction.GetUnspentCoins")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					// TODO : IMPL
					return nil
				},
				Price: 200,
			}
		case vm.InteropNameToID([]byte("Neo.Transaction.GetWitnesses")):
			log.Println("[SYSCALL]", "Neo.Transaction.GetWitnesses")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					tx := v.Estack().Pop().Value().(*transaction.Transaction)
					if len(tx.Scripts) > vm.MaxArraySize {
						return errors.New("too many outputs")
					}
					scripts := make([]vm.StackItem, 0, len(tx.Scripts))
					for i := range tx.Scripts {
						scripts = append(scripts, vm.NewInteropItem(&tx.Scripts[i]))
					}
					v.Estack().PushVal(scripts)
					return nil
				},
				Price: 200,
			}
		case vm.InteropNameToID([]byte("Neo.Witness.GetVerificationScript")):
			log.Println("[SYSCALL]", "Neo.Witness.GetVerificationScript")
			return &vm.InteropFuncPrice{
				Func: func(v *vm.VM) error {
					wit := v.Estack().Pop().Value().(*transaction.Witness)
					script := make([]byte, len(wit.VerificationScript))
					copy(script, wit.VerificationScript)
					v.Estack().PushVal(script)
					return nil
				},
				Price: 100,
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
	data["method"] = "get_count_in_uint64"
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
	log.Println(data)
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

func mOK(v interface{}, ok bool) interface{} {
	if ok == false {
		panic(ok)
	}
	return v
}

func mERR(v interface{}, err error) interface{} {
	if err != nil {
		panic(err)
	}
	return v
}

func mCHK(err error) {
	if err != nil {
		panic(err)
	}
}

// FROM NSPCC CODE ...

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
		data["method"] = "get_hash_by_height_in_hex"
		data["params"] = map[string]interface{}{"Height": uint32(hashint)}
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
	data["method"] = "get_transaction_by_hash_in_hex"
	// query json here
	data["params"] = map[string]interface{}{"Hash": hash.StringLE()}
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

func popInputFromVM(v *vm.VM) (*transaction.Input, error) {
	input := v.Estack().Pop().Value().(*transaction.Input)
	return input, nil
}

func popOutputFromVM(v *vm.VM) (*transaction.Output, error) {
	output := v.Estack().Pop().Value().(*transaction.Output)
	return output, nil
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
