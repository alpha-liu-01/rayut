界面原文是英文。中文在 `rayut_zh_CN.ts`。再加一种语言，只要在这个目录放下对应文件，不用改代码。

Source strings in the QML and C++ are English. Each extra language is a file in this directory.

1. Copy `rayut.ts` to `rayut_<locale>.ts`. The locale is a Qt name such as `de`, `fr`, `ja`, `zh_TW`, or `zh_HK`.
2. Fill the `<translation>` entries and remove `type="unfinished"`. Qt Linguist does this, and so does a text editor.
3. Compile it: `lrelease rayut_<locale>.ts -qm rayut_<locale>.qm`
4. Keep the `.qm` next to the `.ts`.

The build compiles every `rayut_*.ts` when `lrelease` is installed. When it is not, the build ships the `rayut_*.qm` files already in this directory. `rayut.ts` is only the empty template and is not shipped.

At startup the app loads `rayut_<system locale>.qm` from beside the binary. A Chinese locale with no file of its own uses `rayut_zh_CN.qm`. Any other locale with no file stays in English. A locale file that exists is used on its own: strings you leave unfinished stay in English.

Leave these as they are: `ms`, `TLS`, `UDP`, protocol names (`Shadowsocks`, `VMess`, `VLESS`, `Trojan`, `Hysteria2`, `TUIC`), the name Rayut, and the stored group name `本地`.
