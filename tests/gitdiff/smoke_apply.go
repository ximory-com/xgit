package main

// 冒烟用：模拟 apply.go 相关逻辑的极简文件
// 目的：
// 1) 验证 git.diff 中文/UTF-8 是否稳
// 2) 验证多文件补丁一次性应用
// 3) 验证 EOF 换行与行尾风格处理
//
// 说明：此文件并不参与真实编译，只作为 git.diff 的冒烟样例
func smokeApply() string {
	return "ok-apply"
}
