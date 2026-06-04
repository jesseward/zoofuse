package main

import (
	"fmt"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/samuel/go-zookeeper/zk"
)

const (
	// MaxZnodeData represents the maximum size of the data object per znode
	// See "jute.maxbuffer" at https://zookeeper.apache.org/doc/r3.3.3/zookeeperAdmin.html#Unsafe+Options
	MaxZnodeData = 1048575

	// ZNodeMarker is a special file that provides the contents of the parent znode.
	// If a znode is deemed to be a directory (has children), the ZNodeMarker file is used
	// as a file in order to allow access to data. Required since standard directories do not
	// allow file content.
	ZNodeMarker = "__znode_data__"
)

// Zoohandler defines the minimun actions required to fetch, delete and create entries in the Zookeeper directory.
type Zoohandler interface {
	Close()

	// GetChildren Fetches all child nodes for a target Zookeeper node.
	Children(path string) ([]string, *zk.Stat, error)

	// Create, inserts a znode into the Zookeeper directory.
	Create(path string, data []byte, flags int32, acl []zk.ACL) (string, error)

	// Delete removes a single znode from the tree.
	Delete(path string, version int32) error

	// Exists tests whether the znodes exits, returns boolean and if present, the zk.Stat object.
	Exists(path string) (bool, *zk.Stat, error)

	// Get retrieves a single znode entry from the directory.
	Get(path string) ([]byte, *zk.Stat, error)

	Set(path string, data []byte, version int32) (*zk.Stat, error)
}

// ZooHandle functions implement the Zoohandler interface. This orchestrates all communication to the Zookeeper directory.
type ZooHandle struct {
	zk     Zoohandler // Connection object to ZK
	ZKRoot string     // chroot/alias the root of the zookeeper directory to an alternate location (default is /).
}

// ZKPath performs the translation from a fuse directory/file path to a path suitable for the Zookeeper tree. Additionally
// this also supports the ability to "chroot" (`ZKRoot`) a Zookeeper znode to the root "/" view. For example if you were to
// ZKRoot "/my/zookeeper/sub/znode" , the Fuse file system will condsider  "/my/zookeeper/sub/znode" as "/" and entries above
// this path are not visibile within Fuse.
func (z *ZooHandle) ZKPath(filePath string) string {
	cleanPath := path.Clean("/" + filePath)
	cleanPath = strings.TrimSuffix(cleanPath, "/"+ZNodeMarker)
	return path.Join("/", z.ZKRoot, cleanPath)
}

// Close releases the Zookeeper connection.
func (z *ZooHandle) Close() {
	z.zk.Close()
}

// Delete the node with the given path
func (z *ZooHandle) Delete(path string, version int32) error {
	path = z.ZKPath(path)
	slog.Debug("Delete", "path", path)
	return z.zk.Delete(path, version)
}

// Create a node with the given path
func (z *ZooHandle) Create(path string, data []byte, flags int32, acl []zk.ACL) (string, error) {
	path = z.ZKPath(path)
	slog.Debug("Create", "path", path, "dataLen", len(data), "flags", flags)
	return z.zk.Create(path, data, flags, acl)
}

// Children returns the given children list and the stat of the znode path
func (z *ZooHandle) Children(path string) ([]string, *zk.Stat, error) {
	path = z.ZKPath(path)
	slog.Debug("Children", "path", path)
	return z.zk.Children(path)
}

// Exists returns a bool based on the presence of the znode. Since it also returns the zk.Stat it is the preferred call for
// light(er)weight state checking against ZK (instead of say zk.Get(..), which includes the data payload)
func (z *ZooHandle) Exists(path string) (bool, *zk.Stat, error) {
	path = z.ZKPath(path)
	slog.Debug("Exists", "path", path)
	return z.zk.Exists(path)
}

// Get return the data and the stat of the node of the given path.
func (z *ZooHandle) Get(path string) ([]byte, *zk.Stat, error) {
	path = z.ZKPath(path)
	slog.Debug("Get", "path", path)
	return z.zk.Get(path)
}

// Set writes data into a target znode of the given path.
func (z *ZooHandle) Set(path string, data []byte, version int32) (*zk.Stat, error) {
	if len(data) > MaxZnodeData {
		return nil, fmt.Errorf("length of data payload exceeds allowable limit (%d)", MaxZnodeData)
	}
	path = z.ZKPath(path)
	slog.Debug("Set", "path", path, "dataLen", len(data))
	return z.zk.Set(path, data, version)
}

func NewZooHandler(zkConnection []string, zkRoot string) (*ZooHandle, error) {
	c, _, err := zk.Connect(zkConnection, 5*time.Second)

	if err != nil {
		return nil, err
	}
	return &ZooHandle{
		zk:     c,
		ZKRoot: zkRoot,
	}, nil
}
