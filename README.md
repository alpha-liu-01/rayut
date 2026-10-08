# Rayut

<p align="center">
  <img src="app/assets/logo.svg" alt="Rayut" width="96">
</p>

Rayut is a global proxy for Ubuntu Touch. The interface starts a root helper, the helper runs mihomo, and turning the proxy off removes only this program's policy rules.

The program is free software under GPL-3.0-or-later. See `LICENSE`.

## Screenshots

<p align="center">
  <img src="https://i.imgur.com/DgPDeUN.png" alt="Home" width="240">
  <img src="https://i.imgur.com/pu1P84D.png" alt="Node editor" width="240">
</p>

## Binaries

Each shipped binary has a version, a license, and a source. The about page in the app opens the same list.

- `rayut` 0.1.29, GPL-3.0-or-later, https://github.com/alpha-liu-01/rayut
- `rayutd` 0.1.29, GPL-3.0-or-later, the `daemon/` directory of the same repository
- `mihomo` v1.19.32, GPL-3.0, https://github.com/MetaCubeX/mihomo/tree/v1.19.32

The mihomo build is the official linux-arm64 archive for that tag:

https://github.com/MetaCubeX/mihomo/releases/download/v1.19.32/mihomo-linux-arm64-v1.19.32.gz

Archive SHA-256: `9dd862e28b46ff7d775f169cceebc28deccaa0a9e804237d421cd2571e0caba0`

License file SHA-256: `3972dc9744f6499f0f9b2dbf76696f2ae7ad8af9b23dde66d6af86c9dfb36986`

## Other components

- `gopkg.in/yaml.v3` v3.0.1, MIT and Apache-2.0, https://github.com/go-yaml/yaml
- quirc 1.0, ISC, Copyright (C) 2010-2012 Daniel Beer, https://github.com/dlbeer/quirc
- QR Code generator library (C), MIT, Copyright (c) Project Nayuki, https://github.com/nayuki/QR-Code-generator
- Clickable CMake template for framework ubuntu-sdk-20.04. Clickable is GPL-3.0, https://gitlab.com/clickable/clickable. It is the build tool and is not installed on the phone.

The home screen arrangement follows v2rayNG, Copyright 2dust and contributors, GPL-3.0, https://github.com/2dust/v2rayNG. This repository does not include v2rayNG source files.

Texts for these notices are in `app/notices/`. The about page can open them.

## Privileges

The click uses the unconfined AppArmor template in `app/rayut.apparmor`. A confined click cannot create the TUN device or change policy routing, so the helper runs as root.

The interface starts only this command:

```
/usr/bin/sudo -S -p '' -- <application directory>/bin/rayutd --session
```

The password is typed into the authentication dialog, checked by the system authentication service, written to the standard input of `sudo -S`, and then overwritten. It is not stored and it is not written to the log. The package does not install a sudoers file.

The helper's own commands, and the rule cleanup, are listed in `app/notices/privileges.en.txt`. Recovery deletes policy rules only at priorities 9000, 9001, 9002, and 9010 when the rule text belongs to this program, then flushes routing table 2022. It does not flush the main table, it does not run `iptables -F`, and it does not flush nftables.

## Build

From `app/`, with `packaging/core/rayutd` and `packaging/core/mihomo` already in place:

```
clickable build --arch arm64
```

The helper is built from `daemon/` for `linux/arm64`. Interface languages are described in `app/translations/README.md`.

中文说明见 `README.zh-CN.md`。
