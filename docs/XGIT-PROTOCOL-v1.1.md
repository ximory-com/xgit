# XGIT 指令集协议 v1.1

## 指令集索引
- [git.* 指令集 (4)](#git-指令集-4)
  - [git.diff](#gitdiff)
  - [git.revert](#gitrevert)
  - [git.tag](#gittag)
  - [git.commit](#gitcommit)

- [file.* 指令集 (9)](#file-指令集-9)
  - [file.write](#filewrite)
  - [file.append](#fileappend)
  - [file.prepend](#fileprepend)
  - [file.delete](#filedelete)
  - [file.move](#filemove)
  - [file.chmod](#filechmod)
  - [file.eol](#fileeol)
  - [file.binary](#filebinary)
  - [file.image](#fileimage)

- [line.* 指令集 (4)](#line-指令集-4)
  - [line.delete](#linedelete)
  - [line.replace](#linereplace)
  - [line.insert](#lineinsert)
  - [line.append](#lineappend)

- [block.* 指令集 (2)](#block-指令集-2)
  - [block.delete](#blockdelete)
  - [block.replace](#blockreplace)

---

## git 指令集 (4)

### git.diff
说明：基于 git diff 生成的标准补丁应用，提供宽松匹配与自动回滚机制。

### git.revert
说明：撤销指定提交或补丁，恢复到之前状态。

### git.tag
说明：为当前 HEAD 打上一个 tag。

### git.commit
说明：提交工作区的变更，必须单独成批，不能与其他指令共存。

---

## file 指令集 (9)

### file.write
说明：写入文件（覆盖或新建），自动创建目录。

### file.append
说明：在文件末尾追加内容。

### file.prepend
说明：在文件开头插入内容。

### file.delete
说明：删除指定文件。

### file.move
说明：移动或重命名文件。

### file.chmod
说明：修改文件权限。

### file.eol
说明：统一换行符（LF/CRLF）。

### file.binary
说明：写入二进制文件。

### file.image
说明：写入图片文件。

---

## line 指令集 (4)

### line.delete
说明：删除命中的行。
参数：
- lineno：精确行号
- keys：关键字匹配（宽松 AND）
- nthl：命中多行时，取第 n 个
- offset：相对锚点偏移

### line.replace
说明：替换命中的行，可替换多行。
参数：同 line.delete

### line.insert
说明：在命中行前插入内容。
参数：同 line.delete

### line.append
说明：在命中行后追加内容。
参数：同 line.delete

---

## block 指令集 (2)

### block.delete
说明：删除一段连续区间。
参数：
- start-keys：起始锚点（必须唯一命中，支持 nthb）
- end-keys：结束锚点（优先唯一，允许多命中时取第一个）

### block.replace
说明：替换一段连续区间。
参数：同 block.delete

---

## 行为规则

1. **lineno**：精确定位到某一行，若存在多处匹配冲突则直接报错。
2. **keys**：宽松 AND 匹配（去除缩进、大小写忽略），确保唯一命中，否则报错。
3. **nthl**：仅在多处命中时有效，指定取第 n 个。
4. **offset**：相对命中行进行偏移，正数向下，负数向上。
5. **block 范围**：start-keys 必须唯一命中；end-keys 优先唯一，若多处则取第一个。
6. **范围内操作**：lineno 若与范围共存，则表示范围内的相对行号。
7. **禁止情况**：
   - offset 不允许与范围模式（start-keys/end-keys）共存
   - git.commit 必须单独成批执行
