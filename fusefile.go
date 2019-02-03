package main

import (
	"time"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	log "github.com/sirupsen/logrus"
)

// FuseFile is the file object container. FuseFile implements the bare minmum system calls (`read` and `write`)
type FuseFile struct {
	nodefs.File
	data []byte     // contents of the file
	attr *fuse.Attr // file mode attributes
	zh   Zoohandler // reference to the zookeeper connection
	path string     // path of the file
}

func NewFuseFile(data []byte, mode uint32, path string, zh Zoohandler) *FuseFile {
	now := uint64(time.Now().Unix())
	attr := &fuse.Attr{
		Mode:  mode | IfRegRW,
		Size:  uint64(len(data)),
		Atime: now,
		Mtime: now,
		Owner: *fuse.CurrentOwner(),
	}
	return &FuseFile{data: data,
		File: nodefs.NewDefaultFile(),
		attr: attr,
		path: path,
		zh:   zh}
}

// Read implements a simple buffer read operation required for file access.
func (f *FuseFile) Read(buf []byte, off int64) (fuse.ReadResult, fuse.Status) {
	end := int(off) + int(len(buf))
	if end > len(f.data) {
		end = len(f.data)
	}

	return fuse.ReadResultData(f.data[off:end]), fuse.OK
}

// Write pushes the []byte array into the Zookeeper node. An array size of 0 is a (silent) no-op
func (f *FuseFile) Write(content []byte, off int64) (uint32, fuse.Status) {

	// save a round trip to zk in the event the content length is 0
	if len(content) == 0 {
		return 0, fuse.OK
	}

	// TODO: what is the implication of Set(..) with a version of -1. My assumption is that
	// it overwrites (resets) the current znode version in ZK.
	stat, err := f.zh.Set(f.path, content, -1)
	if err != nil {
		log.WithFields(log.Fields{
			"path": f.path,
			"err":  err,
		}).Warn("Failed to Set znode data")
		return 0, fuse.EIO
	}

	f.attr.Size = uint64(stat.DataLength)
	return uint32(stat.DataLength), fuse.OK
}
