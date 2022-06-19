package webfscmd

import (
	"context"
	iofs "io/fs"
	"net"
	"net/http"

	"github.com/brendoncarroll/webfs/pkg/webfs"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newHTTPCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "http",
		Short: "serve files over http",
	}
	laddr := c.Flags().String("addr", "127.0.0.1:7007", "--addr 127.0.0.1:12345")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		h := http.FileServer(http.FS(iofsAdapt{wfs}))
		l, err := net.Listen("tcp", *laddr)
		if err != nil {
			return err
		}
		defer l.Close()
		logrus.Infof("serving on http://%v", l.Addr())
		return http.Serve(l, h)
	}
	return c
}

type iofsAdapt struct {
	wfs *webfs.FS
}

func (fs iofsAdapt) Open(p string) (iofs.File, error) {
	return fs.wfs.Open(context.Background(), p)
}
