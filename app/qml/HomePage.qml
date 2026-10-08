import QtQuick 2.7
import QtQuick.Window 2.2
import Lomiri.Components 1.3
import Lomiri.Components.Popups 1.3
import QtQuick.Layouts 1.3
import Rayut 1.0

Page {
    id: homeRoot

    signal openDrawer()
    signal openAuth()
    signal editNode(int index)
    signal scanAlbum()
    signal scanCamera()

    property bool searching: false
    property string filterText: ""
    property string shownSelector: ""
    property int revealNonce: 0
    property string pendingDeleteName: ""
    property int pendingDeleteIndex: -1

    function byteText(value) {
        var n = Number(value)
        if (n < 0)
            n = 0
        if (n < 1024)
            return n + " B"
        if (n < 1048576)
            return (n / 1024).toFixed(1) + " KB"
        if (n < 1073741824)
            return (n / 1048576).toFixed(1) + " MB"
        return (n / 1073741824).toFixed(2) + " GB"
    }

    function trafficMax() {
        var max = 1
        var rows = Controller.trafficSamples
        for (var i = 0; i < rows.length; i++) {
            var n = Number(rows[i].up) + Number(rows[i].down)
            if (n > max)
                max = n
        }
        return max
    }

    function delayText(value) {
        var n = Number(value)
        if (n > 0)
            return n + " ms"
        if (n < 0)
            return "失败"
        return ""
    }

    function filteredNodes() {
        var groups = Controller.proxySelectors
        var rows = []
        var found = false
        for (var g = 0; g < groups.length; g++) {
            if (groups[g].name === Controller.selectorName) {
                rows = groups[g].nodes
                found = true
                break
            }
        }
        if (!found)
            rows = groups.length > 0 ? groups[0].nodes : Controller.groupNodes
        var query = filterText.toLowerCase()
        if (query === "")
            return rows
        var out = []
        for (var i = 0; i < rows.length; i++) {
            if (String(rows[i].name).toLowerCase().indexOf(query) !== -1)
                out.push(rows[i])
        }
        return out
    }

    function queue(kind, name, index, text) {
        work.kind = kind
        work.name = name === undefined ? "" : name
        work.index = index === undefined ? -1 : index
        work.text = text === undefined ? "" : text
        work.selector = Controller.selectorName
        work.start()
    }

    function revealPolicy() {
        var groups = Controller.proxySelectors
        for (var i = 0; i < groups.length; i++) {
            if (groups[i].name === Controller.selectorName) {
                policyTabs.positionViewAtIndex(i, ListView.Contain)
                return
            }
        }
    }

    function groupMatches(index) {
        var groups = Controller.profileGroups
        if (index < 0 || index >= groups.length)
            return false
        return groups[index].id === Controller.viewedGroup
    }

    function stepGroup(delta) {
        var groups = Controller.profileGroups
        var index = -1
        for (var i = 0; i < groups.length; i++) {
            if (groups[i].id === Controller.viewedGroup)
                index = i
        }
        var next = index + delta
        if (next < 0 || next >= groups.length)
            return
        queue("show", groups[next].id)
    }

    function leaveSearch() {
        searching = false
        filterText = ""
        searchField.text = ""
        header.contents = null
        searchField.visible = false
    }

    onVisibleChanged: {
        if (visible) {
            shownSelector = ""
            revealNonce = revealNonce + 1
            Controller.refreshTraffic()
            Controller.refreshProfileGroups()
        }
    }

    header: PageHeader {
        id: header
        title: "Rayut"

        leadingActionBar.actions: [
            Action {
                iconName: "navigation-menu"
                text: "菜单"
                onTriggered: homeRoot.openDrawer()
            }
        ]

        trailingActionBar {
            numberOfSlots: 3
            actions: [
                Action {
                    iconName: "search"
                    text: "搜索"
                    onTriggered: {
                        if (homeRoot.searching) {
                            homeRoot.leaveSearch()
                            return
                        }
                        homeRoot.searching = true
                        searchField.visible = true
                        header.contents = searchField
                        searchField.forceActiveFocus()
                    }
                },
                Action {
                    iconName: "add"
                    text: "添加"
                    onTriggered: PopupUtils.open(addMenu, header)
                },
                Action {
                    iconName: "contextual-menu"
                    text: "更多"
                    onTriggered: PopupUtils.open(moreMenu, header)
                }
            ]
        }
    }

    TextField {
        id: searchField
        visible: false
        width: parent ? parent.width : units.gu(24)
        height: units.gu(4)
        placeholderText: "节点名"
        inputMethodHints: Qt.ImhNoPredictiveText
        onTextChanged: homeRoot.filterText = text
        onAccepted: focus = false
    }

    Timer {
        id: work
        interval: 1
        property string kind: ""
        property string name: ""
        property int index: -1
        property string text: ""
        property string selector: ""
        onTriggered: {
            var id = Controller.viewedGroup
            var payload = text
            var chosen = selector
            text = ""
            if (kind === "show")
                Controller.showGroup(name)
            else if (kind === "policy")
                Controller.showSelector(name)
            else if (kind === "select")
                Controller.selectNode(id, chosen, name)
            else if (kind === "delay")
                Controller.testNode(id, name)
            else if (kind === "delete")
                Controller.deleteNode(id, index)
            else if (kind === "share") {
                if (Controller.exportNode(id, name))
                    PopupUtils.open(shareDialog, homeRoot.header)
            }
            else if (kind === "use")
                Controller.useGroup(id)
            else if (kind === "clear")
                Controller.clearGroup(id)
            else if (kind === "export")
                Controller.exportGroup(id)
            else if (kind === "testall")
                Controller.testGroup(id)
            else if (kind === "refresh")
                Controller.refreshGroup(id)
            else if (kind === "restart")
                Controller.restartProxy()
            else if (kind === "clip")
                Controller.importClipboard()
            else if (kind === "scheme")
                Controller.importClipboardScheme(name)
            else if (kind === "url")
                Controller.importURL(payload)
            else if (kind === "yaml")
                Controller.importContent(payload)
        }
    }

    Timer {
        interval: 1000
        running: homeRoot.visible && Controller.delayRunning
        repeat: true
        onTriggered: Controller.pollGroupDelay(Controller.viewedGroup)
    }

    Timer {
        interval: 1000
        running: homeRoot.visible
        repeat: true
        onTriggered: {
            Controller.refreshStatus()
            Controller.refreshTraffic()
        }
    }

    ColumnLayout {
        anchors {
            top: header.bottom
            left: parent.left
            right: parent.right
            bottom: parent.bottom
        }
        spacing: 0

        Button {
            Layout.fillWidth: true
            Layout.margins: units.gu(1)
            visible: Controller.viewedGroup !== "" && Controller.viewedGroup !== Controller.activeGroup
            text: Controller.tunRunning ? "请先关闭代理" : "使用本组"
            onClicked: homeRoot.queue("use")
        }

        ListView {
            id: tabs
            Layout.fillWidth: true
            Layout.preferredHeight: visible ? units.gu(5) : 0
            visible: Controller.profileGroups.length > 1
            orientation: ListView.Horizontal
            clip: true
            boundsBehavior: Flickable.StopAtBounds
            model: Controller.profileGroups

            delegate: AbstractButton {
                width: tabLabel.implicitWidth + units.gu(3)
                height: tabs.height
                onClicked: homeRoot.queue("show", modelData.id)

                Rectangle {
                    anchors.fill: parent
                    color: modelData.id === Controller.viewedGroup ? theme.palette.normal.foreground : "transparent"
                }

                Label {
                    id: tabLabel
                    anchors.centerIn: parent
                    text: modelData.name + " (" + modelData.count + ")" + (modelData.active ? " ·" : "")
                    color: modelData.id === Controller.viewedGroup ? theme.palette.normal.foregroundText : theme.palette.normal.backgroundText
                }
            }

            Rectangle {
                anchors.bottom: parent.bottom
                width: parent.width
                height: units.dp(1)
                color: theme.palette.normal.foreground
            }
        }

        ListView {
            id: policyTabs
            Layout.fillWidth: true
            Layout.preferredHeight: visible ? units.gu(4) : 0
            visible: Controller.proxySelectors.length > 1
            orientation: ListView.Horizontal
            clip: true
            boundsBehavior: Flickable.StopAtBounds
            model: Controller.proxySelectors

            Timer {
                id: revealPolicyTimer
                interval: 1
                onTriggered: homeRoot.revealPolicy()
            }

            Connections {
                target: Controller
                onStateChanged: {
                    if (Controller.selectorName === homeRoot.shownSelector)
                        return
                    homeRoot.shownSelector = Controller.selectorName
                    revealPolicyTimer.restart()
                }
            }

            delegate: AbstractButton {
                width: policyLabel.implicitWidth + units.gu(3)
                height: policyTabs.height
                onClicked: homeRoot.queue("policy", modelData.name)

                Rectangle {
                    anchors.bottom: parent.bottom
                    width: parent.width
                    height: units.dp(2)
                    visible: modelData.name === Controller.selectorName
                    color: theme.palette.normal.activity
                }

                Label {
                    id: policyLabel
                    anchors.centerIn: parent
                    text: modelData.name
                    color: theme.palette.normal.backgroundText
                }
            }

            Rectangle {
                anchors.bottom: parent.bottom
                width: parent.width
                height: units.dp(1)
                color: theme.palette.normal.foreground
            }
        }

        Label {
            Layout.fillWidth: true
            Layout.margins: units.gu(2)
            visible: Controller.delayRunning
            text: "测速 " + Controller.delayDone + "/" + Controller.delayTotal
        }

        Item {
            Layout.fillWidth: true
            Layout.fillHeight: true

            ListView {
                id: pager
                anchors.fill: parent
                orientation: ListView.Horizontal
                snapMode: ListView.SnapOneItem
                boundsBehavior: Flickable.StopAtBounds
                highlightRangeMode: ListView.StrictlyEnforceRange
                model: Controller.profileGroups
                property real originX: 0
                onMovementStarted: originX = contentX
                onMovementEnded: {
                    if (Math.abs(contentX - originX) < width / 3)
                        return
                    var groups = Controller.profileGroups
                    if (currentIndex < 0 || currentIndex >= groups.length)
                        return
                    homeRoot.queue("show", groups[currentIndex].id)
                }

                delegate: ListView {
                    id: nodes
                    width: pager.width
                    height: pager.height
                    clip: true
                    boundsBehavior: Flickable.StopAtBounds
                    cacheBuffer: units.gu(40)
                    model: index === pager.currentIndex && homeRoot.groupMatches(index) ? homeRoot.filteredNodes() : []
                    property real keptY: 0
                    property string keptKey: ""
                    property int seenNonce: 0

                    onContentYChanged: {
                        if (moving || dragging || flicking)
                            keptY = contentY
                    }

                    Timer {
                        id: scrollKeep
                        interval: 1
                        property real y: 0
                        onTriggered: nodes.contentY = y
                    }

                    Timer {
                        id: revealNode
                        interval: 1
                        onTriggered: {
                            var rows = homeRoot.filteredNodes()
                            for (var i = 0; i < rows.length; i++) {
                                if (rows[i].selected) {
                                    nodes.positionViewAtIndex(i, ListView.Center)
                                    nodes.keptY = nodes.contentY
                                    return
                                }
                            }
                        }
                    }

                    Connections {
                        target: Controller
                        onStateChanged: {
                            var opened = nodes.seenNonce !== homeRoot.revealNonce
                            if (opened)
                                nodes.seenNonce = homeRoot.revealNonce
                            if (nodes.moving || nodes.dragging || nodes.flicking)
                                return
                            var key = Controller.viewedGroup + "/" + Controller.selectorName
                            if (opened || key !== nodes.keptKey) {
                                nodes.keptKey = key
                                revealNode.restart()
                                return
                            }
                            scrollKeep.y = nodes.keptY
                            scrollKeep.restart()
                        }
                    }

                delegate: Item {
                    width: nodes.width
                    height: units.gu(8)

                    Rectangle {
                        width: units.gu(0.5)
                        height: parent.height
                        visible: modelData.selected
                        color: theme.palette.normal.positive
                    }

                    Column {
                        anchors {
                            left: parent.left
                            right: actions.left
                            leftMargin: units.gu(2)
                            rightMargin: units.gu(1)
                            verticalCenter: parent.verticalCenter
                        }
                        spacing: units.gu(0.4)

                        Label {
                            width: parent.width
                            elide: Text.ElideRight
                            text: modelData.name
                        }

                        Label {
                            width: parent.width
                            elide: Text.ElideRight
                            color: theme.palette.normal.backgroundText
                            text: modelData.type + (modelData.network ? "  " + modelData.network : "") + (homeRoot.delayText(modelData.delay) !== "" ? "  " + homeRoot.delayText(modelData.delay) : "")
                        }
                    }

                    AbstractButton {
                        anchors.fill: parent
                        onClicked: homeRoot.queue("select", modelData.name, modelData.index)
                    }

                    Row {
                        id: actions
                        anchors {
                            right: parent.right
                            rightMargin: units.gu(0.5)
                            verticalCenter: parent.verticalCenter
                        }
                        spacing: 0

                        AbstractButton {
                            width: units.gu(3.5)
                            height: units.gu(4)
                            Accessible.name: "测延迟"
                            onClicked: homeRoot.queue("delay", modelData.name, modelData.index)

                            Icon {
                                anchors.centerIn: parent
                                width: units.gu(2.5)
                                height: width
                                name: "timer"
                            }
                        }

                        AbstractButton {
                            width: units.gu(3.5)
                            height: units.gu(4)
                            visible: modelData.index >= 0
                            Accessible.name: "编辑"
                            onClicked: homeRoot.editNode(modelData.index)

                            Icon {
                                anchors.centerIn: parent
                                width: units.gu(2.5)
                                height: width
                                name: "edit"
                            }
                        }

                        AbstractButton {
                            width: units.gu(3.5)
                            height: units.gu(4)
                            visible: modelData.shareable
                            Accessible.name: "分享"
                            onClicked: homeRoot.queue("share", modelData.name, modelData.index)

                            Icon {
                                anchors.centerIn: parent
                                width: units.gu(2.5)
                                height: width
                                name: "share"
                            }
                        }

                        AbstractButton {
                            width: units.gu(3.5)
                            height: units.gu(4)
                            visible: modelData.index >= 0
                            Accessible.name: "删除"
                            onClicked: {
                                homeRoot.pendingDeleteName = modelData.name
                                homeRoot.pendingDeleteIndex = modelData.index
                                PopupUtils.open(deleteNodeDialog, homeRoot.header)
                            }

                            Icon {
                                anchors.centerIn: parent
                                width: units.gu(2.5)
                                height: width
                                name: "delete"
                            }
                        }
                    }

                    Rectangle {
                        anchors.bottom: parent.bottom
                        width: parent.width
                        height: units.dp(1)
                        color: theme.palette.normal.base
                    }
                }
                }
            }

            Connections {
                target: Controller
                onStateChanged: {
                    var groups = Controller.profileGroups
                    for (var i = 0; i < groups.length; i++) {
                        if (groups[i].id === Controller.viewedGroup && pager.currentIndex !== i) {
                            pager.currentIndex = i
                            return
                        }
                    }
                }
            }

            Label {
                anchors.centerIn: parent
                visible: Controller.helperRunning && homeRoot.filteredNodes().length === 0
                text: Controller.profileGroups.length === 0 ? "还没有分组" : "没有节点"
            }
        }

        Item {
            Layout.fillWidth: true
            Layout.leftMargin: units.gu(2)
            Layout.rightMargin: units.gu(2)
            Layout.bottomMargin: units.gu(1)
            implicitHeight: bottomColumn.implicitHeight

            Rectangle {
                anchors {
                    top: parent.top
                    left: parent.left
                    right: parent.right
                    leftMargin: -units.gu(2)
                    rightMargin: -units.gu(2)
                }
                height: units.dp(1)
                color: theme.palette.normal.foreground
            }

            Column {
                id: bottomColumn
                width: parent.width
                spacing: units.gu(1)

                Label {
                    width: parent.width
                    wrapMode: Text.Wrap
                    visible: Controller.message !== ""
                    color: theme.palette.normal.backgroundText
                    text: Controller.message
                }

                Item {
                    width: parent.width
                    height: units.gu(8)

                    Row {
                        anchors {
                            left: parent.left
                            right: powerButton.left
                            top: parent.top
                            bottom: parent.bottom
                            rightMargin: units.gu(1)
                        }
                        spacing: units.gu(1)

                        Column {
                            width: units.gu(12)
                            anchors.verticalCenter: parent.verticalCenter
                            spacing: units.gu(0.4)

                            Label {
                                width: parent.width
                                elide: Text.ElideRight
                                text: Controller.summary
                            }

                            Label {
                                width: parent.width
                                elide: Text.ElideRight
                                color: theme.palette.normal.backgroundText
                                text: "↑ " + homeRoot.byteText(Controller.uploadRate) + "/s"
                            }

                            Label {
                                width: parent.width
                                elide: Text.ElideRight
                                color: theme.palette.normal.positive
                                text: "↓ " + homeRoot.byteText(Controller.downloadRate) + "/s"
                            }
                        }

                        Item {
                            id: trafficChart
                            width: parent.width - units.gu(13)
                            height: units.gu(4)
                            anchors.verticalCenter: parent.verticalCenter

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
                    }

                    AbstractButton {
                        id: powerButton
                        width: units.gu(7)
                        height: units.gu(7)
                        visible: !Controller.networkBlocked
                        anchors {
                            right: parent.right
                            verticalCenter: parent.verticalCenter
                            verticalCenterOffset: -units.gu(2)
                        }
                        Accessible.name: "总开关"

                        onClicked: {
                            if (!Controller.helperRunning)
                                homeRoot.openAuth()
                            else
                                Controller.toggleProxy()
                        }

                        Rectangle {
                            anchors.fill: parent
                            radius: width / 2
                            color: Controller.tunRunning ? "#3a7ca5" : theme.palette.normal.base
                        }

                        Canvas {
                            anchors.centerIn: parent
                            width: parent.width * 0.46
                            height: parent.height * 0.46
                            antialiasing: true
                            canvasSize: Qt.size(width * Screen.devicePixelRatio, height * Screen.devicePixelRatio)

                            onWidthChanged: requestPaint()
                            onHeightChanged: requestPaint()
                            onCanvasSizeChanged: requestPaint()

                            onPaint: {
                                var ctx = getContext("2d")
                                ctx.reset()
                                var ratio = Screen.devicePixelRatio
                                ctx.scale(ratio, ratio)
                                ctx.scale(width / 24, height / 24)
                                ctx.fillStyle = "#ffffff"
                                ctx.beginPath()
                                ctx.moveTo(2.01, 21)
                                ctx.lineTo(23, 12)
                                ctx.lineTo(2.01, 3)
                                ctx.lineTo(2, 10)
                                ctx.lineTo(17, 12)
                                ctx.lineTo(2, 14)
                                ctx.closePath()
                                ctx.fill()
                            }
                        }
                    }
                }

                Row {
                    width: parent.width
                    spacing: units.gu(1)
                    visible: Controller.networkBlocked

                    Button {
                        width: (parent.width - parent.spacing) / 2
                        text: "关闭"
                        onClicked: Controller.disableTun()
                    }

                    Button {
                        width: (parent.width - parent.spacing) / 2
                        text: "重连"
                        onClicked: Controller.enableTun()
                    }
                }
            }
        }
    }

    Component {
        id: addMenu

        ActionSelectionPopover {
            actions: ActionList {
                Action {
                    iconName: "edit-paste"
                    text: "剪贴板"
                    onTriggered: homeRoot.queue("clip")
                }
                Action {
                    iconName: "stock_image"
                    text: "相册二维码"
                    onTriggered: homeRoot.scanAlbum()
                }
                Action {
                    iconName: "camera-symbolic"
                    text: "相机二维码"
                    onTriggered: homeRoot.scanCamera()
                }
                Action {
                    iconName: "insert-link"
                    text: "订阅链接"
                    onTriggered: PopupUtils.open(urlDialog, homeRoot.header)
                }
                Action {
                    iconName: "stock_document"
                    text: "本地 YAML"
                    onTriggered: PopupUtils.open(yamlDialog, homeRoot.header)
                }
                Action {
                    iconName: "stock_key"
                    text: "Shadowsocks"
                    onTriggered: homeRoot.queue("scheme", "ss")
                }
                Action {
                    iconName: "stock_key"
                    text: "VMess"
                    onTriggered: homeRoot.queue("scheme", "vmess")
                }
                Action {
                    iconName: "stock_key"
                    text: "VLESS"
                    onTriggered: homeRoot.queue("scheme", "vless")
                }
                Action {
                    iconName: "stock_key"
                    text: "Trojan"
                    onTriggered: homeRoot.queue("scheme", "trojan")
                }
                Action {
                    iconName: "stock_key"
                    text: "Hysteria2"
                    onTriggered: homeRoot.queue("scheme", "hysteria2")
                }
                Action {
                    iconName: "stock_key"
                    text: "TUIC"
                    onTriggered: homeRoot.queue("scheme", "tuic")
                }
            }
        }
    }

    Component {
        id: moreMenu

        ActionSelectionPopover {
            actions: ActionList {
                Action {
                    iconName: "view-refresh"
                    text: "重启代理"
                    enabled: Controller.helperRunning && !Controller.versionMismatch
                    onTriggered: homeRoot.queue("restart")
                }
                Action {
                    iconName: "delete"
                    text: "删除当前组里的节点"
                    enabled: Controller.viewedGroup !== ""
                    onTriggered: PopupUtils.open(clearDialog, homeRoot.header)
                }
                Action {
                    iconName: "share"
                    text: "导出分享链接"
                    enabled: Controller.viewedGroup !== ""
                    onTriggered: homeRoot.queue("export")
                }
                Action {
                    iconName: "timer"
                    text: "测试当前组延迟"
                    enabled: Controller.helperRunning && Controller.viewedGroup !== ""
                    onTriggered: homeRoot.queue("testall")
                }
                Action {
                    iconName: "sync"
                    text: "刷新当前订阅"
                    enabled: Controller.groupKind === "subscription"
                    onTriggered: homeRoot.queue("refresh")
                }
            }
        }
    }

    Component {
        id: urlDialog

        Dialog {
            id: urlBox
            title: "订阅链接"

            TextField {
                id: urlField
                placeholderText: "https 订阅链接"
                inputMethodHints: Qt.ImhNoPredictiveText | Qt.ImhSensitiveData
            }

            Button {
                text: "导入"
                color: theme.palette.normal.positive
                onClicked: {
                    var entered = urlField.text
                    urlField.text = ""
                    PopupUtils.close(urlBox)
                    homeRoot.queue("url", "", -1, entered)
                }
            }

            Button {
                text: "取消"
                onClicked: {
                    urlField.text = ""
                    PopupUtils.close(urlBox)
                }
            }
        }
    }

    Component {
        id: yamlDialog

        Dialog {
            id: yamlBox
            title: "本地 YAML"

            TextArea {
                id: yamlField
                height: units.gu(16)
                placeholderText: "粘贴一份 Clash 文档或一条分享链接"
                inputMethodHints: Qt.ImhNoPredictiveText | Qt.ImhSensitiveData
            }

            Button {
                text: "导入"
                color: theme.palette.normal.positive
                onClicked: {
                    var entered = yamlField.text
                    yamlField.text = ""
                    PopupUtils.close(yamlBox)
                    homeRoot.queue("yaml", "", -1, entered)
                }
            }

            Button {
                text: "取消"
                onClicked: {
                    yamlField.text = ""
                    PopupUtils.close(yamlBox)
                }
            }
        }
    }

    Component {
        id: shareDialog

        Dialog {
            id: shareBox
            title: Controller.shareImage === "" ? "导出不了" : "分享"

            Image {
                visible: Controller.shareImage !== ""
                width: units.gu(28)
                height: width
                source: Controller.shareImage
                fillMode: Image.PreserveAspectFit
                smooth: false
                cache: false
            }

            Label {
                visible: Controller.shareImage === ""
                width: parent ? parent.width : units.gu(28)
                wrapMode: Text.Wrap
                text: Controller.shareNote
            }

            Button {
                text: "关闭"
                onClicked: {
                    Controller.clearShare()
                    PopupUtils.close(shareBox)
                }
            }

            Component.onDestruction: Controller.clearShare()
        }
    }

    Component {
        id: deleteNodeDialog

        Dialog {
            id: deleteNodeBox
            title: "删除这个节点？"
            text: homeRoot.pendingDeleteName

            Button {
                text: "删除"
                color: theme.palette.normal.negative
                onClicked: {
                    PopupUtils.close(deleteNodeBox)
                    homeRoot.queue("delete", homeRoot.pendingDeleteName, homeRoot.pendingDeleteIndex)
                }
            }

            Button {
                text: "取消"
                onClicked: PopupUtils.close(deleteNodeBox)
            }
        }
    }

    Component {
        id: clearDialog

        Dialog {
            id: clearBox
            title: "删除本组全部节点？"
            text: "分组会留下来。"

            Button {
                text: "删除"
                color: theme.palette.normal.negative
                onClicked: {
                    PopupUtils.close(clearBox)
                    homeRoot.queue("clear")
                }
            }

            Button {
                text: "取消"
                onClicked: PopupUtils.close(clearBox)
            }
        }
    }
}
