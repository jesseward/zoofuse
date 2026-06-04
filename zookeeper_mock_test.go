package main

import (
	"github.com/samuel/go-zookeeper/zk"
	"github.com/stretchr/testify/mock"
)

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
	args := m.zk.Called(path, data, version)
	return args.Get(0).(*zk.Stat), args.Error(1)
}
