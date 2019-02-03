package main

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/samuel/go-zookeeper/zk"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
)

const (
	// MaxZnodeData represents the maximum size of the data object per znode
	MaxZnodeData = 1048576
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
	zk        Zoohandler // Connection object to ZK
	ZKRoot    string     // chroot/alias the root of the zookeeper directory to an alternate location (default is /).
	FuseMount string     // the full pathname of the fuse mounted filesystem
}

// ZKPath performs the translation from a fuse directory/file path to a path suitable for the Zookeeper tree. Additionally
// this also supports the ability to "chroot" (`ZKRoot`) a Zookeeper znode to the root "/" view. For example if you were to
// ZKRoot "/my/zookeeper/sub/znode" , the Fuse file system will condsider  "/my/zookeeper/sub/znode" as "/" and entries above
// this path are not visibile within Fuse.
// TODO: ugly++
func (z *ZooHandle) ZKPath(path string) string {
	rel, err := filepath.Rel(z.FuseMount, filepath.Join(z.FuseMount, path))
	if err != nil {
		log.Warn(err)
		return filepath.Join("/", z.ZKRoot)
	}
	return filepath.Join("/", z.ZKRoot, rel)
}

// Close releases the Zookeeper connection.
func (z *ZooHandle) Close() {
	z.zk.Close()
}

// Delete the node with the given path
func (z *ZooHandle) Delete(path string, version int32) error {
	path = z.ZKPath(path)
	log.WithFields(log.Fields{
		"path": path,
	}).Debug("")
	return z.zk.Delete(path, version)
}

// Create a node with the given path
func (z *ZooHandle) Create(path string, data []byte, flags int32, acl []zk.ACL) (string, error) {
	path = z.ZKPath(path)
	log.WithFields(log.Fields{
		"path":  path,
		"data":  data,
		"flags": flags,
		"acl":   acl,
	}).Debug("")
	return z.zk.Create(path, data, flags, acl)
}

// Children returns the given children list and the stat of the znode path
func (z *ZooHandle) Children(path string) ([]string, *zk.Stat, error) {
	path = z.ZKPath(path)
	log.WithFields(log.Fields{
		"path": path,
	}).Debug("")
	return z.zk.Children(path)
}

// Exists returns a bool based on the presence of the znode. Since it also returns the zk.Stat it is the preferred call for
// light(er)weight state checking against ZK (instead of say zk.Get(..), which includes the data payload)
func (z *ZooHandle) Exists(path string) (bool, *zk.Stat, error) {
	path = z.ZKPath(path)
	log.WithFields(log.Fields{
		"path": path,
	}).Debug("")
	return z.zk.Exists(path)
}

// Get return the data and the stat of the node of the given path.
func (z *ZooHandle) Get(path string) ([]byte, *zk.Stat, error) {
	path = z.ZKPath(path)
	log.WithFields(log.Fields{
		"path": path,
	}).Debug("")
	return z.zk.Get(path)
}

// Set writes data into a target znode of the given path.
func (z *ZooHandle) Set(path string, data []byte, version int32) (*zk.Stat, error) {
	if len(data) > MaxZnodeData {
		return nil, fmt.Errorf("length of data payload exceeds allowable limit (%d)", MaxZnodeData)
	}
	path = z.ZKPath(path)
	log.WithFields(log.Fields{
		"path": path,
	}).Debug("")
	return z.zk.Set(path, data, version)
}

// MockZooHandle provides a struct with functions that implement the ZooHandle interface, providing capabability to stub out the
// communication path to ZK (via mock.Mock)
type MockZooHandle struct {
	zk        mock.Mock // Connection object to ZK
	ZKRoot    string    // chroot/alias the root of the zookeeper directory to an alternate location (default is /).
	FuseMount string    // The full path of the fuse mount.
}

// Close mocks Zoohandler.Close
func (m *MockZooHandle) Close() {
	m.zk.Called()
}

// Children mocks Zoohandler.Children
func (m *MockZooHandle) Children(path string) ([]string, *zk.Stat, error) {
	args := m.zk.Called(path)
	return args.Get(0).([]string), args.Get(1).(*zk.Stat), args.Error(2)
}

// Get mocks Zoohandler.Get
func (m *MockZooHandle) Get(path string) ([]byte, *zk.Stat, error) {
	args := m.zk.Called(path)
	return args.Get(0).([]byte), args.Get(1).(*zk.Stat), args.Error(2)
}

// Create mocks Zoohandler.Create
func (m *MockZooHandle) Create(path string, data []byte, flags int32, acl []zk.ACL) (string, error) {
	args := m.zk.Called(path, data, flags, acl)
	return args.String(0), args.Error(1)
}

func (m *MockZooHandle) Delete(path string, version int32) error {
	args := m.zk.Called(path)
	return args.Error(0)
}

func (m *MockZooHandle) Exists(path string) (bool, *zk.Stat, error) {
	args := m.zk.Called(path)
	return args.Bool(0), args.Get(1).(*zk.Stat), args.Error(2)
}

func (m *MockZooHandle) Set(path string, data []byte, version int32) (*zk.Stat, error) {
	args := m.zk.Called(path)
	return args.Get(0).(*zk.Stat), args.Error(1)
}

func NewZooHandler(zkConnection []string, zkRoot, fuseMount string) (*ZooHandle, error) {
	c, _, err := zk.Connect(zkConnection, 5*time.Second)

	if err != nil {
		return nil, err
	}
	return &ZooHandle{
		zk:        c,
		ZKRoot:    zkRoot,
		FuseMount: fuseMount,
	}, nil
}
