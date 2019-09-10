package fuseadapt

import (
	"log"
	"os"
	"os/signal"
	"time"

	"bazil.org/fuse"
	fusefs "bazil.org/fuse/fs"
	"github.com/brendoncarroll/webfs/pkg/webfs"
)

func MountAndRun(wfs *webfs.WebFS, p string) error {
	errs := make(chan error)
	fs := &FS{wfs}

	opts := []fuse.MountOption{
		fuse.VolumeName("WebFS"),
	}
	conn, err := fuse.Mount(p, opts...)
	if err != nil {
		return err
	}

	// listen for CTRL^C
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		sig := <-c
		log.Println("got", sig.String(), "exiting")
		log.Println("trying to unmount the filesystem")
		log.Println("you may see errors here; it's fine give it a minute")
		log.Println("if it doesn't work run this: kill", os.Getpid())

		tick := time.NewTicker(3 * time.Second)
		defer tick.Stop()
		var lastErr error
		for range tick.C {
			if err := fuse.Unmount(p); err != nil {
				if lastErr == nil || err.Error() != lastErr.Error() {
					log.Println(err)
				}
				lastErr = err
			} else {
				lastErr = err
				break
			}
		}
		errs <- lastErr
	}()

	if err := fusefs.Serve(conn, fs); err != nil {
		log.Println(err)
	}

	return <-errs
}
