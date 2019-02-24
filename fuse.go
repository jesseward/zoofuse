package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	"github.com/samuel/go-zookeeper/zk"
	log "github.com/sirupsen/logrus"
)

const (
	// IfDirRW file mask for RW directories
	IfDirRW = uint32(0755)
	// IfDirRO file mask for RO directories
	IfDirRO = uint32(0555)

	// IfRegRW file mask for RW files
	IfRegRW = uint32(0644)
	// IfRegRO file mask for RO files
	IfRegRO = uint32(0444)

	// MaxConcurrentRequests represents max number of parallel requests to send to the remote ZK directory.
	// This attempts to speed up OpenDir requests against trees that have many children.
	MaxConcurrentRequests = 25
)

// FuseFS is the container for the filesystem. This is built-upon the go-fuse "pathfs" machinery. The other notable
// property is the `zh` reference, this accepts anytype that implements the `Zoohandler` interface.
type FuseFS struct {
	pathfs.FileSystem            // Maintain reference to go-fuse pathfs
	zh                Zoohandler // ZK connection reference
	FuseRoot          string
	FSServer          *fuse.Server
	IsReadWrite       bool // Will write actions be enabled
}

// dirPermissions returns the appropriate directory permission mask
func dirPermissions(isReadWrite bool) uint32 {
	if isReadWrite {
		return IfDirRW
	}
	return IfDirRO
}

// filePermissions returns the appropriate file permission mask
func filePermissions(isReadWrite bool) uint32 {
	if isReadWrite {
		return IfRegRW
	}
	return IfRegRO
}

// GetAttr manages file system attributes for each file object. On each GetAttr request
// we perform a query (Get) against the znode to ensure it exists. If the znode exists
// this assigns the attributes for the file object. A further check is made to determine
// if the znode has any children, if so the S_IFDIR file mode is set.
func (f *FuseFS) GetAttr(path string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	if path == "" {
		return &fuse.Attr{
			Mode: fuse.S_IFDIR | dirPermissions(f.IsReadWrite),
		}, fuse.OK
	}

	found, stat, err := f.zh.Exists(path)

	if err != nil {
		log.Error(err)
		return nil, fuse.ENOENT
	}

	if !found {
		log.WithFields(log.Fields{
			"path": path,
		}).Warn("znode does not exist")
		return nil, fuse.ENOENT
	}

	var fa fuse.Attr

	// if a znode has 1 or more assigned child nodes, that znode is considered to be a directory.
	// Additionally force IFREG filemode if path name matches the magic/special ZNodeMarker.
	if strings.HasSuffix(path, ZNodeMarker) {
		// marker file is always RO
		fa.Mode = fuse.S_IFREG | IfRegRO
	} else if stat.NumChildren == 0 {
		fa.Mode = fuse.S_IFREG | filePermissions(f.IsReadWrite)
	} else {
		fa.Mode = fuse.S_IFDIR | dirPermissions(f.IsReadWrite)
	}

	// additional file attributues populated from the znode (stat) data.
	fa.Size = uint64(stat.DataLength)
	fa.Mtime = uint64(stat.Mtime / 1000)
	fa.Ctime = uint64(stat.Ctime / 1000)
	return &fa, fuse.OK
}

// OpenDir builds the current working directory from the remote ZK tree. This is done by
// performing a fetch of all `Children` znodes for the current `path`. The only file
// attributes set here is the `mode` (S_IFDIR or S_IFREG)
func (f *FuseFS) OpenDir(path string, context *fuse.Context) ([]fuse.DirEntry, fuse.Status) {
	children, _, err := f.zh.Children(path)
	if err != nil {
		log.WithFields(log.Fields{
			"path": path,
			"err":  err,
		}).Error("failed to fetch children")
		return nil, fuse.ENOENT
	}

	var dirEntries []fuse.DirEntry
	dirEntries = append(dirEntries, fuse.DirEntry{Name: ZNodeMarker, Mode: fuse.S_IFREG})

	if len(children) == 0 {
		return dirEntries, fuse.OK
	}

	maxWorkers := MaxConcurrentRequests

	if maxWorkers > len(children) {
		maxWorkers = len(children)
	}

	chanLimiter := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup
	for _, child := range children {
		wg.Add(1)
		go func(path, directory string) {
			defer wg.Done()
			chanLimiter <- struct{}{}

			defer func() {
				<-chanLimiter
			}()

			found, stat, err := f.zh.Exists(filepath.Join(path, string(os.PathSeparator), directory))
			if err != nil {
				log.Error(err)
				return
			}

			if !found {
				log.WithFields(log.Fields{
					"path": path,
				}).Error("znode does not exist")
				return
			}

			dirEntry := fuse.DirEntry{Name: directory}
			if stat.NumChildren > 0 {
				dirEntry.Mode = fuse.S_IFDIR
			} else {
				dirEntry.Mode = fuse.S_IFREG
			}
			dirEntries = append(dirEntries, dirEntry)
		}(path, child)
	}
	wg.Wait()

	return dirEntries, fuse.OK
}

// Utimens is called after the creation of a file. This syscall sets the timestamps in nanos.
func (f *FuseFS) Utimens(name string, Atime *time.Time, Mtime *time.Time, context *fuse.Context) (code fuse.Status) {
	return fuse.OK
}

func (f *FuseFS) Truncate(name string, size uint64, context *fuse.Context) (code fuse.Status) {
	return fuse.OK
}

// Create new file object. This creates a new znode inside ZK with an emtpy set of data. Create also
// returns a new FuseFile struct that provides read/write capabilities.
func (f *FuseFS) Create(path string, flags uint32, mode uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	_, err := f.zh.Create(path, nil, int32(0), zk.WorldACL(zk.PermAll))

	if err != nil {
		log.WithFields(log.Fields{
			"path": path,
			"err":  err,
		}).Error("failed to create znode.")
		return nil, fuse.ENOENT
	}
	return NewFuseFile(nil, IfRegRW, path, f.zh), fuse.OK
}

// Open a filedescriptor for read or write ops. Open returns a new FuseFile (nodefs.File), populated with the
// current znode payload (or empty)
func (f *FuseFS) Open(path string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	data, _, err := f.zh.Get(path)
	if err != nil {
		log.WithFields(log.Fields{
			"path": path,
			"err":  err,
		}).Error("unable to Get znode from zookeeper")
		return nil, fuse.ENOENT
	}
	return NewFuseFile([]byte(data), IfRegRW, path, f.zh), fuse.OK
}

// Unlink removes the file/znode from the tree.
func (f *FuseFS) Unlink(path string, context *fuse.Context) (code fuse.Status) {
	// gaurd ensures that a user cannot remove the ZNodeMarker file
	if strings.HasSuffix(path, ZNodeMarker) {
		return fuse.EPERM
	}

	err := f.zh.Delete(path, -1)
	if err != nil {
		log.WithFields(log.Fields{
			"path": path,
			"err":  err,
		}).Error("unable to Delete znode from zookeeper")
		return fuse.EIO
	}
	return fuse.OK
}

// Rmdir removes a znode and its children.
func (f *FuseFS) Rmdir(path string, context *fuse.Context) (code fuse.Status) {
	found, stat, err := f.zh.Exists(path)
	if err != nil {
		log.Error(err)
		return fuse.ENOENT
	}

	if !found {
		log.WithFields(log.Fields{
			"path": path,
		}).Error("znode does not exist")
		return fuse.ENOENT
	}

	if stat.NumChildren == 0 {
		log.WithFields(log.Fields{
			"path": path,
		}).Error("ENOTDIR - skipping, number of children is 0.")
		return fuse.ENOTDIR
	}

	err = f.zh.Delete(path, -1)
	if err != nil {
		log.WithFields(log.Fields{
			"path": path,
			"err":  err,
		}).Error("received error when deleting directory")
		return fuse.ENOENT
	}
	return fuse.OK
}

// Mount manages the creation of the FileSystem.
func (f *FuseFS) Mount(opts []string) error {

	log.Infof("mount FUSE filesystem at FuseRoot=%s", f.FuseRoot)
	nfs := pathfs.NewPathNodeFs(f, nil)
	fsopts := nodefs.NewOptions()
	fsopts.EntryTimeout = 1 * time.Second
	fsopts.AttrTimeout = 1 * time.Second
	fsopts.NegativeTimeout = 1 * time.Second
	conn := nodefs.NewFileSystemConnector(nfs.Root(), fsopts)

	server, err := fuse.NewServer(conn.RawFS(), f.FuseRoot, nil)
	if err != nil {
		return err
	}
	f.FSServer = server
	return nil
}

// Serve initiates the FUSE loop. This is a blocking call.
func (f *FuseFS) Serve() {
	f.FSServer.Serve()
}

// Unmount drops the currently mounted Fuse filesystem. This should be called at exit. Note there is still room for data that is left behind, if
// a user has an open file handle that resides within FUSE, the file system will not cleanly unmount.
// TODO: add check for open files under Root mount?
func (f *FuseFS) Unmount() {
	log.Infof("Unmounting FUSE filesystem at FuseRoot=%s ...", f.FuseRoot)
	f.FSServer.Unmount()
}
