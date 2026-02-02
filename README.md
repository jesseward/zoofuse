
```
	·▄▄▄▄•            ·▄▄▄▄• ▄▌.▄▄ · ▄▄▄ .
	▪▀·.█▌▪     ▪     ▐▄▄·█▪██▌▐█ ▀. ▀▄.▀·
	▄█▀▀▀• ▄█▀▄  ▄█▀▄ ██▪ █▌▐█▌▄▀▀▀█▄▐▀▀▪▄
	█▌▪▄█▀▐█▌.▐▌▐█▌.▐▌██▌.▐█▄█▌▐█▄▪▐█▐█▄▄▌
	·▀▀▀ • ▀█▄▀▪ ▀█▄▀▪▀▀▀  ▀▀▀  ▀▀▀▀  ▀▀▀ 
```

ZooFUSE implements a FUSE filesystem that is backed by a ZooKeeper directory tree. This allows the user to navigate through the Zookeeper tree structure using the same command line tools commonly used to navigate a local file system.

Some of the current features include

* Easily mount a remote Zookeeper instance onto a local FUSE filesystem
* Ability to "chroot" or jail a Zookeeper path to the Fuse root (see `zkroot` flag). For example if your znode path of interest is /my/important/data , specifying `-zkroot /my/important/data` will map that tree structure as the root of your FUSE mount . The aim here is to limit one's exposure to the global Zookeeper directory
* Exposes a read-only mode (by default). When launched in read-only mode, file permissions are strict with `+w` capabilities stripped. If you wish to read/write to FUSE, launch zoofuse with the `-rw` flag.
* Ability to read or create znode information. Note that the znode size, `ctime` and `mtime` attributes are appropriate mapped to the FUSE file modes.

**Beware that ZooFUSE supports both read and write operations, making it extremely easy to modify data inside of the  live Zookeeper tree**

Due to this, ZooFUSE defaults to setting file and directory modes as read-only (modes 0444 and 0555). In order to expose both read + write operations, launch ZooFUSe must be launched with the `-rw` flag.

Runtime options
==========
Requirements to launch: 
* An operating system with Libfuse available (a common default in Linux variants). To run on MacOS you will require `osxfuse` or some other Fuse implementation.
* a working Zookeeper instance that is accessible (typically TCP port 2181) from your FUSE host.

```
Usage: ./zoofuse [OPTION]... [MOUNTPOINT]
  -debug
        Enable verbose debug logging (default disabled)
  -logfile string
        Enable logging to a target file, otherwise STDOUT
  -rw
        Enable a read/write ZooFuse filesystem (default is READONLY)
  -zkconn string
        Zookeeper connection string (default "127.0.0.1:2181")
  -zkroot string
        Alias the root Zookeeper tree to an alternate path (default "/")
```

Caveats
=======

*MacOS*

MacOS does not provide native support for FUSE. In order to run this client on MacOS, you must install https://osxfuse.github.io/ 

*Directories*

Zookeeper does not have the notion of a Directory. In order to simulate and map a znode to a file system directory object, a Get (to Zookeeper) is made against the znode, if the target znode > 0 children, this znode is considered to be a "directory" (file type  set to S_IFDIR). This leads to race conditions where certain znodes/file objects may flip back and forth between S_IFDIR and S_IFREG (regular file).

In order to read the contents of a znode that has been mapped as a filesystem directory, Zoofuse places a special file into the directory named `__znode_data__`. This file exposes the contents of a "directory" znode.
