package rwacryptocell

import (
	"bytes"
	"context"
	"errors"
	fmt "fmt"
	"sync/atomic"

	"github.com/golang/protobuf/proto"

	"github.com/brendoncarroll/webfs/pkg/cells"
)

var (
	ErrCellUninitialized = errors.New("cell is uninitialized")
	ErrCellInitialized   = errors.New("cannot initialize already initialized cell")
)

type Spec struct {
	Inner cells.Cell

	PrivateEntity *Entity
	PublicEntity  *Entity
}

type payloadMapping struct {
	payload  []byte
	contents *CellContents
}

type Cell struct {
	innerCell cells.Cell
	spec      Spec

	auxState cells.Cell

	latestPayload atomic.Value
}

func New(spec Spec, auxState cells.Cell) *Cell {
	cell := &Cell{
		innerCell: spec.Inner,
		spec:      spec,
		auxState:  auxState,
	}
	return cell
}

func (c *Cell) Get(ctx context.Context) ([]byte, error) {
	return c.get(ctx)
}

func (c *Cell) get(ctx context.Context) ([]byte, error) {
	contents, err := c.getContents(ctx, true)
	if err != nil {
		return nil, err
	}
	if contents.Who == nil || contents.What == nil {
		return nil, ErrCellUninitialized
	}

	payload, err := GetPayload(contents, c.spec.PrivateEntity)
	if err != nil {
		return nil, err
	}
	c.latestPayload.Store(&payloadMapping{
		contents: contents,
		payload:  payload,
	})

	return payload, err
}

func (c *Cell) getContents(ctx context.Context, validate bool) (*CellContents, error) {
	data, err := c.innerCell.Get(ctx)
	if err != nil {
		return nil, err
	}
	contents := &CellContents{}
	if err := proto.Unmarshal(data, contents); err != nil {
		return nil, err
	}
	if !validate {
		return contents, nil
	}

	// validation
	localWho, err := c.getLocalWho(ctx)
	if err != nil {
		return nil, err
	}

	errs := ValidateContents(localWho, contents)
	if len(errs) > 0 {
		return nil, fmt.Errorf("validation errors %v", errs)
	}

	if err := c.setLocalWho(ctx, localWho, contents.Who); err != nil {
		return nil, err
	}
	return contents, nil
}

func (c *Cell) CAS(ctx context.Context, cur, next []byte) (bool, error) {
	pm := c.latestPayload.Load().(*payloadMapping)
	// if it's not what the cell believes to be the latest don't even try.
	if bytes.Compare(pm.payload, cur) != 0 {
		return false, nil
	}
	// if it is, then use the latest contents as the current
	curContents := pm.contents

	// create next contents with the next payload
	nextContents, err := PutPayload(curContents, c.getPrivate(), next)
	if err != nil {
		return false, err
	}

	success, err := c.cas(ctx, curContents, nextContents)
	if err != nil {
		return false, err
	}

	if success {
		c.latestPayload.Store(&payloadMapping{
			contents: nextContents,
			payload:  next,
		})
	}

	return success, nil
}

func (c *Cell) cas(ctx context.Context, curContents, nextContents *CellContents) (bool, error) {
	curBytes, err := proto.Marshal(curContents)
	if err != nil {
		return false, err
	}
	nextBytes, err := proto.Marshal(nextContents)
	if err != nil {
		return false, err
	}

	success, err := c.innerCell.CAS(ctx, curBytes, nextBytes)
	if err != nil {
		return false, err
	}
	return success, nil
}

func (c *Cell) URL() string {
	return "ccp-" + c.innerCell.URL()
}

func (c *Cell) getPrivate() *Entity {
	return c.spec.PrivateEntity
}

func (c *Cell) getLocalWho(ctx context.Context) (*Who, error) {
	data, err := c.auxState.Get(ctx)
	if err != nil {
		return nil, err
	}
	if len(data) < 1 {
		return nil, nil
	}
	ret := &Who{}
	return ret, proto.Unmarshal(data, ret)
}

func (c *Cell) setLocalWho(ctx context.Context, cur, next *Who) error {
	var curBytes []byte
	var err error
	if cur != nil {
		curBytes, err = proto.Marshal(cur)
		if err != nil {
			return err
		}
	}

	nextBytes, err := proto.Marshal(next)
	if err != nil {
		return err
	}

	success, err := c.auxState.CAS(ctx, curBytes, nextBytes)
	if err != nil {
		return err
	}
	if !success {
		return errors.New("a race occurred")
	}
	return nil
}

// AcceptRemote accpets the Authorization settings from the
// remote server.
func (c *Cell) AcceptRemote(ctx context.Context) error {
	current, err := c.getContents(ctx, false)
	if err != nil {
		return err
	}
	localWho, err := c.getLocalWho(ctx)
	if err != nil {
		return err
	}
	return c.setLocalWho(ctx, localWho, current.Who)
}

func (c *Cell) Init(ctx context.Context) error {
	current, err := c.getContents(ctx, false)
	if err != nil {
		return err
	}

	if current.Who != nil {
		return ErrCellInitialized
	}

	next, err := AddEntity(current, c.spec.PrivateEntity, c.spec.PublicEntity)
	if err != nil {
		return err
	}
	next, err = AddAdmin(next, c.spec.PrivateEntity, c.spec.PublicEntity)
	if err != nil {
		return err
	}
	next, err = AddWriter(next, c.spec.PrivateEntity, c.spec.PublicEntity)
	if err != nil {
		return err
	}
	next, err = AddReader(next, c.spec.PrivateEntity, c.spec.PublicEntity)
	if err != nil {
		return err
	}
	next, err = PutPayload(next, c.spec.PrivateEntity, nil)
	if err != nil {
		return err
	}

	success, err := c.cas(ctx, current, next)
	if err != nil {
		return err
	}
	if !success {
		return errors.New("CAS failed during init. Another party using cell?")
	}

	c.latestPayload.Store(&payloadMapping{
		contents: next,
		payload:  nil,
	})
	return nil
}

func (c *Cell) AddEntity(ctx context.Context, ent *Entity) error {
	prev, err := c.getContents(ctx, true)
	if err != nil {
		return err
	}
	next, err := AddEntity(prev, c.getPrivate(), ent)
	if err != nil {
		return err
	}

	success, err := c.cas(ctx, prev, next)
	if err != nil {
		return err
	}
	if !success {
		return errors.New("CAS failed")
	}
	return nil
}

const (
	roleAdmin = iota
	roleWrite
	roleRead
)

func (c *Cell) GrantAdmin(ctx context.Context, ent *Entity) error {
	return c.grantRole(ctx, ent, roleAdmin)
}

func (c *Cell) GrantWrite(ctx context.Context, ent *Entity) error {
	return c.grantRole(ctx, ent, roleWrite)
}

func (c *Cell) GrantRead(ctx context.Context, ent *Entity) error {
	return c.grantRole(ctx, ent, roleRead)
}

func (c *Cell) grantRole(ctx context.Context, ent *Entity, role int) error {
	prev, err := c.getContents(ctx, true)
	if err != nil {
		return err
	}

	var next *CellContents
	switch role {
	case roleAdmin:
		next, err = AddAdmin(prev, c.getPrivate(), ent)
	case roleWrite:
		next, err = AddWriter(prev, c.getPrivate(), ent)
	case roleRead:
		next, err = AddReader(prev, c.getPrivate(), ent)
	default:
		panic("invalid role: " + fmt.Sprint(role))
	}
	if err != nil {
		return err
	}

	success, err := c.cas(ctx, prev, next)
	if err != nil {
		return err
	}
	if !success {
		return errors.New("CAS failed")
	}
	return nil
}

func (c *Cell) AuxState() string {
	ctx := context.TODO()
	who, err := c.getLocalWho(ctx)
	if err != nil {
		return "error"
	}
	return who.String()
}
