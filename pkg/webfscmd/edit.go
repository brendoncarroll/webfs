package webfscmd

import (
	"context"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit <superblock> <path>",
		Short: "edit a file using $EDITOR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := args[0]
			f, err := os.CreateTemp("", "webfs-edit-")
			if err != nil {
				return err
			}
			defer f.Close()
			defer os.Remove(f.Name())
			// read out
			if err := wfs.Cat(ctx, p, f); err != nil {
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
			// edit
			tmpPath := filepath.Join(f.Name())
			if err := userEditor(ctx, tmpPath); err != nil {
				return err
			}
			f, err = os.Open(tmpPath)
			if err != nil {
				return err
			}
			defer f.Close()
			// write in
			if _, err := f.Seek(0, io.SeekStart); err != nil {
				return err
			}
			return wfs.PutFile(ctx, p, f)
		},
	}
}

func userEditor(ctx context.Context, p string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}
	log.Println(editor, p)
	cmd := exec.CommandContext(ctx, editor, p)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
