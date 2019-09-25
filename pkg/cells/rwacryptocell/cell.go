package rwacryptocell

import (
	"bytes"
	"context"
	"errors"
	fmt "fmt"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/golang/protobuf/proto"

	"github.com/brendoncarroll/webfs/pkg/cells"
)

var (
	ErrCellUninitialized = errors.New("cell is uninitialized")
	ErrCellInitialized   = errors.New("cannot initialize already initialized cell")
)

type Spec struct {
	Inner cells.Cell
	Who   *Who

	PrivateEntity *Entity
	PublicEntity  *Entity
}

type Cell struct {
	innerCell cells.Cell
	spec      Spec

	mu             sync.Mutex
	latestContents *CellContents
	latestPayload  []byte
}

func New(spec Spec) *Cell {
	cell := &Cell{
		innerCell: spec.Inner,
		spec:      spec,
	}
	return cell
}

func (c *Cell) Get(ctx context.Context) ([]byte, error) {
	return c.get(ctx)
}

func (c *Cell) get(ctx context.Context) ([]byte, error) {
	contents, err := c.getContents(ctx)
	if err != nil {
		return nil, err
	}
	if contents.Who == nil || contents.What == nil {
		return nil, ErrCellUninitialized
	}

	errs := ValidateContents(c.spec, contents)
	if len(errs) > 0 {
		return nil, fmt.Errorf("validation errors %v", errs)
	}
	payload, err := GetPayload(contents, c.spec.PrivateEntity)
	if err != nil {
		return nil, err
	}
	c.latestContents = contents
	c.latestPayload = payload
	return payload, err
}

func (c *Cell) getContents(ctx context.Context) (*CellContents, error) {
	data, err := c.innerCell.Get(ctx)
	if err != nil {
		return nil, err
	}
	contents := &CellContents{}
	if err := proto.Unmarshal(data, contents); err != nil {
		return nil, err
	}
	return contents, nil
}

func (c *Cell) CAS(ctx context.Context, cur, next []byte) (bool, error) {
	// if it's not what the cell believes to be the latest don't even try.
	if bytes.Compare(c.latestPayload, cur) != 0 {
		return false, nil
	}
	// if it is, then use the latest contents as the current
	curContents := c.latestContents

	// create next contents with the next payload
	nextContents, err := PutPayload(curContents, c.getPrivate(), next)
	if err != nil {
		return false, err
	}

	return c.cas(ctx, curContents, nextContents)
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
	if success {
		c.updateLatest(nextContents, nextBytes)
	}
	return success, nil
}

func (c *Cell) GetSpec() interface{} {
	return c.spec
}

func (c *Cell) URL() string {
	return "ccp-" + c.innerCell.URL()
}

func (c *Cell) getPrivate() *Entity {
	return c.spec.PrivateEntity
}

func (c *Cell) updateLatest(contents *CellContents, payload []byte) {
	c.mu.Lock()
	c.latestContents = contents
	c.latestPayload = payload
	c.mu.Unlock()
}

func (c *Cell) init(ctx context.Context, retries int) error {
	if retries < 0 {
		return errors.New("cas failed")
	}

	current, err := c.getContents(ctx)
	if err != nil {
		return err
	}

	if current.Who != nil || current.What != nil {
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
		return c.init(ctx, retries-1)
	}
	return nil
}

func httpPut(u string, reqBytes []byte) ([]byte, error) {
	hreq, err := http.NewRequest(http.MethodPut, u, bytes.NewBuffer(reqBytes))
	if err != nil {
		panic(err)
	}
	hres, err := http.DefaultClient.Do(hreq)
	if err != nil {
		return nil, err
	}
	defer hres.Body.Close()
	return ioutil.ReadAll(hres.Body)
}
