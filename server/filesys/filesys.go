package filesys

import (
	"os"

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
