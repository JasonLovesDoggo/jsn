package v2

type FileInfo struct {
	Metadata    *FileMetadata `json:"m,omitempty"` // xattrs, selinux
	Hash        uint64        `json:"h"`           // optional, not set here
	OwnerID     uint32        `json:"u"`           // UID
	GroupID     uint32        `json:"g"`           // GID
	Permissions uint16        `json:"p"`           // rwx + special bits
}

type FileMetadata struct {
	SELinux      map[string]string `json:"s,omitempty"`
	Xattrs       map[string]string `json:"x,omitempty"`
	Capabilities string            `json:"c,omitempty"`  // file capabilities
	ACLs         []string          `json:"a,omitempty"`  // POSIX ACLs
	Immutable    bool              `json:"im,omitempty"` // immutable flag
	AppendOnly   bool              `json:"ao,omitempty"` // append-only flag
}
