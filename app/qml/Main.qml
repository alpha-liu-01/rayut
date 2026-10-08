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
