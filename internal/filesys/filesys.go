package filesys

import (
	"fmt"
	"os"

	"tinygo.org/x/tinyfs"
	"tinygo.org/x/tinyfs/littlefs"
)

func OpenFile(fs *littlefs.LFS, path string, flags int) (*littlefs.File, error) {
	f, err := fs.OpenFile(path, flags)
	if err != nil {
		return nil, err
	}
	// fを*littlefs.Fileに型アサーション
	if f, ok := f.(*littlefs.File); ok {
		return f, nil
	} else {
		return nil, os.ErrInvalid
	}

}

func NewFileSystem() (*littlefs.LFS, func(), error) {
	// create/format/mount the filesystem
	bd := tinyfs.NewMemoryDevice(64, 256, 2048)
	fs := littlefs.New(bd).Configure(&littlefs.Config{
		CacheSize:     128,
		LookaheadSize: 128,
		BlockCycles:   500,
	})
	if err := fs.Format(); err != nil {
		return nil, nil, err
	}
	if err := fs.Mount(); err != nil {
		return nil, nil, err
	}
	unmount := func() {
		if err := fs.Unmount(); err != nil {
			fmt.Println("Could not unmount", err)
		}
	}
	return fs, unmount, nil
}
