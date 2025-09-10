package main

// XGIT:BEGIN IMPORTS
// 说明：本文件提供对现有私有符号的“导出包装”，统一对外 API，避免大面积改动原文件。
// 后续如需新增依赖或切换实现，只需在本锚点区块追加 import 即可。
//
// 注意：这里不主动引入额外第三方库，保持可编译最小集。
//
// （如需扩展，请在此锚点区追加 import 行）
/*
   示例：
   import "strings"
*/
// XGIT:END IMPORTS

// XGIT:BEGIN DOC
// 导出别名与适配：
// 1) 类型导出：让其他文件可直接使用 Patch / FileChunk / BlockChunk / DualLogger
// 2) 方法导出：为 dualLogger 增加 Log() 以兼容 logger.Log(...)
// 3) 函数导出：把内部私有函数（parsePatch/normPath/writeFile/applyBlock）包装成导出函数
//    这样 main.go / apply.go / parser.go 等跨文件调用时无需大改。
//
// 该文件不改变原有逻辑，只是“桥接层”。后续逐步为各源码文件增加细粒度锚点时，
// 可以再收敛或替换这些包装。
// XGIT:END DOC

// 类型别名（导出）
type Patch = patch
type FileChunk = fileChunk
type BlockChunk = blockChunk
type DualLogger = dualLogger

// 给 dualLogger 增加对外统一日志方法（原有是 d.log(...)）
func (d *DualLogger) Log(format string, a ...any) { 
	if d == nil {
		return
	}
	d.log(format, a...)
}

// 导出包装：parsePatch -> ParsePatch
func ParsePatch(patchFile, eof string) (*Patch, error) {
	return parsePatch(patchFile, eof)
}

// 导出包装：normPath -> NormPath
func NormPath(p string) string {
	return normPath(p)
}

// 导出包装：writeFile -> WriteFile
func WriteFile(repo string, rel string, content string, logf func(string, ...any)) error {
	return writeFile(repo, rel, content, logf)
}

// 导出包装：applyBlock -> ApplyBlock
func ApplyBlock(repo string, blk BlockChunk, logf func(string, ...any)) error {
	return applyBlock(repo, blk, logf)
}
