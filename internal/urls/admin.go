package urls

const (
	SocketPath            = "/run/eserved.sock"
	SocketURL             = "http://unix"
	CreateTokenSuburl     = "/admin/v1/token/create"
	TokensListSuburl      = "/admin/v1/token/list"
	RevokeMachineSuburl   = "/admin/v1/machine/revoke"
	MachinesListSuburl    = "/admin/v1/machine/list"
	JobsListSuburl        = "/admin/v1/jobs/list"
	AdminJobsCancelSuburl = "/admin/v1/jobs/cancel"
	BuildStartSuburl      = "/admin/v1/build/start"
	BinaryUploadSuburl    = "/admin/v1/binary/upload"
	BinaryListSuburl      = "/admin/v1/binary/list"
	FlavorApplySuburl     = "/admin/v1/flavor/apply"
	AdminJobsStreamSuburl = "/admin/v1/jobs/stream"
)
