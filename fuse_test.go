package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDirPermissions(t *testing.T) {
	assert.Equal(t, dirPermissions(true), IfDirRW)
	assert.Equal(t, dirPermissions(false), IfDirRO)

}

func TestFilePermissions(t *testing.T) {
	assert.Equal(t, filePermissions(true), IfRegRW)
	assert.Equal(t, filePermissions(false), IfRegRO)
}
