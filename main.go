package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/hanwen/go-fuse/fuse/pathfs"
	log "github.com/sirupsen/logrus"
)

func init() {

	log.SetFormatter(&log.TextFormatter{
		TimestampFormat: "2006-01-02T15:04:05.000",
		FullTimestamp:   true,
	})
	log.SetLevel(log.InfoLevel)
	log.SetReportCaller(true) // show calling method/line num (https://github.com/sirupsen/logrus/pull/850)

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

	if *logFile != "" {
		logH, err := os.OpenFile(*logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err == nil {
			log.SetOutput(logH)
		}
		defer logH.Close()
	}

	if *debug {
		log.SetLevel(log.DebugLevel)
	}

	zooHandler, err := NewZooHandler([]string{*zkConn}, *zkChroot, cmd.Arg(0))
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Fatal("Failed to create ZooHandler")
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
		log.WithFields(log.Fields{
			"err": err,
		}).Fatal("Failed to mount FUSE")
	}
	defer fuseFS.Unmount()

	// attempt self healing logic batch capturing sig int/term.
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fuseFS.Unmount()
		os.Exit(1)
	}()

	banner(fuseFS.FuseRoot, *zkConn, *zkChroot, *logFile, *isReadWrite)
	fuseFS.Serve()
}
