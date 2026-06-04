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
	bytes := []byte{1, 2, 3}
	ff := NewFuseFile(bytes, 0, "mock/path", mockZooKeeper, false)

	// assert that we can read
	buf := make([]byte, 2)
	res, status := ff.Read(buf, 0)
	assert.Equal(t, fuse.OK, status)
	assert.Equal(t, 2, res.Size())

	// assert that we do not panic when we attempt to read beyond the buffer length
	assert.NotPanics(t, func() {
		res, status := ff.Read(buf, int64(len(bytes)+1))
		assert.Equal(t, fuse.OK, status)
		assert.Equal(t, 0, res.Size())
	})
}

// TestWrite creates a FuseFile ojbect and exercises the Write() function.
func TestWrite(t *testing.T) {
	mockZooKeeper := &MockZooHandle{
		zk: mock.Mock{},
	}

	bytes := make([]byte, 3)
	ff := NewFuseFile(bytes, 0, "mock/path", mockZooKeeper, true) // Must be RW

	mockZooKeeper.zk.On("Set", "mock/path", bytes, int32(-1)).Return(&zk.Stat{DataLength: int32(len(bytes))}, nil)

	// assert that we send 3 bytes into the writer and status out == fuse.OK
	size, stat := ff.Write(bytes, 0)
	assert.Equal(t, uint32(3), size)
	assert.Equal(t, fuse.OK, stat)
}

// TestWriteReadOnly verifies that write fails on RO file
func TestWriteReadOnly(t *testing.T) {
	mockZooKeeper := &MockZooHandle{
		zk: mock.Mock{},
	}

	bytes := make([]byte, 3)
	ff := NewFuseFile(bytes, 0, "mock/path", mockZooKeeper, false) // RO

	size, stat := ff.Write(bytes, 0)
	assert.Equal(t, uint32(0), size)
	assert.Equal(t, fuse.EACCES, stat)
}

// TestWriteOffset verifies that write at offset merges data correctly (Read-Modify-Write)
func TestWriteOffset(t *testing.T) {
	mockZooKeeper := &MockZooHandle{
		zk: mock.Mock{},
	}

	initialBytes := []byte("foo")
	ff := NewFuseFile(initialBytes, 0, "mock/path", mockZooKeeper, true)

	writeBytes := []byte("bar")
	expectedBytes := []byte("fbar")

	mockZooKeeper.zk.On("Set", "mock/path", expectedBytes, int32(-1)).Return(&zk.Stat{DataLength: int32(len(expectedBytes))}, nil)

	size, stat := ff.Write(writeBytes, 1)
	assert.Equal(t, uint32(3), size) // bytes written
	assert.Equal(t, fuse.OK, stat)
	assert.Equal(t, expectedBytes, ff.data)
}
