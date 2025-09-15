# file.* 指令弃用说明（2025-09）

> 结论：所有**写入类** file.* 指令（`file.write/append/prepend/replace/delete/move/chmod` 等）
> 统一由 **git.diff** 取代；file.* 写类不再作为执行路径推进。

## 为什么
- git.diff 具备预检（--check/--recount）、结构化 R/D 处理、事务提交与回滚、日志可追踪。
- 基于上下文的最小修改，远比“按行号/直接落盘”稳健。

## 迁移映射
- `file.write` → 以 **A-only** 新建文件（需要时）
- `file.append/prepend/replace/eol` → 统一生成 **M-only** 的最小文本修改（由补丁生成层完成）
- `file.delete` → **D-only**
- `file.move` → **R-only**（禁止 R+M，改名与内容修改须分两步）
- `file.chmod` → **Mode-only**
- `file.image/binary` → 暂不开放（如需二进制，请使用 git 的 GIT binary patch，后续再评估）

## 已明确不再推进的指令族
- `line.xxx`（按行号）：由 git.diff 的上下文定位替代
- `block.xxx`（锚点块）：作为“补丁生成层”的能力，而非执行指令

## 推荐阅读
- 《gitdiff_spec_and_tests.md》：五大基元操作（A/M/R/D/Mode）与标准测试补丁
- 《gitdiff_collab.md》：人机协作最小修改流（M-only 为主）
