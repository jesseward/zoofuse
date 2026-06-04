package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/hanwen/go-fuse/fuse/pathfs"
)

var programLevel = new(slog.LevelVar)

func setupLogger(logFile string, debug bool) (*os.File, error) {
	var w io.Writer = os.Stdout
	var logH *os.File
	var err error

	if logFile != "" {
		logH, err = os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
		if err != nil {
			return nil, err
		}
		w = logH
	}

	if debug {
		programLevel.Set(slog.LevelDebug)
	} else {
		programLevel.Set(slog.LevelInfo)
	}

	opts := &slog.HandlerOptions{
		Level:     programLevel,
		AddSource: true,
	}

	logger := slog.New(slog.NewTextHandler(w, opts))
	slog.SetDefault(logger)

	return logH, nil
}

func banner(rootfs, zk, zkchroot, logFile string, rw bool) {
	b := `
	·▄▄▄▄•            ·▄▄▄▄• ▄▌.▄▄ · ▄▄▄ .
	▪▀·.█▌▪     ▪     ▐▄▄·█▪██▌▐█ ▀. ▀▄.▀·
	▄█▀▀▀• ▄█▀▄  ▄█▀▄ ██▪ █▌▐█▌▄▀▀▀█▄▐▀▀▪▄
	█▌▪▄█▀▐█▌.▐▌▐█▌.▐▌██▌.▐█▄█▌▐█▄▪▐█▐█▄▄▌
	·▀▀▀ • ▀█▄▀▪ ▀█▄▀▪▀▀▀  ▀▀▀  ▀▀▀▀  ▀▀▀ 

|FuseRoot   > %s
|Zookeeper  > %s
|ZK Chroot  > %s
|RW enabled > %t
|Log file   > %s

If you have a lingering mount upon exit, try 'fusemount -u %s' to clean-up.

booted...
`
	fmt.Printf(b, rootfs, zk, zkchroot, rw, logFile, rootfs)
}

func main() {
	// the stretchr/testify/mock package introduces testing flags into the default
	// flagset. Creation of this flagset is to workaround this, so the unwanted flags are
	// not displayed..
	cmd := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	var Usage = func() {
		fmt.Fprintf(cmd.Output(), "Usage: %s [OPTION]... [MOUNTPOINT] \n", os.Args[0])
		cmd.PrintDefaults()
	}
	cmd.Usage = Usage

	var zkChroot = cmd.String("zkroot", "/", "Alias the root Zookeeper tree to an alternate path")
	var zkConn = cmd.String("zkconn", "127.0.0.1:2181", "Zookeeper connection string")
	var isReadWrite = cmd.Bool("rw", false, "Enable a read/write ZooFuse filesystem (default is READONLY)")
	var logFile = cmd.String("logfile", "", "Enable logging to a target file, otherwise STDOUT")
	var debug = cmd.Bool("debug", false, "Enable verbose debug logging (default disabled)")
	cmd.Parse(os.Args[1:])

	if len(cmd.Args()) < 1 {
		Usage()
		os.Exit(1)
	}

	logH, err := setupLogger(*logFile, *debug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup logger: %v\n", err)
		os.Exit(1)
	}
	if logH != nil {
		defer logH.Close()
	}

	zooHandler, err := NewZooHandler([]string{*zkConn}, *zkChroot)
	if err != nil {
		slog.Error("Failed to create ZooHandler", "err", err)
		os.Exit(1)
	}

	fuseFS := FuseFS{
		FileSystem:  pathfs.NewDefaultFileSystem(),
		zh:          zooHandler,
		FuseRoot:    cmd.Arg(0),
		FSServer:    nil,
		IsReadWrite: *isReadWrite,
	}

	err = fuseFS.Mount(nil)
	if err != nil {
		slog.Error("Failed to mount FUSE", "err", err)
		os.Exit(1)
	}
	defer fuseFS.Unmount()

	// attempt self healing logic batch capturing sig int/term.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fuseFS.Unmount()
		os.Exit(1)
	}()

	banner(fuseFS.FuseRoot, *zkConn, *zkChroot, *logFile, *isReadWrite)
	fuseFS.Serve()
}
