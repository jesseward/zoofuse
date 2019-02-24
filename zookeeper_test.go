package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestZKPath verifies that path naming and normalization is handled correctly by teh ZKPath function
func TestZKPath(t *testing.T) {

	mockClient := &MockZooHandle{
		zk: mock.Mock{},
	}

	zh := ZooHandle{zk: mockClient, ZKRoot: "/", FuseMount: "/mnt/fuse"}

	// testing without a chroot zookeeper config.
	zh.ZKRoot = "/"
	assert.Equal(t, "/", zh.ZKPath(""))
	assert.Equal(t, "/", zh.ZKPath("/"))
	assert.Equal(t, "/test-path", zh.ZKPath("/test-path/"))
	assert.Equal(t, "/test-path", zh.ZKPath("test-path"))
	assert.Equal(t, "/test-path/sub-node", zh.ZKPath("test-path/sub-node"))
	assert.Equal(t, "/test-path/sub-node", zh.ZKPath("test-path/sub-node"))

	// limit the scope of ZK access via a jail/chroot.
	zh.ZKRoot = "/chroot"
	assert.Equal(t, "/chroot", zh.ZKPath("/"))
	assert.Equal(t, "/chroot/test-path", zh.ZKPath("test-path"))
	assert.Equal(t, "/chroot/test-path/sub-node", zh.ZKPath("test-path/sub-node"))
	assert.Equal(t, "/chroot/test-path/sub-node", zh.ZKPath("test-path/sub-node"+"/"+ZNodeMarker))
}
