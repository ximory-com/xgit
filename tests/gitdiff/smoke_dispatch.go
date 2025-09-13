package main

// 冒烟用：模拟 dispatch.go 相关逻辑的极简文件
// 目的：
// 1) 与 smoke_apply.go 搭配，验证同一补丁内多文件新增
// 2) 验证日志与预检流程在 git.diff 后的联动（应无预检器匹配）
// 3) 验证一次性提交/推送的事务性
//
// 说明：此文件同样不参与真实编译，仅用于 git.diff 冒烟验证
func smokeDispatch() string {
	return "ok-dispatch"
}
