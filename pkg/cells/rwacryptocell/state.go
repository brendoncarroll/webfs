package rwacryptocell

import (
	"errors"
	fmt "fmt"
	"log"

	"github.com/golang/protobuf/proto"
)

func GetPayload(c *CellState, privEnt *Entity) ([]byte, error) {
	if c.Acl == nil {
		return nil, errors.New("nil acl")
	}
	i := findEntity(c.Acl.Entities, GetPublicEntity(privEnt))
	ri := int32findI(c.Acl.Read, int32(i))
	if ri == -1 {
		return nil, errors.New("entity is not a reader")
	}

	if ri >= len(c.What.Deks) {
		log.Println("INFO: given read access after last write. returning nil")
		return nil, nil
	}
	dekMsg := c.What.Deks[ri]
	secretKey, err := AsymDecrypt(privEnt.EncryptionKey, dekMsg)
	if err != nil {
		return nil, err
	}
	return SymDecrypt(c.What.Payload, secretKey)
}

func PutPayload(prev *CellState, privEntity *Entity, data []byte) (*CellState, error) {
	what, err := createWhat(prev, privEntity.GetSigningKey(), data)
	if err != nil {
		return nil, err
	}
	whatSig, err := Sign(data, privEntity.GetSigningKey())
	if err != nil {
		return nil, err
	}
	whatAuthor := findEntity(prev.Acl.Entities, GetPublicEntity(privEntity))

	return &CellState{
		Acl:       prev.Acl,
		AclAuthor: prev.AclAuthor,
		AclSigs:   prev.AclSigs,

		What:       what,
		WhatAuthor: int32(whatAuthor),
		WhatSig:    whatSig,
	}, nil
}

func AddReader(prev *CellState, privEnt, readEnt *Entity) (*CellState, error) {
	contents := proto.Clone(prev).(*CellState)
	err := updateACL(contents, privEnt, func(x *ACL) (*ACL, error) {
		i := findEntity(x.Entities, readEnt)
		if i < 0 {
			return nil, fmt.Errorf("entity not found")
		}
		x.Read = append(x.Read, int32(i))
		return x, nil
	})
	return contents, err
}

func AddWriter(prev *CellState, privEnt, writeEnt *Entity) (*CellState, error) {
	contents := proto.Clone(prev).(*CellState)
	err := updateACL(contents, privEnt, func(x *ACL) (*ACL, error) {
		i := findEntity(x.Entities, writeEnt)
		if i < 0 {
			return nil, fmt.Errorf("entity not found")
		}
		x.Write = append(x.Write, int32(i))
		return x, nil
	})
	return contents, err
}

func AddAdmin(prev *CellState, privEnt, adminEnt *Entity) (*CellState, error) {
	contents := proto.Clone(prev).(*CellState)
	err := updateACL(contents, privEnt, func(x *ACL) (*ACL, error) {
		adminI := findEntity(x.Entities, adminEnt)
		if adminI < 0 {
			return nil, fmt.Errorf("entity not found")
		}
		x.Admin = append(x.Admin, int32(adminI))
		return x, nil
	})
	return contents, err
}

func AddEntity(prev *CellState, privEnt, newEnt *Entity) (*CellState, error) {
	contents := proto.Clone(prev).(*CellState)
	err := updateACL(contents, privEnt, func(x *ACL) (*ACL, error) {
		i := findEntity(x.Entities, newEnt)
		if i != -1 {
			return nil, errors.New("entity already exists")
		}
		x.Entities = append(x.Entities, newEnt)
		return x, nil
	})
	return contents, err
}

func updateACL(cc *CellState, privEnt *Entity, fn func(*ACL) (*ACL, error)) error {
	x := cc.Acl
	if x == nil {
		x = &ACL{}
	}
	y, err := fn(x)
	if err != nil {
		return err
	}
	cc.Acl = y

	var sig *Sig
	cc.AclAuthor, sig, err = signACL(privEnt, y)
	if err != nil {
		return err
	}
	cc.AclSigs = map[int32]*Sig{cc.AclAuthor: sig}

	return nil
}

func signACL(privEnt *Entity, acl *ACL) (int32, *Sig, error) {
	author := int32(findEntity(acl.Entities, GetPublicEntity(privEnt)))
	if author < 0 {
		return -1, nil, fmt.Errorf("signing entity not found")
	}
	aclBytes, err := proto.Marshal(acl)
	if err != nil {
		return -1, nil, err
	}
	sig, err := Sign(aclBytes, privEnt.SigningKey)
	if err != nil {
		return -1, nil, err
	}
	return author, sig, nil
}

func createWhat(prev *CellState, signer *Key, data []byte) (*What, error) {
	prevWhat := prev.What
	if prevWhat == nil {
		prevWhat = &What{}
	}
	var err error

	encMsg, secretKey, err := SymEncrypt(data)
	if err != nil {
		return nil, err
	}

	deks := make([]*AsymEncMsg, len(prev.Acl.Read))
	for i, entityID := range prev.Acl.Read {
		entity := prev.Acl.Entities[entityID]
		deks[i], err = AsymEncrypt(secretKey, entity.EncryptionKey)
		if err != nil {
			log.Println("error encrypting for", entity, err)
			deks[i] = nil
		}
	}

	what := &What{
		Deks:    deks,
		Payload: encMsg,
		Gen:     prevWhat.Gen + 1,
	}

	return what, nil
}

func int32findI(s []int32, x int32) int {
	for i := range s {
		if s[i] == x {
			return i
		}
	}
	return -1
}
