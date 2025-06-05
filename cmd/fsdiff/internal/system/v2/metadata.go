package v2

type FileInfo struct {
	Hash        uint64        `json:"h"`           // optional, not set here
	Permissions uint16        `json:"p"`           // rwx + special bits
	OwnerID     uint32        `json:"u"`           // UID
	GroupID     uint32        `json:"g"`           // GID
	Metadata    *FileMetadata `json:"m,omitempty"` // xattrs, selinux
}

type FileMetadata struct {
	SELinux map[string]string `json:"s,omitempty"`
	Xattrs  map[string]string `json:"x,omitempty"`
}
