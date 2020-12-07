package main

import (
	"errors"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/vm"
)

//func getBlockHashFromElement(element *vm.Element) (util.Uint256, error) {
//	var hash util.Uint256
//	hashbytes := element.Bytes()
//	if len(hashbytes) <= 5 {
//		hashint := element.BigInt().Int64()
//		if hashint < 0 || hashint > math.MaxUint32 {
//			return hash, errors.New("bad block index")
//		}
//		// TODO:
//		// hash = bc.GetHeaderHash(int(hashint)) call RPC GetHashByHeight
//	} else {
//		return util.Uint256DecodeBytesBE(hashbytes)
//	}
//	return hash, nil
//}

//func getTransactionAndHeight(v *vm.VM) (*transaction.Transaction, uint32, error) {
//	hashbytes := v.Estack().Pop().Bytes()
//	hash, err := util.Uint256DecodeBytesBE(hashbytes)
//	fmt.Println(hash)
//	if err != nil {
//		return nil, 0, err
//	}
//	// TODO: call RPC to get transaction By hash;
//	// return c	// return cd.GetTransaction(hash)d.GetTransaction(hash)
//	return nil,0,nil
//}

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


