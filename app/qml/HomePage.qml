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
            return qsTr("Failed")
        return ""
    }

    function shownDelay(node) {
        var posted = Controller.nodeDelays
        if (posted && posted[node.name] !== undefined)
            return posted[node.name]
        return node.delay
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
                text: qsTr("Menu")
                onTriggered: homeRoot.openDrawer()
            }
        ]

        trailingActionBar {
            numberOfSlots: 3
            actions: [
                Action {
                    iconName: "search"
                    text: qsTr("Search")
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
                    text: qsTr("Add")
                    onTriggered: PopupUtils.open(addMenu, header)
                },
                Action {
                    iconName: "contextual-menu"
                    text: qsTr("More")
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
        placeholderText: qsTr("Node name")
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
            text: Controller.tunRunning ? qsTr("Turn the proxy off first") : qsTr("Use this group")
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
                highlightMoveDuration: 200
                highlightResizeDuration: 0
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
                            text: modelData.type + (modelData.network ? "  " + modelData.network : "") + (homeRoot.delayText(homeRoot.shownDelay(modelData)) !== "" ? "  " + homeRoot.delayText(homeRoot.shownDelay(modelData)) : "")
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
                            Accessible.name: qsTr("Test delay")
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
                            Accessible.name: qsTr("Edit")
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
                            Accessible.name: qsTr("Share")
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
                            Accessible.name: qsTr("Delete")
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
                anchors {
                    top: parent.top
                    right: parent.right
                    margins: units.gu(1)
                }
                z: 2
                visible: Controller.delayRunning
                text: qsTr("Testing %1/%2").arg(Controller.delayDone).arg(Controller.delayTotal)
                color: theme.palette.normal.backgroundText
            }

            Label {
                anchors.centerIn: parent
                visible: Controller.helperRunning && homeRoot.filteredNodes().length === 0
                text: Controller.profileGroups.length === 0 ? qsTr("No groups yet") : qsTr("No nodes")
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
                            width: units.gu(16)
                            anchors.verticalCenter: parent.verticalCenter
                            spacing: units.gu(0.4)

                            Label {
                                width: parent.width
                                wrapMode: Text.Wrap
                                maximumLineCount: 2
                                elide: Text.ElideRight
                                text: Controller.summary
                            }

                            Label {
                                width: parent.width
                                elide: Text.ElideRight
                                color: theme.palette.normal.backgroundText
                                text: qsTr("↑ %1/s").arg(homeRoot.byteText(Controller.uploadRate))
                            }

                            Label {
                                width: parent.width
                                elide: Text.ElideRight
                                color: theme.palette.normal.positive
                                text: qsTr("↓ %1/s").arg(homeRoot.byteText(Controller.downloadRate))
                            }
                        }

                        Item {
                            id: trafficChart
                            width: parent.width - units.gu(17)
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
                        Accessible.name: qsTr("Main switch")

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
                        text: qsTr("Turn off")
                        onClicked: Controller.disableTun()
                    }

                    Button {
                        width: (parent.width - parent.spacing) / 2
                        text: qsTr("Reconnect")
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
                    text: qsTr("Clipboard")
                    onTriggered: homeRoot.queue("clip")
                }
                Action {
                    iconName: "stock_image"
                    text: qsTr("Album QR code")
                    onTriggered: homeRoot.scanAlbum()
                }
                Action {
                    iconName: "camera-symbolic"
                    text: qsTr("Camera QR code")
                    onTriggered: homeRoot.scanCamera()
                }
                Action {
                    iconName: "insert-link"
                    text: qsTr("Subscription link")
                    onTriggered: PopupUtils.open(urlDialog, homeRoot.header)
                }
                Action {
                    iconName: "stock_document"
                    text: qsTr("Local YAML")
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
                    text: qsTr("Restart the proxy")
                    enabled: Controller.helperRunning && !Controller.versionMismatch
                    onTriggered: homeRoot.queue("restart")
                }
                Action {
                    iconName: "delete"
                    text: qsTr("Delete the nodes in this group")
                    enabled: Controller.viewedGroup !== ""
                    onTriggered: PopupUtils.open(clearDialog, homeRoot.header)
                }
                Action {
                    iconName: "share"
                    text: qsTr("Export share links")
                    enabled: Controller.viewedGroup !== ""
                    onTriggered: homeRoot.queue("export")
                }
                Action {
                    iconName: "timer"
                    text: qsTr("Test delay for this group")
                    enabled: Controller.helperRunning && Controller.viewedGroup !== ""
                    onTriggered: homeRoot.queue("testall")
                }
                Action {
                    iconName: "sync"
                    text: qsTr("Refresh this subscription")
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
            title: qsTr("Subscription link")

            TextField {
                id: urlField
                placeholderText: qsTr("https subscription link")
                inputMethodHints: Qt.ImhNoPredictiveText | Qt.ImhSensitiveData
            }

            Button {
                text: qsTr("Import")
                color: theme.palette.normal.positive
                onClicked: {
                    var entered = urlField.text
                    urlField.text = ""
                    PopupUtils.close(urlBox)
                    homeRoot.queue("url", "", -1, entered)
                }
            }

            Button {
                text: qsTr("Cancel")
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
            title: qsTr("Local YAML")

            TextArea {
                id: yamlField
                height: units.gu(16)
                placeholderText: qsTr("Paste a Clash document or one share link")
                inputMethodHints: Qt.ImhNoPredictiveText | Qt.ImhSensitiveData
            }

            Button {
                text: qsTr("Import")
                color: theme.palette.normal.positive
                onClicked: {
                    var entered = yamlField.text
                    yamlField.text = ""
                    PopupUtils.close(yamlBox)
                    homeRoot.queue("yaml", "", -1, entered)
                }
            }

            Button {
                text: qsTr("Cancel")
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
            title: Controller.shareImage === "" ? qsTr("Cannot export") : qsTr("Share")

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
                text: qsTr("Close")
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
            title: qsTr("Delete this node?")
            text: homeRoot.pendingDeleteName

            Button {
                text: qsTr("Delete")
                color: theme.palette.normal.negative
                onClicked: {
                    PopupUtils.close(deleteNodeBox)
                    homeRoot.queue("delete", homeRoot.pendingDeleteName, homeRoot.pendingDeleteIndex)
                }
            }

            Button {
                text: qsTr("Cancel")
                onClicked: PopupUtils.close(deleteNodeBox)
            }
        }
    }

    Component {
        id: clearDialog

        Dialog {
            id: clearBox
            title: qsTr("Delete every node in this group?")
            text: qsTr("The group will remain.")

            Button {
                text: qsTr("Delete")
                color: theme.palette.normal.negative
                onClicked: {
                    PopupUtils.close(clearBox)
                    homeRoot.queue("clear")
                }
            }

            Button {
                text: qsTr("Cancel")
                onClicked: PopupUtils.close(clearBox)
            }
        }
    }
}
