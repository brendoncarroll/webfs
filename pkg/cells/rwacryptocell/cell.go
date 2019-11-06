package rwacryptocell

import (
	"bytes"
	"context"
	"errors"
	fmt "fmt"
	"log"
	"sync"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"

	"github.com/brendoncarroll/webfs/pkg/cells"
)

var (
	ErrCellUnclaimed = errors.New("cell is unclaimed")
	ErrCellClaimed   = errors.New("cannot initialize already initialized cell")
)

type Spec struct {
	Inner, AuxState cells.Cell

	PrivateEntity *Entity
	PublicEntity  *Entity
}

type mapping struct {
	raw []byte

	state   *CellState
	payload []byte
}

type Cell struct {
	spec Spec

	innerCell cells.Cell
	auxState  cells.Cell

	mu            sync.Mutex
	latestMapping *mapping
}

func New(spec Spec) *Cell {
	cell := &Cell{
		innerCell: spec.Inner,
		spec:      spec,
		auxState:  spec.AuxState,
	}
	return cell
}

func (c *Cell) Get(ctx context.Context) ([]byte, error) {
	m, err := c.getMapping(ctx, true)
	if err != nil {
		return nil, err
	}
	if m.state == nil {
		return nil, ErrCellUnclaimed
	}
	return m.payload, nil
}

func (c *Cell) getRaw(ctx context.Context) ([]byte, error) {
	return c.innerCell.Get(ctx)
}

func (c *Cell) getMapping(ctx context.Context, validate bool) (*mapping, error) {
	var m *mapping
	defer c.storeMapping(m)

	raw, err := c.getRaw(ctx)
	if err != nil {
		return nil, err
	}

	// empty
	if len(raw) < 1 {
		m = &mapping{
			raw:     raw,
			state:   nil,
			payload: nil,
		}
		return m, nil
	}

	state := &CellState{}
	if err := proto.Unmarshal(raw, state); err != nil {
		m = &mapping{
			raw:     raw,
			state:   nil,
			payload: nil,
		}
		return nil, err
	}
	m = &mapping{
		raw:     raw,
		state:   state,
		payload: nil,
	}

	if !validate {
		return m, nil
	}

	// validation
	localACL, err := c.getLocalACL(ctx)
	if err != nil {
		return nil, err
	}
	errs := ValidateState(localACL, state)
	if len(errs) > 0 {
		return nil, fmt.Errorf("validation errors %v", errs)
	}
	if err := c.setLocalACL(ctx, state.Acl); err != nil {
		return nil, err
	}

	// get payload
	payload, err := GetPayload(state, c.getPrivate())
	if err != nil {
		return nil, err
	}
	m = &mapping{
		raw:     raw,
		state:   state,
		payload: payload,
	}

	return m, nil
}

func (c *Cell) CAS(ctx context.Context, cur, next []byte) (bool, error) {
	var err error
	pm := c.getMappingStale()
	if pm == nil {
		pm, err = c.getMapping(ctx, true)
		if err != nil {
			return false, err
		}
	}
	if pm.state == nil {
		return false, ErrCellUnclaimed
	}
	// if it's not what the cell believes to be the latest don't even try.
	if bytes.Compare(pm.payload, cur) != 0 {
		log.Println("INFO: caller does not have latest cell state")
		if _, err := c.getMapping(ctx, true); err != nil {
			return false, err
		}
		return c.CAS(ctx, cur, next)
	}
	// if it is, then use the latest contents as the current
	curState := pm.state

	// create next contents with the next payload
	nextState, err := PutPayload(curState, c.getPrivate(), next)
	if err != nil {
		return false, err
	}

	success, err := c.cas(ctx, curState, nextState, cur)
	if err != nil {
		return false, err
	}

	return success, nil
}

func (c *Cell) cas(ctx context.Context, curState, nextState *CellState, ptext []byte) (bool, error) {
	var err error
	pm := c.getMappingStale()
	if pm == nil {
		pm, err = c.getMapping(ctx, true)
	}
	curBytes := pm.raw

	nextBytes, err := proto.Marshal(nextState)
	if err != nil {
		return false, err
	}

	success, err := c.innerCell.CAS(ctx, curBytes, nextBytes)
	if err != nil {
		return false, err
	}

	ptext2 := make([]byte, len(ptext))
	copy(ptext2, ptext)
	c.storeMapping(&mapping{
		raw:     nextBytes,
		state:   nextState,
		payload: ptext2,
	})
	return success, nil
}

func (c *Cell) URL() string {
	return "ccp-" + c.innerCell.URL()
}

func (c *Cell) storeMapping(pm *mapping) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.latestMapping = pm
}

func (c *Cell) getMappingStale() *mapping {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.latestMapping
}

func (c *Cell) getPrivate() *Entity {
	return c.spec.PrivateEntity
}

func (c *Cell) getLocalState(ctx context.Context) (*LocalState, error) {
	data, err := c.auxState.Get(ctx)
	if err != nil {
		return nil, err
	}
	if len(data) < 1 {
		return nil, nil
	}
	ret := &LocalState{}
	return ret, proto.Unmarshal(data, ret)
}

func (c *Cell) setLocalState(ctx context.Context, next *LocalState) error {
	var (
		nextBytes []byte
		err       error
	)

	curBytes, err := c.auxState.Get(ctx)
	if err != nil {
		return err
	}

	if next != nil {
		nextBytes, err = proto.Marshal(next)
		if err != nil {
			return err
		}
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

func (c *Cell) getLocalACL(ctx context.Context) (*ACL, error) {
	localState, err := c.getLocalState(ctx)
	if err != nil {
		return nil, err
	}
	if localState == nil {
		return nil, nil
	}
	return localState.Acl, nil
}

func (c *Cell) setLocalACL(ctx context.Context, next *ACL) error {
	return c.setLocalState(ctx, &LocalState{Acl: next})
}

func (c *Cell) Claim(ctx context.Context) error {
	m, err := c.getMapping(ctx, false)
	if err != nil {
		return err
	}
	current := m.state
	if current != nil && current.Acl != nil {
		return ErrCellClaimed
	}

	next := &CellState{}
	next, err = AddEntity(next, c.spec.PrivateEntity, c.spec.PublicEntity)
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

	success, err := c.cas(ctx, current, next, nil)
	if err != nil {
		return err
	}
	if !success {
		return errors.New("CAS failed during init. Another party using cell?")
	}

	if err := c.setLocalACL(ctx, next.Acl); err != nil {
		return err
	}

	return nil
}

func (c *Cell) Join(ctx context.Context, fn func(x interface{}) bool) error {
	m, err := c.getMapping(ctx, false)
	if err != nil {
		return err
	}
	current := m.state
	if current == nil {
		return ErrCellUnclaimed
	}
	ok := fn(current.Acl)
	if !ok {
		return errors.New("join refused by callback")
	}
	return c.setLocalACL(ctx, current.Acl)
}

func (c *Cell) AddEntity(ctx context.Context, ent *Entity) error {
	m, err := c.getMapping(ctx, true)
	if err != nil {
		return err
	}
	prev := m.state

	next, err := AddEntity(prev, c.getPrivate(), ent)
	if err != nil {
		return err
	}

	success, err := c.cas(ctx, prev, next, nil)
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
	m, err := c.getMapping(ctx, true)
	if err != nil {
		return err
	}
	prev := m.state

	var next *CellState
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

	success, err := c.cas(ctx, prev, next, nil)
	if err != nil {
		return err
	}
	if !success {
		return errors.New("CAS failed")
	}
	return nil
}

func (c *Cell) ResetAuxState(ctx context.Context) error {
	return cells.ForcePut(ctx, c.auxState, nil, 10)
}

func (c *Cell) Inspect(ctx context.Context) string {
	buf := &bytes.Buffer{}
	buf.WriteString("--RWA CRYPTO CELL--\n")
	buf.WriteString("URL: " + c.URL() + "\n")

	buf.WriteString("CELL STATE:\n")
	mp, err := c.getMapping(ctx, false)
	if err != nil {
		fmt.Fprint(buf, "error: ", err)
	}
	state := mp.state
	m := jsonpb.Marshaler{Indent: " "}
	m.Marshal(buf, state)
	buf.WriteString("\n")

	buf.WriteString("AUX STATE:\n")
	local, err := c.getLocalACL(ctx)
	if err != nil {
		fmt.Fprint(buf, "error: ", err)
	}
	m.Marshal(buf, local)
	return buf.String()
}
