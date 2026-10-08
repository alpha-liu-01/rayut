import QtQuick 2.7
import Lomiri.Components 1.3
import Lomiri.Components.Popups 1.3
import Lomiri.Components.Extras.PamAuthentication 0.1
import Lomiri.Content 1.3
import QtQuick.Layouts 1.3
import Rayut 1.0

MainView {
    id: root
    objectName: "mainView"
    applicationName: "rayut.rayut"
    anchorToKeyboard: true

    width: units.gu(45)
    height: units.gu(75)

    PamAuthentication {
        id: pam
        serviceName: "sudo"
    }

    property var authDialogItem: null
    property var scanTransfer: null
    property bool scanHold: false
    property string scanHint: ""

    function acceptScan(transfer) {
        if (!transfer)
            return
        root.scanTransfer = transfer
        if (transfer.state === ContentTransfer.Aborted) {
            scanAction.hasImage = false
            scanAction.shouldPop = stack.depth > 1
            scanAction.start()
            return
        }
        if (transfer.state !== ContentTransfer.Charged)
            return
        root.scanHold = true
        scanAction.shouldPop = stack.depth > 1
        var count = transfer.items ? transfer.items.length : 0
        if (count < 1) {
            scanAction.hasImage = false
            root.scanHint = "无法识别的图片"
            scanAction.start()
            return
        }
        scanAction.imageUrl = transfer.items[0].url
        scanAction.hasImage = true
        scanAction.start()
    }

    Connections {
        target: Qt.application
        onStateChanged: {
            if (Qt.application.state === Qt.ApplicationSuspended && !root.scanHold && !root.scanTransfer)
                Qt.quit()
        }
    }

    Connections {
        target: ContentHub
        onImportRequested: root.acceptScan(transfer)
    }

    Timer {
        id: authAction
        interval: 1
        property string action: ""
        onTriggered: {
            var item = root.authDialogItem
            if (!item)
                return
            if (action === "cancel") {
                item.clearSecret()
                root.authDialogItem = null
                PopupUtils.close(item)
                return
            }
            var entered = item.takePassword()
            if (!pam.validatePasswordToken(entered)) {
                item.showFailure("Authentication failed")
                return
            }
            root.authDialogItem = null
            PopupUtils.close(item)
            Controller.startHelper(entered)
        }
    }

    Component {
        id: authDialog

        Dialog {
            id: dialog
            title: "Authentication required"
            text: "Enter passcode or passphrase:"

            function takePassword() {
                var entered = passwordField.text
                passwordField.text = ""
                return entered
            }

            function clearSecret() {
                passwordField.text = ""
            }

            function showFailure(message) {
                failure.text = message
            }

            function schedule(next) {
                root.authDialogItem = dialog
                authAction.action = next
                authAction.start()
            }

            TextField {
                id: passwordField
                placeholderText: "passcode or passphrase"
                echoMode: TextInput.Password
                inputMethodHints: Qt.ImhNoPredictiveText | Qt.ImhSensitiveData
                onAccepted: dialog.schedule("submit")
            }

            Label {
                id: failure
                visible: text !== ""
                wrapMode: Text.Wrap
                color: theme.palette.normal.negative
            }

            Button {
                text: "Authenticate"
                color: theme.palette.normal.positive
                onClicked: dialog.schedule("submit")
            }

            Button {
                text: "Cancel"
                onClicked: dialog.schedule("cancel")
            }

            Component.onCompleted: passwordField.forceActiveFocus()
        }
    }

    ContentPeerModel {
        id: picturePeers
        contentType: ContentType.Pictures
        handler: ContentHandler.Source
        property string want: ""
        onFindPeersCompleted: {
            if (want === "")
                return
            var fragment = want
            want = ""
            for (var i = 0; i < peers.length; i++) {
                if (peers[i].appId.indexOf(fragment) !== -1) {
                    root.scanHold = true
                    peers[i].selectionType = ContentTransfer.Single
                    root.scanTransfer = peers[i].request()
                    return
                }
            }
            root.scanHold = false
            root.scanHint = "没有相机"
        }
    }

    Timer {
        id: scanAction
        interval: 1
        property url imageUrl
        property bool hasImage: false
        property bool shouldPop: false
        onTriggered: {
            var transfer = root.scanTransfer
            var url = imageUrl
            var useImage = hasImage
            hasImage = false
            root.scanHold = false
            if (useImage)
                Controller.importFromImage(url)
            if (transfer)
                transfer.finalize()
            root.scanTransfer = null
            if (shouldPop && stack.depth > 1)
                stack.pop()
        }
    }

    Connections {
        target: root.scanTransfer
        ignoreUnknownSignals: true
        onStateChanged: root.acceptScan(root.scanTransfer)
    }

    PageStack {
        id: stack
        anchors.fill: parent

        Component.onCompleted: stack.push(homePage)
    }

    Component {
        id: homePage

        Page {
            id: homeRoot

            function byteText(value) {
                var n = Number(value)
                if (n < 0) {
                    n = 0
                }
                if (n < 1024) {
                    return n + " B"
                }
                if (n < 1048576) {
                    return (n / 1024).toFixed(1) + " KB"
                }
                if (n < 1073741824) {
                    return (n / 1048576).toFixed(1) + " MB"
                }
                return (n / 1073741824).toFixed(2) + " GB"
            }

            function trafficMax() {
                var max = 1
                var rows = Controller.trafficSamples
                for (var i = 0; i < rows.length; i++) {
                    var n = Number(rows[i].up) + Number(rows[i].down)
                    if (n > max) {
                        max = n
                    }
                }
                return max
            }

            onVisibleChanged: {
                if (visible) {
                    Controller.refreshTraffic()
                }
            }

            header: PageHeader {
                id: header
                title: "Rayut"

                trailingActionBar.actions: [
                    Action {
                        iconName: "note"
                        text: "节点"
                        onTriggered: stack.push(nodePage)
                    },
                    Action {
                        iconName: "note"
                        text: "订阅"
                        onTriggered: stack.push(profilePage)
                    },
                    Action {
                        iconName: "info"
                        text: "日志"
                        onTriggered: stack.push(sessionPage)
                    }
                ]
            }

        ColumnLayout {
            spacing: units.gu(2)
            anchors {
                margins: units.gu(2)
                top: header.bottom
                left: parent.left
                right: parent.right
                bottom: parent.bottom
            }

            Label {
                Layout.fillWidth: true
                wrapMode: Text.Wrap
                text: Controller.summary
            }

            Label {
                Layout.fillWidth: true
                wrapMode: Text.Wrap
                visible: Controller.profileText !== ""
                text: Controller.profileText
            }

            Button {
                Layout.fillWidth: true
                text: "总开关"
                onClicked: {
                    if (!Controller.helperRunning) {
                        PopupUtils.open(authDialog, root)
                    } else {
                        Controller.toggleProxy()
                    }
                }
            }

            Label {
                Layout.fillWidth: true
                wrapMode: Text.Wrap
                text: "本次会话  上传 " + homeRoot.byteText(Controller.sessionUpload) + "  下载 " + homeRoot.byteText(Controller.sessionDownload)
            }

            Label {
                Layout.fillWidth: true
                wrapMode: Text.Wrap
                text: "累计  上传 " + homeRoot.byteText(Controller.totalUpload) + "  下载 " + homeRoot.byteText(Controller.totalDownload)
            }

            Label {
                Layout.fillWidth: true
                wrapMode: Text.Wrap
                color: theme.palette.normal.backgroundText
                text: "速率  上传 " + homeRoot.byteText(Controller.uploadRate) + "/s  下载 " + homeRoot.byteText(Controller.downloadRate) + "/s"
            }

            Item {
                id: trafficChart
                Layout.fillWidth: true
                Layout.preferredHeight: units.gu(6)
                visible: Controller.trafficSamples.length > 0

                Row {
                    anchors.fill: parent
                    spacing: 1

                    Repeater {
                        model: Controller.trafficSamples

                        Item {
                            width: Math.max(1, (trafficChart.width - Math.max(0, Controller.trafficSamples.length - 1)) / Math.max(1, Controller.trafficSamples.length))
                            height: trafficChart.height

                            Rectangle {
                                width: parent.width
                                height: parent.height * Number(modelData.down) / homeRoot.trafficMax()
                                anchors.bottom: parent.bottom
                                color: theme.palette.normal.positive
                            }

                            Rectangle {
                                width: parent.width
                                height: parent.height * Number(modelData.up) / homeRoot.trafficMax()
                                anchors.bottom: parent.bottom
                                anchors.bottomMargin: parent.height * Number(modelData.down) / homeRoot.trafficMax()
                                color: theme.palette.normal.activity
                            }
                        }
                    }
                }
            }

            Label {
                Layout.fillWidth: true
                visible: Controller.trafficSamples.length > 0
                color: theme.palette.normal.backgroundText
                text: "绿为下载，活动色为上传"
            }

            Timer {
                interval: 1000
                running: stack.currentPage === homeRoot
                repeat: true
                onTriggered: Controller.refreshTraffic()
            }

            Label {
                Layout.fillWidth: true
                wrapMode: Text.Wrap
                color: theme.palette.normal.backgroundText
                text: Controller.versionText
            }

            Label {
                Layout.fillWidth: true
                wrapMode: Text.Wrap
                color: theme.palette.normal.backgroundText
                text: Controller.message
            }

            Item {
                Layout.fillHeight: true
            }
        }
        }
    }

    Component {
        id: nodePage

        Page {
            id: nodePageRoot
            property string selectedGroup: ""
            property var currentGroup: {
                var groups = Controller.proxyGroups
                if (!groups || groups.length === 0)
                    return null
                for (var i = 0; i < groups.length; i++) {
                    if (groups[i].name === selectedGroup)
                        return groups[i]
                }
                return groups[0]
            }

            header: PageHeader {
                id: nodeHeader
                title: "节点"
            }

            Timer {
                id: nodeAction
                interval: 1
                property string kind: ""
                property string groupName: ""
                property string nodeName: ""
                onTriggered: {
                    if (kind === "delay")
                        Controller.testDelay(nodeName)
                    else if (kind === "select")
                        Controller.selectProxy(groupName, nodeName)
                }
            }

            Flickable {
                id: nodeFlick
                anchors {
                    top: nodeHeader.bottom
                    left: parent.left
                    right: parent.right
                    bottom: parent.bottom
                }
                contentHeight: nodeColumn.height + units.gu(4)
                clip: true

                Column {
                    id: nodeColumn
                    width: nodeFlick.width - units.gu(4)
                    x: units.gu(2)
                    y: units.gu(2)
                    spacing: units.gu(1)

                    Label {
                        width: parent.width
                        wrapMode: Text.Wrap
                        text: Controller.proxyGroups.length === 0 ? "没有可显示的组。请先打开代理。" : ""
                        visible: text !== ""
                    }

                    Repeater {
                        model: Controller.proxyGroups
                        delegate: Button {
                            width: nodeColumn.width
                            text: modelData.name
                            color: nodePageRoot.currentGroup && modelData.name === nodePageRoot.currentGroup.name ? theme.palette.normal.positive : theme.palette.normal.base
                            onClicked: nodePageRoot.selectedGroup = modelData.name
                        }
                    }

                    Label {
                        width: parent.width
                        wrapMode: Text.Wrap
                        visible: nodePageRoot.currentGroup !== null
                        text: nodePageRoot.currentGroup ? ("当前：" + nodePageRoot.currentGroup.now) : ""
                    }

                    Repeater {
                        model: nodePageRoot.currentGroup ? nodePageRoot.currentGroup.nodes : []
                        delegate: Row {
                            width: nodeColumn.width
                            spacing: units.gu(1)

                            Label {
                                width: parent.width * 0.42
                                wrapMode: Text.Wrap
                                text: (modelData.selected ? "当前 " : "") + modelData.name
                            }

                            Label {
                                width: units.gu(8)
                                text: modelData.delayText
                            }

                            Button {
                                visible: nodePageRoot.currentGroup && nodePageRoot.currentGroup.selectable && !modelData.selected
                                text: "选择"
                                onClicked: {
                                    nodePageRoot.selectedGroup = nodePageRoot.currentGroup.name
                                    nodeAction.kind = "select"
                                    nodeAction.groupName = nodePageRoot.currentGroup.name
                                    nodeAction.nodeName = modelData.name
                                    nodeAction.start()
                                }
                            }

                            Button {
                                text: "延迟"
                                onClicked: {
                                    nodeAction.kind = "delay"
                                    nodeAction.nodeName = modelData.name
                                    nodeAction.start()
                                }
                            }
                        }
                    }

                    Button {
                        width: parent.width
                        text: "刷新"
                        onClicked: Controller.refreshGroups()
                    }

                    Label {
                        width: parent.width
                        wrapMode: Text.Wrap
                        color: theme.palette.normal.backgroundText
                        text: Controller.message
                    }
                }
            }

            Component.onCompleted: Controller.refreshGroups()
        }
    }

    Component {
        id: sessionPage

        Page {
            header: PageHeader {
                id: sessionHeader
                title: "日志"
            }

            Timer {
                id: sessionLoad
                interval: 1
                onTriggered: Controller.refreshSession()
            }

            Flickable {
                id: sessionFlick
                anchors {
                    top: sessionHeader.bottom
                    left: parent.left
                    right: parent.right
                    bottom: parent.bottom
                }
                contentHeight: sessionColumn.height + units.gu(4)
                clip: true

                Column {
                    id: sessionColumn
                    width: sessionFlick.width - units.gu(4)
                    x: units.gu(2)
                    y: units.gu(2)
                    spacing: units.gu(1)

                    Label {
                        width: parent.width
                        wrapMode: Text.Wrap
                        text: Controller.helperRunning ? "" : "请先在首页连接"
                        visible: text !== ""
                    }

                    Label {
                        width: parent.width
                        text: "连接"
                        font.bold: true
                    }

                    Label {
                        width: parent.width
                        wrapMode: Text.Wrap
                        visible: Controller.helperRunning && Controller.sessionConnections.length === 0
                        text: "这次会话还没有连接"
                    }

                    Repeater {
                        model: Controller.sessionConnections
                        delegate: Label {
                            width: sessionColumn.width
                            wrapMode: Text.Wrap
                            text: modelData.destination + "\n" + modelData.rule + " · " + modelData.chain + "\n上传 " + modelData.upload + " · 下载 " + modelData.download
                        }
                    }

                    Label {
                        width: parent.width
                        text: "日志"
                        font.bold: true
                    }

                    Label {
                        width: parent.width
                        wrapMode: Text.Wrap
                        visible: Controller.helperRunning && Controller.sessionLogs.length === 0
                        text: "这次会话还没有日志"
                    }

                    Repeater {
                        model: Controller.sessionLogs
                        delegate: Label {
                            width: sessionColumn.width
                            wrapMode: Text.Wrap
                            text: modelData.type + "  " + modelData.payload
                        }
                    }

                    Button {
                        width: parent.width
                        text: "刷新"
                        enabled: Controller.helperRunning
                        onClicked: sessionLoad.start()
                    }

                    Label {
                        width: parent.width
                        wrapMode: Text.Wrap
                        color: theme.palette.normal.backgroundText
                        text: Controller.message
                    }
                }
            }

            Component.onCompleted: sessionLoad.start()
        }
    }

    Component {
        id: profilePage

        Page {
            header: PageHeader {
                id: profileHeader
                title: "订阅"
            }

            Timer {
                id: templateAction
                interval: 1
                property string templateId: ""
                onTriggered: Controller.applyRuleTemplate(templateId)
            }

            Flickable {
                id: flick
                anchors {
                    top: profileHeader.bottom
                    left: parent.left
                    right: parent.right
                    bottom: parent.bottom
                }
                contentHeight: profileColumn.implicitHeight + units.gu(4)
                clip: true

                ColumnLayout {
                    id: profileColumn
                    width: parent.width - units.gu(4)
                    x: units.gu(2)
                    y: units.gu(2)
                    spacing: units.gu(2)

                    Label {
                        Layout.fillWidth: true
                        wrapMode: Text.Wrap
                        text: Controller.helperRunning ? Controller.profileText : "请先在首页连接"
                    }

                    Button {
                        Layout.fillWidth: true
                        text: "全局代理"
                        enabled: Controller.helperRunning
                        color: Controller.ruleTemplate === "global" ? theme.palette.normal.positive : theme.palette.normal.base
                        onClicked: {
                            templateAction.templateId = "global"
                            templateAction.start()
                        }
                    }

                    Button {
                        Layout.fillWidth: true
                        text: "绕过局域网"
                        enabled: Controller.helperRunning
                        color: Controller.ruleTemplate === "lan" ? theme.palette.normal.positive : theme.palette.normal.base
                        onClicked: {
                            templateAction.templateId = "lan"
                            templateAction.start()
                        }
                    }

                    Button {
                        Layout.fillWidth: true
                        text: "绕过局域网和中国大陆"
                        enabled: Controller.helperRunning
                        color: Controller.ruleTemplate === "lan-china" ? theme.palette.normal.positive : theme.palette.normal.base
                        onClicked: {
                            templateAction.templateId = "lan-china"
                            templateAction.start()
                        }
                    }

                    TextField {
                        id: urlField
                        Layout.fillWidth: true
                        placeholderText: "https 订阅链接"
                        enabled: Controller.helperRunning
                        inputMethodHints: Qt.ImhNoPredictiveText | Qt.ImhSensitiveData
                    }

                    Button {
                        Layout.fillWidth: true
                        text: "导入链接"
                        enabled: Controller.helperRunning
                        onClicked: Controller.importURL(urlField.text)
                    }

                    TextArea {
                        id: yamlField
                        Layout.fillWidth: true
                        Layout.preferredHeight: units.gu(18)
                        placeholderText: "本地 YAML 或一条分享链接"
                        enabled: Controller.helperRunning
                    }

                    Button {
                        Layout.fillWidth: true
                        text: "导入 YAML"
                        enabled: Controller.helperRunning
                        onClicked: Controller.importContent(yamlField.text)
                    }

                    Button {
                        Layout.fillWidth: true
                        text: "相册"
                        enabled: Controller.helperRunning
                        onClicked: {
                            root.scanHint = ""
                            stack.push(picturePickerPage)
                        }
                    }

                    Button {
                        Layout.fillWidth: true
                        text: "相机"
                        enabled: Controller.helperRunning
                        onClicked: {
                            root.scanHint = ""
                            root.scanHold = true
                            picturePeers.want = "camera.ubports"
                            picturePeers.findPeers()
                        }
                    }

                    Label {
                        Layout.fillWidth: true
                        wrapMode: Text.Wrap
                        visible: root.scanHint !== ""
                        text: root.scanHint
                    }

                    Button {
                        Layout.fillWidth: true
                        text: "编辑当前配置"
                        enabled: Controller.helperRunning && !Controller.versionMismatch
                        onClicked: stack.push(editorPage)
                    }

                    Button {
                        Layout.fillWidth: true
                        text: "刷新"
                        enabled: Controller.helperRunning
                        onClicked: Controller.refreshProfile()
                    }

                    Button {
                        Layout.fillWidth: true
                        text: "激活"
                        enabled: Controller.helperRunning && !Controller.tunRunning
                        onClicked: Controller.activateProfile()
                    }

                    Label {
                        Layout.fillWidth: true
                        wrapMode: Text.Wrap
                        color: theme.palette.normal.backgroundText
                        text: Controller.message
                    }
                }
            }
        }
    }

    Component {
        id: editorPage

        Page {
            id: editorRoot
            property string mode: "text"
            property int formIndex: -1

            header: PageHeader {
                id: editHeader
                title: editorRoot.mode === "text" ? "文本" : "节点"
            }

            Timer {
                id: loadEdit
                interval: 1
                onTriggered: {
                    Controller.loadProfileDocument()
                    editorRoot.mode = "text"
                }
            }

            Timer {
                id: saveEdit
                interval: 1
                onTriggered: Controller.saveProfileText()
            }

            Timer {
                id: activateEdit
                interval: 1
                onTriggered: Controller.activateProfile()
            }

            Timer {
                id: openNodes
                interval: 1
                onTriggered: {
                    if (Controller.previewProfile())
                        editorRoot.mode = "nodes"
                }
            }

            Timer {
                id: writeNode
                interval: 1
                onTriggered: {
                    var port = parseInt(portField.text, 10)
                    if (isNaN(port))
                        port = 0
                    if (Controller.applyProxyEdit(editorRoot.formIndex, nameField.text, typeField.text, serverField.text, port, networkField.text, tlsSwitch.checked, udpSwitch.checked, secretField.text)) {
                        secretField.text = ""
                        editorRoot.mode = "text"
                    }
                }
            }

            Item {
                id: editBody
                anchors {
                    top: editHeader.bottom
                    left: parent.left
                    right: parent.right
                    bottom: parent.bottom
                }

                Item {
                    id: textPage
                    visible: editorRoot.mode === "text"
                    anchors.fill: parent

                    ListView {
                        id: lineView
                        anchors {
                            top: parent.top
                            left: parent.left
                            right: parent.right
                            bottom: textButtons.top
                            leftMargin: units.gu(2)
                            rightMargin: units.gu(2)
                            topMargin: units.gu(1)
                        }
                        clip: true
                        boundsBehavior: Flickable.StopAtBounds
                        cacheBuffer: units.gu(8)
                        model: Controller.editLines

                        // Only the visible rows exist. The whole subscription is about
                        // 14k lines, and one text control was repainting all of them.
                        delegate: TextInput {
                            width: lineView.width
                            height: units.gu(2.5)
                            clip: true
                            color: theme.palette.normal.backgroundText
                            font.family: "Ubuntu Mono"
                            font.pixelSize: FontUtils.sizeToPixels("small")
                            inputMethodHints: Qt.ImhNoPredictiveText | Qt.ImhNoAutoUppercase | Qt.ImhSensitiveData
                            property string source: model.line

                            function claimFocus() {
                                if (Controller.editFocusRow !== index)
                                    return
                                forceActiveFocus()
                                cursorPosition = Math.min(Controller.editFocusColumn, text.length)
                            }

                            onSourceChanged: {
                                if (text !== source)
                                    text = source
                            }
                            onTextChanged: Controller.setEditLine(index, text)
                            Keys.onReturnPressed: Controller.splitEditLine(index, cursorPosition)
                            Keys.onPressed: {
                                if (event.key === Qt.Key_Backspace && cursorPosition === 0 && index > 0) {
                                    event.accepted = true
                                    Controller.joinEditLine(index)
                                }
                            }
                            Connections {
                                target: Controller
                                onEditFocusChanged: claimFocus()
                            }
                            Component.onCompleted: {
                                if (text !== source)
                                    text = source
                                claimFocus()
                            }
                        }
                    }

                    Column {
                        id: textButtons
                        anchors {
                            left: parent.left
                            right: parent.right
                            bottom: parent.bottom
                            margins: units.gu(2)
                        }
                        spacing: units.gu(1)

                    Button {
                        width: parent.width
                        text: "保存"
                        enabled: Controller.helperRunning && !Controller.versionMismatch && lineView.count > 0
                        onClicked: saveEdit.start()
                    }

                    Button {
                        width: parent.width
                        text: "节点"
                        enabled: Controller.helperRunning && lineView.count > 0
                        onClicked: openNodes.start()
                    }

                    Button {
                        width: parent.width
                        text: "激活"
                        enabled: Controller.helperRunning && !Controller.tunRunning && !Controller.versionMismatch
                        onClicked: activateEdit.start()
                    }

                    Label {
                        width: parent.width
                        wrapMode: Text.Wrap
                        text: "保存只校验，不替换正在使用的配置。激活前请先关闭代理。"
                    }

                    Label {
                        width: parent.width
                        wrapMode: Text.Wrap
                        color: theme.palette.normal.backgroundText
                        text: Controller.message
                    }
                    }
                }

                Flickable {
                    id: editFlick
                    visible: editorRoot.mode !== "text"
                    anchors.fill: parent
                    contentHeight: (editorRoot.mode === "form" ? formColumn.height : nodeColumn.height) + units.gu(4)
                    clip: true

                Column {
                    id: nodeColumn
                    visible: editorRoot.mode === "nodes"
                    width: editFlick.width - units.gu(4)
                    x: units.gu(2)
                    y: units.gu(2)
                    spacing: units.gu(1)

                    Label {
                        width: parent.width
                        wrapMode: Text.Wrap
                        visible: Controller.editProxies.length === 0
                        text: "这份配置没有内联节点，请用文本编辑。"
                    }

                    Repeater {
                        model: editorRoot.mode === "nodes" ? Controller.editProxies : []
                        delegate: Button {
                            width: nodeColumn.width
                            text: modelData.name + "  " + modelData.type + "  " + modelData.port
                            onClicked: {
                                editorRoot.formIndex = modelData.index
                                nameField.text = modelData.name
                                typeField.text = modelData.type
                                serverField.text = modelData.server
                                portField.text = modelData.port
                                networkField.text = modelData.network
                                tlsSwitch.checked = modelData.tls
                                udpSwitch.checked = modelData.udp
                                secretField.text = ""
                                secretField.placeholderText = modelData.hasSecret ? "已有密钥，留空则不修改" : "密钥，可留空"
                                editorRoot.mode = "form"
                            }
                        }
                    }

                    Button {
                        width: parent.width
                        text: "返回文本"
                        onClicked: editorRoot.mode = "text"
                    }

                    Label {
                        width: parent.width
                        wrapMode: Text.Wrap
                        color: theme.palette.normal.backgroundText
                        text: Controller.message
                    }
                }

                Column {
                    id: formColumn
                    visible: editorRoot.mode === "form"
                    width: editFlick.width - units.gu(4)
                    x: units.gu(2)
                    y: units.gu(2)
                    spacing: units.gu(1)

                    TextField {
                        id: nameField
                        width: parent.width
                        placeholderText: "节点名"
                        inputMethodHints: Qt.ImhNoPredictiveText
                    }

                    TextField {
                        id: typeField
                        width: parent.width
                        placeholderText: "类型"
                        inputMethodHints: Qt.ImhNoPredictiveText
                    }

                    TextField {
                        id: serverField
                        width: parent.width
                        placeholderText: "服务器"
                        inputMethodHints: Qt.ImhNoPredictiveText
                    }

                    TextField {
                        id: portField
                        width: parent.width
                        placeholderText: "端口"
                        inputMethodHints: Qt.ImhDigitsOnly
                    }

                    TextField {
                        id: networkField
                        width: parent.width
                        placeholderText: "网络，例如 tcp 或 ws"
                        inputMethodHints: Qt.ImhNoPredictiveText
                    }

                    Row {
                        width: parent.width
                        spacing: units.gu(2)

                        Label {
                            text: "TLS"
                        }

                        Switch {
                            id: tlsSwitch
                        }

                        Label {
                            text: "UDP"
                        }

                        Switch {
                            id: udpSwitch
                        }
                    }

                    TextField {
                        id: secretField
                        width: parent.width
                        echoMode: TextInput.Password
                        placeholderText: "密钥，可留空"
                        inputMethodHints: Qt.ImhSensitiveData | Qt.ImhNoPredictiveText
                    }

                    Button {
                        width: parent.width
                        text: "写回文本"
                        onClicked: writeNode.start()
                    }

                    Button {
                        width: parent.width
                        text: "返回节点"
                        onClicked: {
                            secretField.text = ""
                            editorRoot.mode = "nodes"
                        }
                    }

                    Label {
                        width: parent.width
                        wrapMode: Text.Wrap
                        text: "留空密钥会保留原来的密钥。这里不显示原密钥。"
                    }

                    Label {
                        width: parent.width
                        wrapMode: Text.Wrap
                        color: theme.palette.normal.backgroundText
                        text: Controller.message
                    }
                }
                }
            }

            Component.onCompleted: loadEdit.start()
        }
    }

    Component {
        id: picturePickerPage

        Page {
            header: PageHeader {
                id: pickerHeader
                title: "相册"
            }

            ContentPeerPicker {
                anchors {
                    top: pickerHeader.bottom
                    left: parent.left
                    right: parent.right
                    bottom: parent.bottom
                }
                contentType: ContentType.Pictures
                handler: ContentHandler.Source
                onPeerSelected: {
                    root.scanHold = true
                    peer.selectionType = ContentTransfer.Single
                    root.scanTransfer = peer.request()
                }
                onCancelPressed: {
                    scanAction.hasImage = false
                    scanAction.shouldPop = true
                    scanAction.start()
                }
            }
        }
    }

    Component.onCompleted: Controller.refresh()
}
