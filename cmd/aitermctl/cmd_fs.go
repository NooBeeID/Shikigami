package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"aiterm/internal/ipc"
)

const fsUsage = `penggunaan:
  aitermctl fs ls <root> [relDir]
  aitermctl fs cat <root> <path>
  aitermctl fs write <root> <path> <expectedMtime>   (isi dari stdin)`

func init() {
	RegisterCommand(Command{
		Name:    "fs",
		Summary: "Operasi file service (ls, cat, write)",
		Run:     runFS,
	})
}

func fsError(err error) error {
	var ie *ipc.Error
	if !errors.As(err, &ie) {
		return err
	}
	switch ie.Code {
	case ipc.ErrBinary:
		return fmt.Errorf("file biner tidak didukung (%s)", ie.Message)
	case ipc.ErrConflict:
		if m, ok := ie.Data.(map[string]any); ok {
			if v, ok := m["currentMtime"].(float64); ok {
				return fmt.Errorf("file berubah di disk (mtime disk: %d)", int64(v))
			}
		}
		return fmt.Errorf("file berubah di disk (%s)", ie.Message)
	case ipc.ErrNotFound:
		return fmt.Errorf("tidak ditemukan (%s)", ie.Message)
	case ipc.ErrInvalid:
		return fmt.Errorf("parameter tidak valid (%s)", ie.Message)
	}
	return err
}

func runFS(ctx context.Context, c *ipc.Client, args []string) error {
	if len(args) == 0 {
		return errors.New(fsUsage)
	}
	switch args[0] {
	case "ls":
		if len(args) < 2 || len(args) > 3 {
			return errors.New(fsUsage)
		}
		relDir := ""
		if len(args) == 3 {
			relDir = args[2]
		}
		var out struct {
			Entries []struct {
				Name string `json:"name"`
				Type string `json:"type"`
				Size int64  `json:"size"`
			} `json:"entries"`
		}
		if err := c.Call("fs.list", map[string]any{"root": args[1], "relDir": relDir}, &out); err != nil {
			return fsError(err)
		}
		for _, e := range out.Entries {
			fmt.Printf("%s\t%d\t%s\n", e.Type, e.Size, e.Name)
		}
		return nil
	case "cat":
		if len(args) != 3 {
			return errors.New(fsUsage)
		}
		var out struct {
			Content string `json:"content"`
		}
		if err := c.Call("fs.read", map[string]any{"root": args[1], "path": args[2]}, &out); err != nil {
			return fsError(err)
		}
		_, err := io.WriteString(os.Stdout, out.Content)
		return err
	case "write":
		if len(args) != 4 {
			return errors.New(fsUsage)
		}
		mt, err := strconv.ParseInt(args[3], 10, 64)
		if err != nil {
			return errors.New("parameter tidak valid: expectedMtime harus angka")
		}
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		var out struct {
			Mtime int64 `json:"mtime"`
			Size  int64 `json:"size"`
		}
		if err := c.Call("fs.write", map[string]any{
			"root": args[1], "path": args[2], "content": string(data), "expectedMtime": mt,
		}, &out); err != nil {
			return fsError(err)
		}
		fmt.Printf("mtime\t%d\nsize\t%d\n", out.Mtime, out.Size)
		return nil
	}
	return errors.New(fsUsage)
}
