package fileops

// 仅声明所需能力；主包里的 DualLogger 已实现 Log(...)，能自动满足此接口
type DualLogger interface {
	Log(format string, a ...any)
}