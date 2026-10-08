import QtQuick 2.7
import Lomiri.Components 1.3
import Lomiri.Components.Popups 1.3
import QtQuick.Layouts 1.3
import Rayut 1.0

Page {
    id: coreRoot

    property string pendingTag: ""

    header: PageHeader {
        id: coreHeader
        title: "核心"
    }

    Component.onCompleted: Controller.refreshCores()

    Flickable {
        anchors {
            top: coreHeader.bottom
            left: parent.left
            right: parent.right
            bottom: parent.bottom
        }
        contentWidth: width
        contentHeight: column.height + units.gu(4)
        clip: true

        Column {
            id: column
            x: units.gu(2)
            y: units.gu(2)
            width: coreRoot.width - units.gu(4)
            spacing: units.gu(2)

            Label {
                width: parent.width
                wrapMode: Text.Wrap
                text: Controller.corePlace === "data"
                      ? "已安装。下次启动使用数据目录里的 " + Controller.coreTag
                      : "还没安装。当前启动用的是随应用带的核心"
            }

            Label {
                width: parent.width
                wrapMode: Text.Wrap
                color: theme.palette.normal.backgroundText
                text: Controller.message
            }

            Repeater {
                model: Controller.coreReleases

                delegate: Column {
                    width: column.width
                    spacing: units.gu(1)

                    Label {
                        width: parent.width
                        text: modelData.tag + (modelData.installed ? "  已在数据目录" : "")
                    }

                    Label {
                        width: parent.width
                        wrapMode: Text.Wrap
                        text: modelData.license
                    }

                    Label {
                        width: parent.width
                        wrapMode: Text.Wrap
                        text: modelData.source
                    }

                    Button {
                        width: parent.width
                        enabled: !Controller.coreBusy
                        text: Controller.coreBusy ? "正在校验并安装" : "安装此版本"
                        onClicked: {
                            coreRoot.pendingTag = modelData.tag
                            if (Controller.tunRunning) {
                                PopupUtils.open(disconnectDialog, coreRoot)
                            } else {
                                installLater.disconnectFirst = false
                                installLater.start()
                            }
                        }
                    }
                }
            }
        }
    }

    Timer {
        id: installLater
        interval: 1
        property bool disconnectFirst: false
        onTriggered: Controller.installCore(coreRoot.pendingTag, disconnectFirst)
    }

    Component {
        id: disconnectDialog

        Dialog {
            id: disconnectBox
            title: "先断开再替换"
            text: "替换核心前需要断开。旧进程还占用网络时，不会再启动第二份。"

            Button {
                text: "断开并安装"
                onClicked: {
                    installLater.disconnectFirst = true
                    installLater.start()
                    PopupUtils.close(disconnectBox)
                }
            }

            Button {
                text: "取消"
                onClicked: PopupUtils.close(disconnectBox)
            }
        }
    }
}
