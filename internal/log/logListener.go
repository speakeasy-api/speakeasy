package log

type Msg struct {
	Msg  string
	Type MsgType
}

type MsgType string

var (
	MsgInfo   MsgType = "info"
	MsgWarn   MsgType = "warn"
	MsgError  MsgType = "error"
	MsgGithub MsgType = "github"
)
