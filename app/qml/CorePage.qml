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
        title: qsTr("Core")
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
                      ? qsTr("Installed. The next start uses %1 from the data directory.").arg(Controller.coreTag)
                      : qsTr("Not installed yet. Startup still uses the core shipped with the app.")
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
                        text: modelData.installed
                              ? qsTr("%1, installed in the data directory").arg(modelData.tag)
                              : modelData.tag
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
                        text: Controller.coreBusy ? qsTr("Checking and installing") : qsTr("Install this version")
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
            title: qsTr("Disconnect before replacing")
            text: qsTr("Disconnect before replacing. If the old process still holds the network, a second core will not be started.")

            Button {
                text: qsTr("Disconnect and install")
                onClicked: {
                    installLater.disconnectFirst = true
                    installLater.start()
                    PopupUtils.close(disconnectBox)
                }
            }

            Button {
                text: qsTr("Cancel")
                onClicked: PopupUtils.close(disconnectBox)
            }
        }
    }
}
