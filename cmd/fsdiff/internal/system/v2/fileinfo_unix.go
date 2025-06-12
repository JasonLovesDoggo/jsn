//go:build unix

package v2

import (
	"io/fs"
	"syscall"

	"golang.org/x/sys/unix"
)

// File attribute flags for ext2/3/4 filesystems
const (
	FS_SECRM_FL     = 0x00000001 // Secure deletion
	FS_UNRM_FL      = 0x00000002 // Undelete
	FS_COMPR_FL     = 0x00000004 // Compress file
	FS_SYNC_FL      = 0x00000008 // Synchronous updates
	FS_IMMUTABLE_FL = 0x00000010 // Immutable file
	FS_APPEND_FL    = 0x00000020 // Append only
	FS_NODUMP_FL    = 0x00000040 // Do not dump file
	FS_NOATIME_FL   = 0x00000080 // Do not update atime
)

func GetFileInfo(path string, info fs.FileInfo) *FileInfo {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return &FileInfo{}
	}

	perm := uint16(info.Mode().Perm() & 0777)

	// Convert special mode bits to their traditional octal representation
	var special uint16
	if info.Mode()&fs.ModeSetuid != 0 {
		special |= 0o4000
	}
	if info.Mode()&fs.ModeSetgid != 0 {
		special |= 0o2000
	}
	if info.Mode()&fs.ModeSticky != 0 {
		special |= 0o1000
	}

	meta := &FileMetadata{}
	hasMetadata := false

	// Batch xattr collection in one pass to reduce syscalls
	xattrs := getAllXattrs(path)

	// Extract security-specific xattrs from the batch
	if selinux, ok := xattrs["security.selinux"]; ok {
		meta.SELinux = map[string]string{"label": selinux}
		hasMetadata = true
		delete(xattrs, "security.selinux") // Remove from general xattrs
	}

	if caps, ok := xattrs["security.capability"]; ok {
		meta.Capabilities = caps
		hasMetadata = true
		delete(xattrs, "security.capability") // Remove from general xattrs
	}

	// Extract POSIX ACLs from xattrs
	var acls []string
	if defaultAcl, ok := xattrs["system.posix_acl_default"]; ok {
		acls = append(acls, "default:"+defaultAcl)
		delete(xattrs, "system.posix_acl_default")
	}
	if accessAcl, ok := xattrs["system.posix_acl_access"]; ok {
		acls = append(acls, "access:"+accessAcl)
		delete(xattrs, "system.posix_acl_access")
	}
	if len(acls) > 0 {
		meta.ACLs = acls
		hasMetadata = true
	}

	// Store remaining xattrs
	if len(xattrs) > 0 {
		meta.Xattrs = xattrs
		hasMetadata = true
	}

	// Get file attributes for regular files and directories only
	if stat.Mode&syscall.S_IFREG != 0 || stat.Mode&syscall.S_IFDIR != 0 {
		if attrs, err := getFileAttrs(path); err == nil {
			meta.Immutable = attrs&FS_IMMUTABLE_FL != 0
			meta.AppendOnly = attrs&FS_APPEND_FL != 0
			meta.NoDump = attrs&FS_NODUMP_FL != 0
			meta.Compressed = attrs&FS_COMPR_FL != 0

			if meta.Immutable || meta.AppendOnly || meta.NoDump || meta.Compressed {
				hasMetadata = true
			}
		}
	}

	// Only keep metadata if something is present
	if !hasMetadata {
		meta = nil
	}

	return &FileInfo{
		Permissions: perm | special,
		OwnerID:     stat.Uid,
		GroupID:     stat.Gid,
		Metadata:    meta,
	}
}

// getXattr fetches an extended attribute value as string
func getXattr(path, attr string) string {
	// First call to get size
	sz, err := unix.Getxattr(path, attr, nil)
	if err != nil || sz <= 0 {
		return ""
	}
	buf := make([]byte, sz)
	_, err = unix.Getxattr(path, attr, buf)
	if err != nil {
		return ""
	}
	return string(buf)
}

// listXattr returns a list of xattr keys
func listXattr(path string) []string {
	// First call to get size
	sz, err := unix.Listxattr(path, nil)
	if err != nil || sz <= 0 {
		return nil
	}
	buf := make([]byte, sz)
	sz, err = unix.Listxattr(path, buf)
	if err != nil || sz <= 0 {
		return nil
	}

	// Split null-delimited list
	var keys []string
	start := 0
	for i, b := range buf {
		if b == 0 {
			if i > start {
				keys = append(keys, string(buf[start:i]))
			}
			start = i + 1
		}
	}
	return keys
}

// getFileAttrs gets file attributes using ioctl (for ext2/3/4 filesystems)
func getFileAttrs(path string) (uint32, error) {
	fd, err := unix.Open(path, unix.O_RDONLY, 0)
	if err != nil {
		return 0, err
	}
	defer unix.Close(fd)

	attrs, err := unix.IoctlGetUint32(fd, unix.FS_IOC_GETFLAGS)
	return attrs, err
}

// getAllXattrs efficiently retrieves all extended attributes in one pass
func getAllXattrs(path string) map[string]string {
	keys := listXattr(path)
	if len(keys) == 0 {
		return nil
	}

	xattrs := make(map[string]string, len(keys))
	for _, key := range keys {
		if val := getXattr(path, key); val != "" {
			xattrs[key] = val
		}
	}

	return xattrs
}
