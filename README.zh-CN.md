# Rayut

<p align="center">
  <img src="app/assets/logo.svg" alt="Rayut" width="96">
</p>

Rayut 是 Ubuntu Touch 上的全局代理。界面启动一个以 root 运行的助手，助手运行 mihomo。关闭代理时，只删除本程序自己的策略规则。

本程序是自由软件，许可证是 GPL-3.0-or-later。全文见 `LICENSE`。

## 截图

<p align="center">
  <img src="https://i.imgur.com/DgPDeUN.png" alt="首页" width="240">
  <img src="https://i.imgur.com/pu1P84D.png" alt="节点编辑" width="240">
</p>

## 二进制

随包装上的每一份二进制都有版本、许可证和来源。应用里的关于页打开的是同一份清单。

- `rayut` 0.1.29，GPL-3.0-or-later，https://github.com/alpha-liu-01/rayut
- `rayutd` 0.1.29，GPL-3.0-or-later，同一仓库的 `daemon/` 目录
- `mihomo` v1.19.32，GPL-3.0，https://github.com/MetaCubeX/mihomo/tree/v1.19.32

mihomo 用的是该 tag 的官方 linux-arm64 压缩包：

https://github.com/MetaCubeX/mihomo/releases/download/v1.19.32/mihomo-linux-arm64-v1.19.32.gz

压缩包 SHA-256：`9dd862e28b46ff7d775f169cceebc28deccaa0a9e804237d421cd2571e0caba0`

许可证文件 SHA-256：`3972dc9744f6499f0f9b2dbf76696f2ae7ad8af9b23dde66d6af86c9dfb36986`

## 其它组件

- `gopkg.in/yaml.v3` v3.0.1，MIT 与 Apache-2.0，https://github.com/go-yaml/yaml
- quirc 1.0，ISC，版权 (C) 2010-2012 Daniel Beer，https://github.com/dlbeer/quirc
- QR Code generator library (C)，MIT，版权 (c) Project Nayuki，https://github.com/nayuki/QR-Code-generator
- 框架 ubuntu-sdk-20.04 的 Clickable CMake 模板。Clickable 是 GPL-3.0，https://gitlab.com/clickable/clickable。它是构建工具，不装到手机上。

首页排布参照 v2rayNG，版权 2dust 及贡献者，GPL-3.0，https://github.com/2dust/v2rayNG。本仓库不包含 v2rayNG 的源码文件。

这些说明的正文在 `app/notices/`。关于页可以打开它们。

## 权限

Click 使用 `app/rayut.apparmor` 里的 unconfined 模板。受限制的 Click 不能创建 TUN 设备，也不能改策略路由，所以助手以 root 运行。

界面只启动这一条命令：

```
/usr/bin/sudo -S -p '' -- <application directory>/bin/rayutd --session
```

密码输入认证框，由系统认证服务核对，写进 `sudo -S` 的标准输入，然后被覆盖。密码不保存，也不写入日志。安装包不安装 sudoers 文件。

助手自己执行的命令，以及规则清理，写在 `app/notices/privileges.zh.txt`。恢复只删除优先级 9000、9001、9002、9010 上属于本程序的策略规则，然后清空路由表 2022。不清空主路由表，不执行 `iptables -F`，也不清空 nftables。

## 构建

在 `app/` 下，并且 `packaging/core/rayutd` 和 `packaging/core/mihomo` 已经放好时：

```
clickable build --arch arm64
```

助手从 `daemon/` 构建，目标是 `linux/arm64`。界面语言见 `app/translations/README.md`。

English: `README.md`.
