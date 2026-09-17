package openstackdb

// Deleted controls inclusion of soft-deleted records. The zero value excludes them.
type Deleted uint8

const (
	ExcludeDeleted Deleted = iota
	IncludeDeleted
	OnlyDeleted
)
