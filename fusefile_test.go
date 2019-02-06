package main

import (
	"testing"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/samuel/go-zookeeper/zk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRead(t *testing.T) {
	mockZooKeeper := &MockZooHandle{
		zk: mock.Mock{},
	}
	bytes := make([]byte, 3)
	ff := NewFuseFile(bytes, 0, "mock/path", mockZooKeeper)

	// assert that we can read from the Nth byte (n=3).
	buf := []byte{}
	_, b := ff.Read(buf, 3)
	assert.Equal(t, fuse.Status(0), b, "return status was not 0")
	// assert that we panic when we attempt to read beyond the buffer length
	// TODO: is this a bug in go-fuse https://github.com/hanwen/go-fuse/blob/master/fuse/nodefs/files.go#L46
	// there is no upper boundry protection around the offset (off), so it allows to read beyond the buffer.
	// Though I am not sure if this would be hit in a normal situation...
	assert.Panics(t, func() { ff.Read(buf, int64(len(bytes)+1)) }, "did not panic when attempting to read beyond buffer")

}

// TestWrite creates a FuseFile ojbect and exercises the Write() function.
func TestWrite(t *testing.T) {
	mockZooKeeper := &MockZooHandle{
		zk: mock.Mock{},
	}

	bytes := make([]byte, 3)
	ff := NewFuseFile(bytes, 0, "mock/path", mockZooKeeper)

	mockZooKeeper.zk.On("Set", "mock/path", bytes, int32(-1)).Return(&zk.Stat{DataLength: int32(len(bytes))}, nil)

	// assert that we send 3 bytes into the writer and status out == fuse.OK
	size, stat := ff.Write(bytes, 0)
	assert.Equal(t, uint32(3), size)
	assert.Equal(t, fuse.OK, stat)
}
