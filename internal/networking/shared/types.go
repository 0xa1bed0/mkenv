package shared

type Proto string

const (
	ProtoTCP Proto = "tcp"
	ProtoUDP Proto = "udp"
)

type Listener struct {
	Port  int   `json:"port"`
	Proto Proto `json:"proto"` // "tcp" or "udp"
	PID   int
	UID   int
	Cmd   string
}

type Snapshot struct {
	Listeners map[int]Listener `json:"listeners"` // port number => Listener
}

type OnSnapshotResponse struct {
	Response map[int]string `json:"ports_allocation_status"`
}

type OnInstallResponse struct {
	Logs string `json:"logs"`
}

type Expose struct {
	Listener Listener `json:"listener"`
}

type BlockedPorts struct {
	Ports []int `json:"ports"`
}

type Install struct {
	PkgName string `json:"PkgName"`
}
