package rwacryptocell

import (
	"errors"
	fmt "fmt"
	"sort"

	proto "github.com/golang/protobuf/proto"
)

func ValidateState(acl *ACL, next *CellState) []error {
	var errs []error

	if acl == nil {
		return nil
	}
	errs2 := ValidateACL(acl, next.Acl, next.AclSigs)
	errs = append(errs, errs2...)

	errs2 = ValidateWhat(next.Acl, next.What, next.WhatAuthor, next.WhatSig)
	errs = append(errs, errs2...)

	return errs
}

func ValidateWhat(acl *ACL, what *What, signerIndex int32, sig *Sig) []error {
	var errs []error

	if acl == nil {
		err := errors.New("missing acl")
		errs = append(errs, err)
		return errs
	}

	if !int32Contains(acl.Write, signerIndex) {
		err := fmt.Errorf("author does not have write permission")
		errs = append(errs, err)
	}

	author := acl.Entities[int(signerIndex)]

	if what == nil {
		what = &What{}
	}
	whatBytes, err := proto.Marshal(what)
	if err != nil {
		panic(err)
	}

	if err := VerifySig(whatBytes, author.SigningKey, sig); err != nil {
		errs = append(errs, err)
	}

	return nil
}

func ValidateACL(prev, next *ACL, sigs map[int32]*Sig) []error {
	var errs []error
	if next == nil {
		err := errors.New("nil acl")
		errs = append(errs, err)
		return errs
	}

	aclBytes, err := proto.Marshal(next)
	if err != nil {
		panic(err)
	}

	admins := prev.Admin
	sort.Slice(admins, func(i, j int) bool {
		return admins[i] < admins[j]
	})

	// must find an admin acl signed off on it
	for i, sig := range sigs {
		if int32Contains(admins, i) {
			signer := prev.Entities[i].SigningKey
			if err := VerifySig(aclBytes, signer, sig); err != nil {
				errs = append(errs, err)
			}
			return errs
		}
	}
	noSigErr := fmt.Errorf("no valid admin sig found")
	errs = append(errs, noSigErr)
	return errs
}

func int32Contains(s []int32, x int32) bool {
	n := sort.Search(len(s), func(i int) bool {
		return x >= s[i]
	})
	if n >= len(s) {
		return false
	}
	return s[n] == x
}
