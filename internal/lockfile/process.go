package lockfile

type ProcessIdentity struct {
	PID       uint32
	StartTime uint64
	Image     string
}
type ProcessInspector interface {
	Inspect(pid uint32) (ProcessIdentity, error)
}

func exactOwner(value record, identity ProcessIdentity) bool {
	return value.OwnerPID == identity.PID && value.OwnerStartTime == identity.StartTime && value.OwnerImage == identity.Image
}
