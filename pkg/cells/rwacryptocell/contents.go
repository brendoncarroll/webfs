package rwacryptocell

import (
	"errors"
	fmt "fmt"
	"log"
	"sort"

	"github.com/golang/protobuf/proto"
)

func GetPayload(c *CellContents, privEnt *Entity) ([]byte, error) {
	if c.Who == nil {
		return nil, errors.New("nil who")
	}
	i := findEntity(c.Who.Entities, GetPublicEntity(privEnt))
	ri := int32findI(c.Who.Read, int32(i))
	if ri == -1 {
		return nil, errors.New("entity is not a reader")
	}

	dekMsg := c.What.Deks[ri]
	secretKey, err := AsymDecrypt(privEnt.EncryptionKey, dekMsg)
	if err != nil {
		return nil, err
	}
	return SymDecrypt(c.What.Payload, secretKey)
}

func PutPayload(prev *CellContents, privEntity *Entity, data []byte) (*CellContents, error) {
	what, err := createWhat(prev, privEntity.GetSigningKey(), data)
	if err != nil {
		return nil, err
	}
	whatSig, err := Sign(data, privEntity.GetSigningKey())
	if err != nil {
		return nil, err
	}
	whatAuthor := findEntity(prev.Who.Entities, GetPublicEntity(privEntity))

	return &CellContents{
		Who:       prev.Who,
		WhoAuthor: prev.WhoAuthor,
		WhoSigs:   prev.WhoSigs,

		What:       what,
		WhatAuthor: int32(whatAuthor),
		WhatSig:    whatSig,
	}, nil
}

func AddReader(prev *CellContents, privEnt, readEnt *Entity) (*CellContents, error) {
	contents := proto.Clone(prev).(*CellContents)
	err := updateWho(contents, privEnt, func(x *Who) (*Who, error) {
		i := findEntity(x.Entities, readEnt)
		if i < 0 {
			return nil, fmt.Errorf("entity not found")
		}
		x.Read = append(x.Read, int32(i))
		return x, nil
	})
	return contents, err
}

func AddWriter(prev *CellContents, privEnt, writeEnt *Entity) (*CellContents, error) {
	contents := proto.Clone(prev).(*CellContents)
	err := updateWho(contents, privEnt, func(x *Who) (*Who, error) {
		i := findEntity(x.Entities, writeEnt)
		if i < 0 {
			return nil, fmt.Errorf("entity not found")
		}
		x.Write = append(x.Write, int32(i))
		return x, nil
	})
	return contents, err
}

func AddAdmin(prev *CellContents, privEnt, adminEnt *Entity) (*CellContents, error) {
	contents := proto.Clone(prev).(*CellContents)
	err := updateWho(contents, privEnt, func(x *Who) (*Who, error) {
		adminI := findEntity(x.Entities, adminEnt)
		if adminI < 0 {
			return nil, fmt.Errorf("entity not found")
		}
		x.Admin = append(x.Admin, int32(adminI))
		return x, nil
	})
	return contents, err
}

func AddEntity(prev *CellContents, privEnt, newEnt *Entity) (*CellContents, error) {
	contents := proto.Clone(prev).(*CellContents)
	err := updateWho(contents, privEnt, func(x *Who) (*Who, error) {
		i := findEntity(x.Entities, newEnt)
		if i != -1 {
			return nil, errors.New("entity already exists")
		}
		x.Entities = append(x.Entities, newEnt)
		return x, nil
	})
	return contents, err
}

func updateWho(cc *CellContents, privEnt *Entity, fn func(*Who) (*Who, error)) error {
	x := cc.Who
	if x == nil {
		x = &Who{}
	}
	y, err := fn(x)
	if err != nil {
		return err
	}
	cc.Who = y

	cc.WhoAuthor, cc.WhoSigs, err = signWho(privEnt, y)
	if err != nil {
		return err
	}

	return nil
}

func signWho(privEnt *Entity, who *Who) (int32, map[int32]*Sig, error) {
	author := int32(findEntity(who.Entities, GetPublicEntity(privEnt)))
	if author < 0 {
		return -1, nil, fmt.Errorf("signing entity not found")
	}
	whoBytes, err := who.XXX_Marshal(nil, true)
	if err != nil {
		return -1, nil, err
	}
	sig, err := Sign(whoBytes, privEnt.SigningKey)
	if err != nil {
		return -1, nil, err
	}
	sigs := map[int32]*Sig{
		author: sig,
	}
	return author, sigs, nil
}

func createWhat(prev *CellContents, signer *Key, data []byte) (*What, error) {
	prevWhat := prev.What
	if prevWhat == nil {
		prevWhat = &What{}
	}
	var err error

	encMsg, secretKey, err := SymEncrypt(data)
	if err != nil {
		return nil, err
	}

	deks := make([]*AsymEncMsg, len(prev.Who.Read))
	for i, entityID := range prev.Who.Read {
		entity := prev.Who.Entities[entityID]
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
	n := sort.Search(len(s), func(i int) bool {
		return x >= s[i]
	})
	if n < len(s) {
		return n
	}
	return -1
}
