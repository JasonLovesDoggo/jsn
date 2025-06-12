//go:build unix

package v2

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestGetFileInfo_Basic(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/testfile"
	require.NoError(t, os.WriteFile(path, []byte("hi"), 0754))

	err := unix.Setxattr(path, "user.testattr", []byte("testvalue"), 0)
	require.NoError(t, err)

	info, err := os.Lstat(path)
	require.NoError(t, err)

	fi := GetFileInfo(path, info)

	require.Equal(t, uint16(0754), fi.Permissions&0777)
	require.NotZero(t, fi.OwnerID)
	require.NotZero(t, fi.GroupID)
	require.NotNil(t, fi.Metadata)
	require.Contains(t, fi.Metadata.Xattrs, "user.testattr")
	require.Equal(t, "testvalue", fi.Metadata.Xattrs["user.testattr"])
}

func TestGetFileInfo_EmptyXattrs(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/plainfile"
	require.NoError(t, os.WriteFile(path, []byte("plain"), 0600))

	info, err := os.Lstat(path)
	require.NoError(t, err)

	fi := GetFileInfo(path, info)
	require.Equal(t, uint16(0600), fi.Permissions&0777)
	require.NotZero(t, fi.OwnerID)
	require.NotZero(t, fi.GroupID)
	require.Nil(t, fi.Metadata) // no xattrs or selinux
}

func TestGetFileInfo_SELinux_SkipIfNotPresent(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/selinuxfile"
	require.NoError(t, os.WriteFile(path, []byte("a"), 0644))

	// Try to manually set a fake SELinux label (may fail without root)
	_ = unix.Setxattr(path, "security.selinux", []byte("system_u:object_r:tmp_t:s0"), 0)

	info, err := os.Lstat(path)
	require.NoError(t, err)

	fi := GetFileInfo(path, info)

	// Not all systems support SELinux — so we test presence *if available*
	if fi.Metadata != nil && fi.Metadata.SELinux != nil {
		require.Contains(t, fi.Metadata.SELinux["label"], ":")
	}
}

func TestGetFileInfo_Capabilities(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "capfile")
	require.NoError(t, os.WriteFile(path, []byte("test"), 0755))

	// Try to set file capabilities (may fail without root privileges)
	capData := []byte{0x01, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x00}
	err := unix.Setxattr(path, "security.capability", capData, 0)

	info, err := os.Lstat(path)
	require.NoError(t, err)

	fi := GetFileInfo(path, info)

	// Test capabilities are extracted from xattrs if present
	if fi.Metadata != nil && fi.Metadata.Capabilities != "" {
		assert.NotEmpty(t, fi.Metadata.Capabilities)
		// Should not appear in general xattrs since it's extracted separately
		assert.NotContains(t, fi.Metadata.Xattrs, "security.capability")
	}
}

func TestGetFileInfo_PosixACLs(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "aclfile")
	require.NoError(t, os.WriteFile(path, []byte("test"), 0644))

	// Try to set POSIX ACL (may fail without ACL support)
	aclData := []byte("user::rw-,group::r--,other::r--")
	_ = unix.Setxattr(path, "system.posix_acl_access", aclData, 0)

	info, err := os.Lstat(path)
	require.NoError(t, err)

	fi := GetFileInfo(path, info)

	// Test ACLs are extracted if present
	if fi.Metadata != nil && len(fi.Metadata.ACLs) > 0 {
		assert.Contains(t, fi.Metadata.ACLs[0], "access:")
		// Should not appear in general xattrs since it's extracted separately
		assert.NotContains(t, fi.Metadata.Xattrs, "system.posix_acl_access")
	}
}

func TestGetFileInfo_DirectoryACLs(t *testing.T) {
	tmp := t.TempDir()
	dirPath := filepath.Join(tmp, "acldir")
	require.NoError(t, os.Mkdir(dirPath, 0755))

	// Try to set default ACL for directory
	defaultAcl := []byte("user::rwx,group::r-x,other::r-x")
	_ = unix.Setxattr(dirPath, "system.posix_acl_default", defaultAcl, 0)

	info, err := os.Lstat(dirPath)
	require.NoError(t, err)

	fi := GetFileInfo(dirPath, info)

	// Test default ACLs are extracted for directories
	if fi.Metadata != nil && len(fi.Metadata.ACLs) > 0 {
		found := false
		for _, acl := range fi.Metadata.ACLs {
			if assert.Contains(t, acl, "default:") {
				found = true
				break
			}
		}
		if found {
			// Should not appear in general xattrs since it's extracted separately
			assert.NotContains(t, fi.Metadata.Xattrs, "system.posix_acl_default")
		}
	}
}

func TestGetFileInfo_FileAttributes(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "attrfile")
	require.NoError(t, os.WriteFile(path, []byte("test"), 0644))

	info, err := os.Lstat(path)
	require.NoError(t, err)

	fi := GetFileInfo(path, info)

	// File attributes should be checked for regular files
	// Most test environments won't have special attributes set,
	// but we can verify the fields exist and are boolean
	if fi.Metadata != nil {
		assert.IsType(t, false, fi.Metadata.Immutable)
		assert.IsType(t, false, fi.Metadata.AppendOnly)
	}
}

func TestGetFileInfo_MultipleXattrs(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "multiattr")
	require.NoError(t, os.WriteFile(path, []byte("test"), 0644))

	// Set multiple extended attributes
	require.NoError(t, unix.Setxattr(path, "user.attr1", []byte("value1"), 0))
	require.NoError(t, unix.Setxattr(path, "user.attr2", []byte("value2"), 0))
	require.NoError(t, unix.Setxattr(path, "user.attr3", []byte("value3"), 0))

	info, err := os.Lstat(path)
	require.NoError(t, err)

	fi := GetFileInfo(path, info)

	require.NotNil(t, fi.Metadata)
	require.NotNil(t, fi.Metadata.Xattrs)
	assert.Equal(t, "value1", fi.Metadata.Xattrs["user.attr1"])
	assert.Equal(t, "value2", fi.Metadata.Xattrs["user.attr2"])
	assert.Equal(t, "value3", fi.Metadata.Xattrs["user.attr3"])
	assert.Len(t, fi.Metadata.Xattrs, 3)
}

func TestGetFileInfo_SpecialPermissions(t *testing.T) {
	tmp := t.TempDir()

	// Test setuid file
	suidPath := filepath.Join(tmp, "suidfile")
	require.NoError(t, os.WriteFile(suidPath, []byte("test"), 0755))
	require.NoError(t, os.Chmod(suidPath, 0755|os.ModeSetuid))

	info, err := os.Lstat(suidPath)
	require.NoError(t, err)
	fi := GetFileInfo(suidPath, info)

	// Check that the file mode contains the setuid bit
	assert.True(t, info.Mode()&os.ModeSetuid != 0, "File should have setuid bit set")
	// The permissions field should have the setuid bit (4755)
	assert.True(t, fi.Permissions&PERM_SETUID != 0, "Should detect setuid bit in permissions")
	assert.Equal(t, uint16(PERM_SETUID|0o755), fi.Permissions, "Should have setuid bit set")

	// Test setgid file
	sgidPath := filepath.Join(tmp, "sgidfile")
	require.NoError(t, os.WriteFile(sgidPath, []byte("test"), 0755))
	require.NoError(t, os.Chmod(sgidPath, 0755|os.ModeSetgid))

	info, err = os.Lstat(sgidPath)
	require.NoError(t, err)
	fi = GetFileInfo(sgidPath, info)

	// Check that the file mode contains the setgid bit
	assert.True(t, info.Mode()&os.ModeSetgid != 0, "File should have setgid bit set")
	// The permissions field should have the setgid bit (2755)
	assert.True(t, fi.Permissions&PERM_SETGID != 0, "Should detect setgid bit in permissions")
	assert.Equal(t, uint16(PERM_SETGID|0o755), fi.Permissions, "Should have setgid bit set")
}

func TestGetAllXattrs(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "xattrtest")
	require.NoError(t, os.WriteFile(path, []byte("test"), 0644))

	// Set multiple xattrs
	require.NoError(t, unix.Setxattr(path, "user.test1", []byte("val1"), 0))
	require.NoError(t, unix.Setxattr(path, "user.test2", []byte("val2"), 0))

	xattrs := getAllXattrs(path)
	assert.NotNil(t, xattrs)
	assert.Equal(t, "val1", xattrs["user.test1"])
	assert.Equal(t, "val2", xattrs["user.test2"])
}

func TestGetAllXattrs_Empty(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "noxattr")
	require.NoError(t, os.WriteFile(path, []byte("test"), 0644))

	xattrs := getAllXattrs(path)
	assert.Nil(t, xattrs)
}

func TestGetFileInfo_SecurityFieldsSeparation(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "sectest")
	require.NoError(t, os.WriteFile(path, []byte("test"), 0644))

	// Set security-related xattrs that should be extracted separately
	_ = unix.Setxattr(path, "security.selinux", []byte("test_label"), 0)
	_ = unix.Setxattr(path, "security.capability", []byte("test_caps"), 0)
	_ = unix.Setxattr(path, "system.posix_acl_access", []byte("test_acl"), 0)
	require.NoError(t, unix.Setxattr(path, "user.normal", []byte("normal_val"), 0))

	info, err := os.Lstat(path)
	require.NoError(t, err)

	fi := GetFileInfo(path, info)
	require.NotNil(t, fi.Metadata)

	// Verify security fields are separated from general xattrs
	if fi.Metadata.SELinux != nil {
		assert.NotContains(t, fi.Metadata.Xattrs, "security.selinux")
	}
	if fi.Metadata.Capabilities != "" {
		assert.NotContains(t, fi.Metadata.Xattrs, "security.capability")
	}
	if len(fi.Metadata.ACLs) > 0 {
		assert.NotContains(t, fi.Metadata.Xattrs, "system.posix_acl_access")
	}

	// Normal xattrs should still be present
	assert.Contains(t, fi.Metadata.Xattrs, "user.normal")
	assert.Equal(t, "normal_val", fi.Metadata.Xattrs["user.normal"])
}
