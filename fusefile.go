package main

import (
	"log/slog"
	"sync"
	"time"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
)

// FuseFile is the file object container. FuseFile implements the bare minmum system calls (`read` and `write`)
type FuseFile struct {
	nodefs.File
	mu          sync.Mutex
	data        []byte     // contents of the file
	attr        *fuse.Attr // file mode attributes
	zh          Zoohandler // reference to the zookeeper connection
	path        string     // path of the file
	isReadWrite bool
}

func NewFuseFile(data []byte, mode uint32, path string, zh Zoohandler, isReadWrite bool) *FuseFile {
	now := uint64(time.Now().Unix())
	attr := &fuse.Attr{
		Mode:  mode,
		Size:  uint64(len(data)),
		Atime: now,
		Mtime: now,
		Owner: *fuse.CurrentOwner(),
	}
	return &FuseFile{data: data,
		File:        nodefs.NewDefaultFile(),
		attr:        attr,
		path:        path,
		zh:          zh,
		isReadWrite: isReadWrite}
}

// Read implements a simple buffer read operation required for file access.
func (f *FuseFile) Read(buf []byte, off int64) (fuse.ReadResult, fuse.Status) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if off >= int64(len(f.data)) {
		return fuse.ReadResultData(nil), fuse.OK
	}
	end := int(off) + int(len(buf))
	if end > len(f.data) {
		end = len(f.data)
	}

	return fuse.ReadResultData(f.data[off:end]), fuse.OK
}

// Write pushes the []byte array into the Zookeeper node. An array size of 0 is a (silent) no-op. Returns
// the number of bytes written and the status of the errno returns to kernel.
func (f *FuseFile) Write(content []byte, off int64) (uint32, fuse.Status) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.isReadWrite {
		return 0, fuse.EACCES
	}
	// save a round trip to zk in the event the content length is 0
	if len(content) == 0 {
		return 0, fuse.OK
	}

	neededLength := int(off) + len(content)
	if neededLength > len(f.data) {
		newData := make([]byte, neededLength)
		copy(newData, f.data)
		f.data = newData
	}
	copy(f.data[off:], content)

	stat, err := f.zh.Set(f.path, f.data, -1)
	if err != nil {
		slog.Warn("Failed to Set znode data", "path", f.path, "err", err)
		return 0, fuse.EIO
	}

	f.attr.Size = uint64(stat.DataLength)
	return uint32(len(content)), fuse.OK
}
