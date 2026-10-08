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
    property bool drawerOpen: false
    property string pendingEditorGroup: ""
    property int pendingEditorIndex: -1
    property bool pendingEditorCard: false

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

        HomePage {
            onOpenDrawer: root.drawerOpen = true
            onOpenAuth: PopupUtils.open(authDialog, root)
            onEditNode: {
                root.pendingEditorGroup = Controller.viewedGroup
                root.pendingEditorIndex = index
                root.pendingEditorCard = true
                stack.push(editorPage)
            }
            onScanAlbum: {
                root.scanHint = ""
                stack.push(picturePickerPage)
            }
            onScanCamera: {
                root.scanHint = ""
                root.scanHold = true
                picturePeers.want = "camera.ubports"
                picturePeers.findPeers()
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
        id: aboutPage

        Page {
            header: PageHeader {
                id: aboutHeader
                title: "关于"
            }

            Column {
                anchors {
                    top: aboutHeader.bottom
                    left: parent.left
                    right: parent.right
                    margins: units.gu(2)
                }
                spacing: units.gu(2)

                Label {
                    width: parent.width
                    wrapMode: Text.Wrap
                    text: Controller.versionText
                }
            }
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
                title: "规则模板"
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
                        text: Controller.viewedGroup === Controller.activeGroup && Controller.tunRunning ? "请先关闭代理" : "规则作用在正在查看的组上。"
                    }

                    Button {
                        Layout.fillWidth: true
                        text: "全局代理"
                        enabled: Controller.helperRunning && !(Controller.viewedGroup === Controller.activeGroup && Controller.tunRunning)
                        color: Controller.ruleTemplate === "global" ? theme.palette.normal.positive : theme.palette.normal.base
                        onClicked: {
                            templateAction.templateId = "global"
                            templateAction.start()
                        }
                    }

                    Button {
                        Layout.fillWidth: true
                        text: "绕过局域网"
                        enabled: Controller.helperRunning && !(Controller.viewedGroup === Controller.activeGroup && Controller.tunRunning)
                        color: Controller.ruleTemplate === "lan" ? theme.palette.normal.positive : theme.palette.normal.base
                        onClicked: {
                            templateAction.templateId = "lan"
                            templateAction.start()
                        }
                    }

                    Button {
                        Layout.fillWidth: true
                        text: "绕过局域网和中国大陆"
                        enabled: Controller.helperRunning && !(Controller.viewedGroup === Controller.activeGroup && Controller.tunRunning)
                        color: Controller.ruleTemplate === "lan-china" ? theme.palette.normal.positive : theme.palette.normal.base
                        onClicked: {
                            templateAction.templateId = "lan-china"
                            templateAction.start()
                        }
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
        id: groupsPage

        GroupsPage {
        }
    }

    Component {
        id: editorPage

        Page {
            id: editorRoot
            property string mode: "text"
            property string groupId: ""
            property int pendingIndex: -1
            property bool card: false
            property int formIndex: -1

            header: PageHeader {
                id: editHeader
                title: editorRoot.mode === "text" ? "配置编辑" : "节点"
            }

            function openForm(row) {
                editorRoot.formIndex = row.index
                nameField.text = row.name
                typeField.text = row.type
                serverField.text = row.server
                portField.text = row.port
                networkField.text = row.network
                tlsSwitch.checked = row.tls
                udpSwitch.checked = row.udp
                secretField.text = ""
                secretField.placeholderText = row.hasSecret ? "已有密钥，留空则不修改" : "密钥，可留空"
                editorRoot.mode = "form"
            }

            Timer {
                id: loadEdit
                interval: 1
                onTriggered: {
                    var id = editorRoot.groupId !== "" ? editorRoot.groupId : root.pendingEditorGroup
                    editorRoot.groupId = id
                    editorRoot.card = root.pendingEditorCard
                    editorRoot.pendingIndex = root.pendingEditorIndex
                    Controller.loadGroupDocument(id)
                    if (editorRoot.card && editorRoot.pendingIndex >= 0) {
                        var rows = Controller.editProxies
                        for (var i = 0; i < rows.length; i++) {
                            if (rows[i].index === editorRoot.pendingIndex) {
                                editorRoot.openForm(rows[i])
                                return
                            }
                        }
                    }
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
                        if (editorRoot.card)
                            Controller.saveProfileText()
                        else
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

                    Label {
                        width: parent.width
                        wrapMode: Text.Wrap
                        text: "保存前会校验。正在使用的组要先关闭代理才能改。"
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
                        text: editorRoot.card ? "保存" : "写回文本"
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

    Timer {
        id: drawerNav
        interval: 1
        property string page: ""
        onTriggered: {
            root.drawerOpen = false
            if (page === "groups")
                stack.push(groupsPage)
            else if (page === "rules")
                stack.push(profilePage)
            else if (page === "session")
                stack.push(sessionPage)
            else if (page === "about")
                stack.push(aboutPage)
            else if (page === "editor") {
                root.pendingEditorGroup = Controller.viewedGroup
                root.pendingEditorIndex = -1
                root.pendingEditorCard = false
                stack.push(editorPage)
            }
        }
    }

    Item {
        id: drawerLayer
        anchors.fill: parent
        visible: root.drawerOpen
        z: 10

        MouseArea {
            anchors.fill: parent
            onClicked: root.drawerOpen = false
        }

        Rectangle {
            width: parent.width * 0.75
            height: parent.height
            color: theme.palette.normal.background

            MouseArea {
                anchors.fill: parent
            }

            Rectangle {
                anchors.right: parent.right
                width: units.dp(1)
                height: parent.height
                color: theme.palette.normal.foreground
                z: 2
            }

            Column {
                anchors {
                    fill: parent
                    margins: units.gu(2)
                }
                spacing: units.gu(1)

                Label {
                    width: parent.width
                    text: "Rayut"
                    font.pixelSize: FontUtils.sizeToPixels("large")
                }

                Row {
                    width: parent.width
                    spacing: units.gu(1)

                    Label {
                        width: parent.width - killSwitch.width - units.gu(1)
                        height: killSwitch.height
                        verticalAlignment: Text.AlignVCenter
                        text: "断开即拦截"
                    }

                    Switch {
                        id: killSwitch
                        checked: Controller.killSwitch
                        onClicked: Controller.setKillSwitch(!Controller.killSwitch)
                    }
                }

                Row {
                    width: parent.width
                    spacing: units.gu(1)

                    Label {
                        width: parent.width - lanSwitch.width - units.gu(1)
                        height: lanSwitch.height
                        verticalAlignment: Text.AlignVCenter
                        text: "局域网共享"
                    }

                    Switch {
                        id: lanSwitch
                        checked: Controller.allowLan
                        onClicked: Controller.setAllowLan(!Controller.allowLan)
                    }
                }

                Label {
                    width: parent.width
                    wrapMode: Text.WordWrap
                    text: Controller.allowLan ? "同一局域网可连接端口 " + Controller.lanPort : "端口 " + Controller.lanPort + " 只在本机"
                }

                Button {
                    width: parent.width
                    text: "订阅分组"
                    onClicked: {
                        drawerNav.page = "groups"
                        drawerNav.start()
                    }
                }

                Button {
                    width: parent.width
                    text: "规则模板"
                    onClicked: {
                        drawerNav.page = "rules"
                        drawerNav.start()
                    }
                }

                Button {
                    width: parent.width
                    text: "配置编辑"
                    onClicked: {
                        drawerNav.page = "editor"
                        drawerNav.start()
                    }
                }

                Button {
                    width: parent.width
                    text: "日志和当前连接"
                    onClicked: {
                        drawerNav.page = "session"
                        drawerNav.start()
                    }
                }

                Button {
                    width: parent.width
                    text: "关于"
                    onClicked: {
                        drawerNav.page = "about"
                        drawerNav.start()
                    }
                }
            }
        }
    }

    Component.onCompleted: Controller.refresh()
}
